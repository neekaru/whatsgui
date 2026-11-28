// Integración con WhatsApp Web: usa sus propios módulos internos (window.require) para
// subir el audio, enviar el sticker con stickerAudio y abrir el diálogo nativo de "Reenviar".
(() => {
  const WAAS = (window.WAAS = window.WAAS || {});
  const log = (lvl, ev, data) => WAAS.log(lvl, ev, data);
  const W = (WAAS.wa = {});

  const SOUND_PROPS = ["sound_stickers_consumption_enabled", "sound_stickers_creation_enabled", "sound_stickers_audio_playing_enabled"];
  const NEEDED = [
    "WAWebChatCollection", "WAWebMsgCollection", "WAWebMediaPrep", "WAWebMediaOpaqueData",
    "WAWebUploadManagerMainThread", "WAWebUploadManager", "WAWebCreateMediaUploadMetrics", "WAWebMmsMediaTypes",
    "WAWebMediaGetUploadOriginForChat", "WAWebSoundStickerGating", "WAWebSoundStickerUtils", "WAWebABProps",
    "WAWebModalManager", "WAWebForwardMessageFlowLoadable", "WAWebStateUtils", "WAWebUserPrefsMeUser",
    "WAWebFindChatAction", "WAWebSendMsgResultAction", "WAWebAck", "WALogger", "react",
  ];

  // ------------------------------------------------------------ acceso a módulos
  function hasLoader() { return typeof window.require === "function" && typeof window.__d === "function"; }

  function ns(name) {
    if (typeof window.importNamespace === "function") {
      try { const m = window.importNamespace(name); if (m) return m; } catch (e) { /* probar require */ }
    }
    return window.require(name);
  }
  function def(name) {
    const m = ns(name);
    return m && m.default !== undefined ? m.default : m;
  }
  async function load(name, timeoutMs = 15000) {
    try { return ns(name); } catch (e) { /* puede ser un módulo diferido */ }
    if (typeof window.requireLazy !== "function") throw new Error(`Módulo ${name} no disponible y no hay requireLazy`);
    await new Promise((resolve, reject) => {
      const t = setTimeout(() => reject(new Error(`Tiempo agotado cargando el módulo ${name}`)), timeoutMs);
      window.requireLazy([name], () => { clearTimeout(t); resolve(); });
    });
    return ns(name);
  }
  W.ns = ns; W.def = def; W.load = load; W.hasLoader = hasLoader;

  function waVersion() {
    try { return (window.Debug && window.Debug.VERSION) || null; } catch (e) { return null; }
  }

  function abValues() {
    const out = {};
    try {
      const AB = ns("WAWebABProps");
      for (const p of SOUND_PROPS.concat(["sound_stickers_channels_send_enabled"])) {
        try { out[p] = AB.getABPropConfigValue(p); } catch (e) { out[p] = "error: " + e.message; }
      }
    } catch (e) { out.error = e.message; }
    return out;
  }

  // ------------------------------------------------------------ diagnóstico
  W.diagnose = async function () {
    const mods = {};
    for (const n of NEEDED) {
      try { const m = ns(n); mods[n] = m ? "ok" : "vacío"; } catch (e) { mods[n] = "falta: " + e.message.slice(0, 120); }
    }
    let caps = null;
    try { caps = await WAAS.capabilities(); } catch (e) { caps = { error: e.message }; }
    let activeChat = null;
    try { const c = ns("WAWebChatCollection").ChatCollection.getActive(); activeChat = c ? describeChat(c) : null; } catch (e) { activeChat = "error: " + e.message; }
    const d = {
      extension: WAAS.VERSION,
      waVersion: waVersion(),
      loader: { require: typeof window.require, importNamespace: typeof window.importNamespace, requireLazy: typeof window.requireLazy },
      modulos: mods,
      abPropsOriginales: W._abOriginal || abValues(),
      abPropsActuales: abValues(),
      navegador: caps,
      chatAbierto: activeChat,
      gatingForzado: !!W._forced,
    };
    log("info", "diagnostico", d);
    return d;
  };

  function describeChat(chat) {
    try {
      const id = chat.id;
      return {
        tipo: id.isGroup && id.isGroup() ? "grupo" : id.server === "lid" ? "lid" : id.isUser && id.isUser() ? "usuario" : String(id.server),
        id: WAAS.maskIds(String(id._serialized || id)),
        nombre: WAAS.maskIds(String(chat.formattedTitle || chat.name || "")).slice(0, 40),
      };
    } catch (e) { return { error: e.message }; }
  }
  W.describeChat = describeChat;

  // ------------------------------------------------------------ activar stickers con sonido (local)
  W.forceSoundStickers = function () {
    if (W._forced) return true;
    W._abOriginal = abValues();
    let method = null;
    try {
      const AB = ns("WAWebABProps");
      const orig = AB.getABPropConfigValue;
      if (!orig.__waas) {
        const patched = function (name) {
          if (SOUND_PROPS.includes(name)) return true;
          return orig.apply(this, arguments);
        };
        patched.__waas = true;
        AB.getABPropConfigValue = patched;
      }
      method = "WAWebABProps";
    } catch (e) { log("warn", "gating:abprops-fallo", e); }
    let G;
    try { G = ns("WAWebSoundStickerGating"); } catch (e) { log("error", "gating:modulo-falta", e); }
    const ok = () => { try { return G.isSoundStickerCreationEnabled() === true && G.isSoundStickerConsumptionEnabled() === true; } catch (e) { return false; } };
    if (G && !ok()) {
      try {
        G.isSoundStickerConsumptionEnabled = () => true;
        G.isSoundStickerCreationEnabled = () => true;
        G.isSoundStickerPlaybackEnabled = () => true;
        method = "WAWebSoundStickerGating";
      } catch (e) { log("warn", "gating:modulo-fallo", e); }
    }
    W._forced = !!G && ok();
    log(W._forced ? "info" : "error", "gating:forzar", { metodo: method, original: W._abOriginal, ahora: abValues(), activo: W._forced });
    return W._forced;
  };

  // ------------------------------------------------------------ registrar logs internos de WA durante una operación
  W.hookWALogger = function () {
    if (W._loggerHooked) return;
    try {
      const L = ns("WALogger");
      const RELEVANT = /sticker|media|upload|sound|forward|prep|send|mms|encrypt|kaleidoscope/i;
      for (const lvl of ["LOG", "WARN", "ERROR"]) {
        const orig = L[lvl];
        if (typeof orig !== "function" || orig.__waas) continue;
        const wrapped = function (strings) {
          try {
            if (WAAS.activeRun && strings && strings.raw) {
              const vals = Array.prototype.slice.call(arguments, 1);
              let msg = "";
              for (let i = 0; i < strings.length; i++) msg += strings[i] + (i < vals.length ? String(vals[i]) : "");
              if (lvl !== "LOG" || RELEVANT.test(msg)) log(lvl === "ERROR" ? "error" : lvl === "WARN" ? "warn" : "info", "wa-" + lvl.toLowerCase(), msg);
            }
          } catch (e) { /* nunca romper el logger de WA */ }
          return orig.apply(this, arguments);
        };
        wrapped.__waas = true;
        L[lvl] = wrapped;
      }
      W._loggerHooked = true;
      log("info", "walogger:enganchado", null);
    } catch (e) { log("warn", "walogger:no-enganchado", e); }
  };

  // ------------------------------------------------------------ chats
  W.getActiveChat = function () {
    return ns("WAWebChatCollection").ChatCollection.getActive() || null;
  };

  W.getSelfChat = async function () {
    const Me = ns("WAWebUserPrefsMeUser");
    const CC = ns("WAWebChatCollection").ChatCollection;
    const getters = ["getMaybeMePnUser", "getMeUserOrThrow", "getMaybeMeLidUser"];
    let firstWid = null;
    for (const g of getters) {
      try {
        const wid = Me[g] && Me[g]();
        if (!wid) continue;
        firstWid = firstWid || wid;
        const c = CC.get(wid) || CC.get(String(wid._serialized || wid));
        if (c) { log("info", "chat-propio:encontrado", { via: g, chat: describeChat(c) }); return c; }
      } catch (e) { /* siguiente */ }
    }
    if (!firstWid) throw new Error("No se pudo obtener tu propio número");
    const r = await ns("WAWebFindChatAction").findOrCreateLatestChat(firstWid, "forwardSelectedModals");
    const chat = r && (r.chat || r);
    log("info", "chat-propio:creado", { chat: describeChat(chat) });
    return chat;
  };

  // ------------------------------------------------------------ subida del audio
  function toB64(v) {
    if (v == null) return v;
    if (typeof v === "string") return v;
    const bytes = v instanceof ArrayBuffer ? new Uint8Array(v) : new Uint8Array(v.buffer || v);
    let s = "";
    for (const b of bytes) s += String.fromCharCode(b);
    return btoa(s);
  }
  function b64Len(s) { try { return atob(s).length; } catch (e) { return -1; } }

  const noopQpl = new Proxy({}, { get: () => () => undefined });

  W.uploadAudio = async function (oggBlob, seconds, chat) {
    const MT = ns("WAWebMmsMediaTypes").MEDIA_TYPES;
    let uploadOrigin = 1;
    try { uploadOrigin = def("WAWebMediaGetUploadOriginForChat")(chat); } catch (e) { log("warn", "subida:origen", e); }
    const params = {
      blob: oggBlob, type: MT.AUDIO, signal: new AbortController().signal, uploadQpl: noopQpl,
      uploadOrigin, fileOrigin: undefined, userUploadAttemptCount: 0, forwardedFromWeb: false, isViewOnce: false, isHdPhoto: false,
    };
    let res, via;
    try {
      const metrics = def("WAWebCreateMediaUploadMetrics")(params);
      res = await def("WAWebUploadManagerMainThread").encryptAndUpload(params, metrics);
      via = "MainThread";
    } catch (e) {
      log("warn", "subida:mainthread-fallo", e);
      res = await def("WAWebUploadManager").encryptAndUpload(params);
      via = "UploadManager";
    }
    const bytes = new Uint8Array(await oggBlob.arrayBuffer());
    const filehash = await WAAS._internals.sha256b64(bytes);
    const stickerAudio = {
      deprecatedMms3Url: res.url || undefined,
      directPath: res.directPath,
      mediaKey: toB64(res.mediaKey),
      mediaKeyTimestamp: res.mediaKeyTimestamp,
      filehash,
      encFilehash: toB64(res.encFilehash),
      size: bytes.length,
      mimetype: "audio/ogg; codecs=opus",
      duration: seconds,
    };
    let complete = null;
    try { complete = ns("WAWebSoundStickerUtils").isCompleteStickerAudio(stickerAudio); } catch (e) { complete = "error: " + e.message; }
    log(complete === true ? "info" : "error", "subida:audio", {
      via,
      tiposRespuesta: Object.fromEntries(Object.entries(res || {}).map(([k, v]) => [k, v == null ? String(v) : v.constructor ? v.constructor.name : typeof v])),
      directPath: stickerAudio.directPath ? stickerAudio.directPath.split("?")[0] : null,
      tieneUrl: !!stickerAudio.deprecatedMms3Url,
      mediaKeyBytes: b64Len(stickerAudio.mediaKey),
      encFilehashBytes: b64Len(stickerAudio.encFilehash),
      filehash, size: stickerAudio.size, duration: seconds,
      isCompleteStickerAudio: complete,
    });
    if (complete !== true) throw new Error("WhatsApp menganggap audio yang diunggah tidak valid (isCompleteStickerAudio=" + complete + ")");
    return stickerAudio;
  };

  // ------------------------------------------------------------ envío del sticker
  W.sendSticker = async function (chat, webpBlob, info, stickerAudio) {
    const Opaque = def("WAWebMediaOpaqueData");
    const MediaPrep = def("WAWebMediaPrep");
    const opaque = await Opaque.createFromData(webpBlob, "image/webp");
    const data = {
      type: "sticker", mediaBlob: opaque, mimetype: "image/webp", size: webpBlob.size,
      fullWidth: info.lado || 512, fullHeight: info.lado || 512, isAnimated: !!info.isAnimated, emojis: ["🔊"],
    };
    const prep = new MediaPrep("sticker", Promise.resolve(data));
    log("info", "envio:preparado", { chat: describeChat(chat), webpBytes: webpBlob.size, conAudio: !!stickerAudio });
    const res = await prep.sendToChat({ chat, options: stickerAudio ? { stickerAudio } : {} });
    const OK = ns("WAWebSendMsgResultAction").SendMsgResult.OK;
    const msg = res && res.msg;
    const resumen = {
      resultado: res && res.messageSendResult,
      ok: res && res.messageSendResult === OK,
      msgId: msg && msg.id ? WAAS.maskIds(String(msg.id._serialized || msg.id)) : null,
      msgTieneStickerAudio: msg ? !!(msg.stickerAudio || (msg.attributes && msg.attributes.stickerAudio)) : null,
      ack: msg ? msg.ack : null,
    };
    log(resumen.ok ? "info" : "error", "envio:resultado", resumen);
    if (!resumen.ok) throw new Error("WhatsApp gagal mengirim stiker (" + resumen.resultado + ")");
    return msg;
  };

  W.waitForAck = async function (msg, timeoutMs = 30000) {
    const ACK = ns("WAWebAck").ACK;
    const t0 = Date.now();
    while (Date.now() - t0 < timeoutMs) {
      const a = msg.ack;
      if (a >= ACK.SENT) { log("info", "envio:ack", { ack: a, ms: Date.now() - t0 }); return a; }
      if (a < 0) throw new Error("Pesan gagal dikirim (ack " + a + ")");
      await new Promise((r) => setTimeout(r, 300));
    }
    log("warn", "envio:ack-tiempo-agotado", { ack: msg.ack });
    return msg.ack;
  };

  W.openForward = async function (msg) {
    const React = ns("react");
    const MM = ns("WAWebModalManager");
    const FL = await load("WAWebForwardMessageFlowLoadable");
    const SU = ns("WAWebStateUtils");
    let model = msg;
    try {
      const MC = ns("WAWebMsgCollection").MsgCollection;
      model = MC.get(msg.id) || msg;
    } catch (e) { /* usar msg */ }
    const close = MM.closeModalManager || (() => MM.ModalManager.close());
    MM.ModalManager.open(
      React.createElement(FL.ForwardMessageFlowLoadable, {
        msgs: [SU.unproxy(model)],
        onClose: close,
        onForward: (msgs) => log("info", "reenvio:hecho", { mensajes: msgs ? msgs.length : null }),
      }),
      { transition: "modal-flow" }
    );
    log("info", "reenvio:dialogo-abierto", null);
  };

  // Flujo completo: subir audio, enviar al chat de destino y (opcional) abrir Reenviar.
  W.publish = async function ({ webp, ogg, info }, { destino = "propio", abrirReenviar = true, onProgress } = {}) {
    const prog = onProgress || (() => {});
    W.hookWALogger();
    if (!W.forceSoundStickers()) log("warn", "publicar:gating", "Gagal mengaktifkan fitur stiker bersuara; audio mungkin terabaikan");
    prog("Mencari chat tujuan…");
    const chat = destino === "abierto" ? W.getActiveChat() : await W.getSelfChat();
    if (!chat) throw new Error(destino === "abierto" ? "Buka chat terlebih dahulu" : "Chat Anda sendiri tidak ditemukan");
    prog("Mengunggah audio…");
    const stickerAudio = await W.uploadAudio(ogg, info.segundosAudio, chat);
    prog("Mengirim stiker…");
    const msg = await W.sendSticker(chat, webp, info, stickerAudio);
    if (abrirReenviar) {
      prog("Menunggu konfirmasi server…");
      await W.waitForAck(msg);
      prog("Membuka dialog Teruskan…");
      await W.openForward(msg);
    }
    return { chat, msg };
  };
})();
