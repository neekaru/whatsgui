// Interfaz: panel en Shadow DOM que se abre con el botón de la extensión.
// Tres pestañas, cada una con su formulario: video con su sonido, imagen + audio, y video o GIF + otro audio.
// Sin innerHTML (WhatsApp Web usa Trusted Types): todo el DOM se crea con createElement.
(() => {
  const WAAS = (window.WAAS = window.WAAS || {});
  const log = (lvl, ev, data) => WAAS.log(lvl, ev, data);
  const FRAME = 1 / 30;     // paso del ajuste fino
  const MIN_LEN = 0.3;      // duración mínima de un tramo
  const DEFAULT_LEN = 10;   // tramo inicial al abrir un archivo

  const CSS = `
  :host { all: initial; }
  * { box-sizing: border-box; font-family: system-ui, "Segoe UI", Roboto, sans-serif; }
  .overlay { position: fixed; inset: 0; z-index: 2147483001; background: rgba(0,0,0,.5); display: flex; align-items: center; justify-content: center; }
  .panel { --bg:#ffffff; --fg:#111b21; --muted:#667781; --card:#f0f2f5; --line:#d1d7db; --accent:#00a884; --accent-fg:#fff;
           --ok:#25d366; --ok-bg:rgba(37,211,102,.18); --warn:#d29a00; --warn-bg:rgba(240,180,0,.2); --no:#8696a0; --err:#d13b3b;
           width: min(680px, 96vw); max-height: 94vh; overflow: auto; background: var(--bg); color: var(--fg);
           border-radius: 14px; padding: 18px 20px 14px; box-shadow: 0 12px 48px rgba(0,0,0,.45); font-size: 14px; }
  .panel.dark { --bg:#111b21; --fg:#e9edef; --muted:#8696a0; --card:#202c33; --line:#2a3942; --ok-bg:rgba(37,211,102,.22); --warn-bg:rgba(240,180,0,.22); }
  header { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; margin-bottom: 8px; }
  h2 { margin: 0; font-size: 19px; }
  .muted { color: var(--muted); font-size: 12px; }
  .hdr-r { display: flex; align-items: center; gap: 8px; flex: none; }
  .kofi { display: inline-flex; align-items: center; gap: 4px; padding: 5px 13px; border-radius: 18px; background: #fff3cf; border: 1px solid #f2c14e;
          color: #7a4b00; font-size: 12.5px; font-weight: 700; text-decoration: none; white-space: nowrap; transition: background .12s, border-color .12s; }
  .panel.dark .kofi { background: rgba(255,196,60,.12); border-color: rgba(255,196,60,.45); color: #ffcf66; }
  .kofi:hover { background: #ffe9a8; }
  .panel.dark .kofi:hover { background: rgba(255,196,60,.22); }
  .note { color: var(--warn); font-size: 12.5px; }
  .tabs { display: flex; gap: 4px; border-bottom: 1px solid var(--line); margin: 6px 0 4px; }
  .tabs button { border: none; background: none; color: var(--muted); padding: 8px 12px; font-size: 13.5px; font-weight: 600; border-bottom: 3px solid transparent; margin-bottom: -1px; }
  .tabs button.on { color: var(--fg); border-bottom-color: var(--accent); }
  .tabdesc { margin: 6px 2px 0; }
  section { background: var(--card); border-radius: 10px; padding: 12px 14px; margin: 10px 0; }
  section.off { opacity: .45; pointer-events: none; }
  h3 { margin: 0 0 8px; font-size: 14px; display: flex; align-items: center; gap: 8px; }
  h3 .n { display: inline-flex; width: 22px; height: 22px; border-radius: 50%; background: var(--accent); color: var(--accent-fg); align-items: center; justify-content: center; font-size: 12px; }
  .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin: 6px 0; }
  .grow { flex: 1; }
  button { font: inherit; cursor: pointer; }
  button:disabled { opacity: .45; cursor: default; }
  .act { padding: 8px 16px; border-radius: 18px; border: none; background: var(--accent); color: var(--accent-fg); font-weight: 600; }
  .sec { padding: 5px 11px; border-radius: 14px; border: 1px solid var(--line); background: var(--bg); color: var(--fg); font-size: 12.5px; }
  .icon { width: 28px; padding: 4px 0; text-align: center; }
  .close { background: none; border: none; font-size: 22px; color: var(--muted); line-height: 1; }
  input[type=text] { width: 200px; padding: 4px 8px; border-radius: 6px; border: 1px solid var(--line); background: var(--bg); color: var(--fg); font: inherit; }
  .meta { display: grid; grid-template-columns: auto 1fr; gap: 6px 10px; align-items: center; margin: 4px 0 10px; }
  input[type=number] { width: 78px; padding: 4px 6px; border-radius: 6px; border: 1px solid var(--line); background: var(--bg); color: var(--fg); font: inherit; }
  select { padding: 4px 6px; border-radius: 6px; border: 1px solid var(--line); background: var(--bg); color: var(--fg); font: inherit; max-width: 100%; }
  label.chk { display: flex; gap: 8px; align-items: flex-start; margin: 6px 0; }
  video { display: block; width: 100%; max-height: 260px; background: #000; border-radius: 8px; }
  .thumb { width: 150px; height: 150px; object-fit: contain; border-radius: 8px; background: repeating-conic-gradient(#bbb 0 25%, #eee 0 50%) 50% / 18px 18px; }
  .track { position: relative; height: 34px; margin: 12px 8px 6px; background: var(--line); border-radius: 6px; cursor: pointer; touch-action: none; user-select: none; }
  .track.audio { height: 48px; }
  .track canvas { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
  .track .sel { position: absolute; top: 0; bottom: 0; background: rgba(0,168,132,.35); border-top: 2px solid var(--accent); border-bottom: 2px solid var(--accent); pointer-events: none; }
  .track .handle { position: absolute; top: -5px; bottom: -5px; width: 14px; margin-left: -7px; background: var(--accent); border-radius: 4px; cursor: ew-resize; }
  .track .handle::after { content: ""; position: absolute; left: 4px; top: 10px; bottom: 10px; width: 4px; border-left: 1px solid rgba(255,255,255,.8); border-right: 1px solid rgba(255,255,255,.8); }
  .track .ph { position: absolute; top: -8px; bottom: -8px; width: 2px; margin-left: -1px; background: var(--fg); pointer-events: none; }
  .times { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); margin: 0 8px; }
  .trim { display: grid; grid-template-columns: auto auto 1fr; gap: 6px 10px; align-items: center; }
  .seg { display: inline-flex; border: 1px solid var(--line); border-radius: 16px; overflow: hidden; }
  .seg button { border: none; padding: 5px 12px; background: var(--bg); color: var(--fg); font-size: 12.5px; }
  .seg button.on { background: var(--accent); color: var(--accent-fg); }
  .summary { background: var(--bg); border-radius: 8px; padding: 8px 10px; margin: 6px 0; font-size: 13px; }
  table.grid { border-collapse: separate; border-spacing: 3px; margin: 6px 0; font-size: 11.5px; }
  table.grid th { color: var(--muted); font-weight: 600; padding: 2px 4px; text-align: center; }
  table.grid th.side { text-align: right; }
  table.grid td button { width: 52px; padding: 5px 0; border-radius: 6px; border: 1px solid transparent; font-size: 11.5px; background: transparent; color: var(--no); }
  table.grid td button.ok { background: var(--ok-bg); color: var(--fg); }
  table.grid td button.baja { background: var(--warn-bg); color: var(--fg); }
  table.grid td button.no, table.grid td button.fuera { cursor: default; opacity: .6; }
  table.grid td button.sel { border-color: var(--fg); font-weight: 700; }
  .legend { display: flex; gap: 12px; flex-wrap: wrap; font-size: 11.5px; color: var(--muted); }
  .legend i { display: inline-block; width: 12px; height: 12px; border-radius: 3px; vertical-align: -2px; margin-right: 4px; }
  .stale { color: var(--warn); font-size: 12.5px; font-weight: 600; }
  .bar { height: 6px; background: var(--line); border-radius: 3px; overflow: hidden; margin-top: 8px; }
  .bar > div { height: 100%; width: 0; background: var(--accent); transition: width .2s; }
  .status { font-size: 13px; margin: 6px 0 0; min-height: 18px; }
  .status.err { color: var(--err); font-weight: 600; }
  .result { display: flex; gap: 14px; align-items: center; flex-wrap: wrap; margin-top: 10px; }
  details { margin-top: 8px; }
  summary { cursor: pointer; color: var(--muted); font-size: 12.5px; }
  .logbox { max-height: 150px; overflow: auto; font: 11px/1.35 ui-monospace, Consolas, monospace; background: var(--bg); padding: 6px 8px; border-radius: 6px; white-space: pre-wrap; margin-top: 6px; }
  .logbox .e { color: var(--err); } .logbox .w { color: var(--warn); }
  `;

  function h(tag, props, ...children) {
    const el = document.createElement(tag);
    if (props) {
      for (const [k, v] of Object.entries(props)) {
        if (k === "class") el.className = v;
        else if (k === "style") el.style.cssText = v;
        else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
        else if (k in el) el[k] = v;
        else el.setAttribute(k, v);
      }
    }
    for (const c of children.flat()) if (c != null) el.append(c.nodeType ? c : document.createTextNode(String(c)));
    return el;
  }

  function attachStyles(root) {
    try {
      const sheet = new CSSStyleSheet();
      sheet.replaceSync(CSS);
      root.adoptedStyleSheets = [sheet];
    } catch (e) {
      log("warn", "ui:estilos-fallback", e);
      root.append(h("style", { textContent: CSS }));
    }
  }

  function download(blob, name) {
    const a = h("a", { href: URL.createObjectURL(blob), download: name });
    document.body.append(a);
    a.click();
    setTimeout(() => { URL.revokeObjectURL(a.href); a.remove(); }, 2000);
  }

  async function saveLog() {
    const stored = await WAAS.getStoredLog();
    const entries = stored && stored.length ? stored : WAAS.entries;
    const content = JSON.stringify({ exportado: new Date().toISOString(), version: WAAS.VERSION, ua: navigator.userAgent, entradas: entries }, null, 1);
    const blob = new Blob([content], { type: "application/json" });
    const name = "registro-stickers-con-sonido-" + new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19) + ".json";
    if (window.showSaveFilePicker) {
      try {
        const handle = await window.showSaveFilePicker({ suggestedName: name, types: [{ description: "Registro JSON", accept: { "application/json": [".json"] } }] });
        const w = await handle.createWritable();
        await w.write(blob);
        await w.close();
        log("info", "registro:guardado", { nombre: handle.name, entradas: entries.length });
        return;
      } catch (e) {
        if (e && e.name === "AbortError") return;
        log("warn", "registro:selector-fallo", e);
      }
    }
    download(blob, name);
    log("info", "registro:descargado", { nombre: name, entradas: entries.length });
  }
  WAAS.saveLog = saveLog;

  const fmtT = (t) => (t >= 60 ? Math.floor(t / 60) + ":" + (t % 60).toFixed(2).padStart(5, "0") : t.toFixed(2) + " s");
  const fmtB = (b) => (b >= 1e6 ? (b / 1e6).toFixed(2) + " MB" : Math.round(b / 1000) + " KB");
  const isDark = () => {
    try {
      return document.body.classList.contains("dark") || document.documentElement.classList.contains("dark") ||
        matchMedia("(prefers-color-scheme: dark)").matches;
    } catch (e) { return false; }
  };
  const store = {
    get(k, def) { try { const v = localStorage.getItem(k); return v == null ? def : JSON.parse(v); } catch (e) { return def; } },
    set(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) { /* sin almacenamiento */ } },
  };
  const baseName = (f) => (f && f.name.replace(/\.[^.]+$/, "")) || "sticker";

  function mount() {
    if (document.getElementById("waas-root")) return;
    const host = h("div", { id: "waas-root" });
    document.body.append(host);
    const root = host.attachShadow({ mode: "open" });
    attachStyles(root);

    // Estado compartido: solo puede haber una operación a la vez en todo el panel.
    const G = { busy: false, forms: [], refreshTabs: () => {} };
    const setBusy = (b) => { G.busy = b; G.forms.forEach((f) => f.refresh()); G.refreshTabs(); };
    const run = async (name, fn) => {
      const ac = new AbortController();
      G.abort = ac;
      setBusy(true);
      try { return await WAAS.run(name, () => fn(ac.signal)); } finally { G.abort = null; setBusy(false); }
    };

    // ============================================================ línea de tiempo con asas
    // kind "video": muestra el video y salta al cuadro donde quedan las asas.
    // kind "audio": muestra la forma de onda; el reproductor queda oculto.
    function makeTrimmer(kind, hint) {
      const isVideo = kind === "video";
      const media = isVideo ? h("video", { playsInline: true, preload: "auto" }) : h("audio", { preload: "auto" });
      const wave = isVideo ? null : h("canvas");
      const selBar = h("div", { class: "sel" });
      const hS = h("div", { class: "handle", title: "Inicio del tramo (arrastra)" });
      const hE = h("div", { class: "handle", title: "Fin del tramo (arrastra)" });
      const ph = h("div", { class: "ph" });
      const track = h("div", { class: "track" + (isVideo ? "" : " audio"), title: "Haz clic o arrastra para moverte" }, wave, selBar, ph, hS, hE);
      const tLeft = h("span"), tRight = h("span");
      const playSegBtn = h("button", { class: "sec", textContent: "▶ Tramo" });
      const playCurBtn = h("button", { class: "sec", textContent: "▶ Desde el cursor" });
      const playEndBtn = h("button", { class: "sec", textContent: "▶ Final del tramo" });
      const curLbl = h("span", { class: "muted grow", style: "text-align:right" });
      const startNum = h("input", { type: "number", min: 0, step: 0.01 });
      const endNum = h("input", { type: "number", min: 0, step: 0.01 });
      const startCur = h("button", { class: "sec", textContent: "⤓ cursor", title: "Usar la posición del cursor como inicio" });
      const endCur = h("button", { class: "sec", textContent: "⤓ cursor", title: "Usar la posición del cursor como fin" });
      const lenLbl = h("span", { class: "muted" });
      const t = { start: 0, end: 0, playhead: 0, stopAt: null, dur: 0, offset: 0, url: null, peaks: null, peakMax: 1, muted: false };
      const listeners = new Set();
      const emit = () => listeners.forEach((fn) => fn());
      const mkStep = (txt, title, fn) => h("button", { class: "sec icon", textContent: txt, title, onclick: fn });
      const el = h("div", null,
        isVideo ? media : null, track, h("div", { class: "times" }, tLeft, tRight),
        h("div", { class: "row" }, playSegBtn, playCurBtn, playEndBtn, curLbl),
        h("div", { class: "trim" },
          h("span", null, "Inicio"),
          h("span", { class: "row", style: "margin:0" }, mkStep("−", "Un cuadro antes", () => setStart(t.start - FRAME)), startNum, mkStep("+", "Un cuadro después", () => setStart(t.start + FRAME))),
          h("span", { class: "row", style: "margin:0" }, startCur),
          h("span", null, "Fin"),
          h("span", { class: "row", style: "margin:0" }, mkStep("−", "Un cuadro antes", () => setEnd(t.end - FRAME)), endNum, mkStep("+", "Un cuadro después", () => setEnd(t.end + FRAME))),
          h("span", { class: "row", style: "margin:0" }, endCur, lenLbl)),
        hint ? h("div", { class: "muted", style: "margin-top:6px" }, hint) : null);

      const layout = () => {
        const d = t.dur || 1;
        const pct = (x) => Math.max(0, Math.min(100, (x / d) * 100)) + "%";
        selBar.style.left = pct(t.start);
        selBar.style.width = Math.max(0, ((t.end - t.start) / d) * 100) + "%";
        hS.style.left = pct(t.start);
        hE.style.left = pct(t.end);
        ph.style.left = pct(t.playhead);
        tLeft.textContent = "0 s";
        tRight.textContent = fmtT(t.dur);
        curLbl.textContent = "cursor " + fmtT(t.playhead);
        if (root.activeElement !== startNum) startNum.value = t.start.toFixed(2);
        if (root.activeElement !== endNum) endNum.value = t.end.toFixed(2);
        lenLbl.textContent = "duración " + fmtT(t.end - t.start);
      };

      const drawWave = () => {
        if (!wave) return;
        const w = track.clientWidth, hh = track.clientHeight;
        if (!w) return;
        const dpr = window.devicePixelRatio || 1;
        wave.width = Math.round(w * dpr); wave.height = Math.round(hh * dpr);
        const ctx = wave.getContext("2d");
        ctx.clearRect(0, 0, wave.width, wave.height);
        if (!t.peaks) return;
        ctx.fillStyle = getComputedStyle(track).getPropertyValue("--muted").trim() || "#667781";
        const n = t.peaks.length, cols = wave.width, mid = wave.height / 2, norm = 1 / (t.peakMax || 1);
        for (let x = 0; x < cols; x++) {
          const a = Math.floor((x / cols) * n), b = Math.max(a + 1, Math.floor(((x + 1) / cols) * n));
          let p = 0;
          for (let i = a; i < b && i < n; i++) if (t.peaks[i] > p) p = t.peaks[i];
          const hgt = Math.max(1, p * norm * (wave.height - 4));
          ctx.fillRect(x, mid - hgt / 2, 1, hgt);
        }
      };
      if (wave && typeof ResizeObserver !== "undefined") new ResizeObserver(drawWave).observe(track);

      // Saltos encadenados: el siguiente se pide cuando terminó el anterior (arrastre fluido).
      let seeking = false, pendingSeek = null;
      const seekTo = (x) => { pendingSeek = x; if (!seeking) doSeek(); };
      const doSeek = () => {
        if (pendingSeek == null) return;
        const x = pendingSeek; pendingSeek = null; seeking = true;
        t.playhead = x;
        media.currentTime = t.offset + x;
        layout();
      };
      media.addEventListener("seeked", () => { seeking = false; if (pendingSeek != null) doSeek(); });

      const changed = () => { layout(); emit(); };
      const setStart = (x, seek = true) => {
        if (!t.dur) return;
        t.start = Math.max(0, Math.min(x, t.end - MIN_LEN));
        changed();
        if (seek) { stop(); seekTo(t.start); }
      };
      const setEnd = (x, seek = true) => {
        if (!t.dur) return;
        t.end = Math.min(t.dur, Math.max(x, t.start + MIN_LEN));
        changed();
        if (seek) { stop(); seekTo(Math.max(t.start, t.end - FRAME)); }
      };

      const tAt = (ev) => {
        const r = track.getBoundingClientRect();
        return Math.max(0, Math.min(1, (ev.clientX - r.left) / r.width)) * t.dur;
      };
      let drag = null;
      track.addEventListener("pointerdown", (ev) => {
        if (!t.dur || G.busy) return;
        drag = ev.target === hS ? "s" : ev.target === hE ? "e" : "p";
        track.setPointerCapture(ev.pointerId);
        stop();
        applyDrag(ev);
      });
      track.addEventListener("pointermove", (ev) => { if (drag) applyDrag(ev); });
      const endDrag = () => { drag = null; };
      track.addEventListener("pointerup", endDrag);
      track.addEventListener("pointercancel", endDrag);
      const applyDrag = (ev) => {
        const x = tAt(ev);
        if (drag === "s") setStart(x);
        else if (drag === "e") setEnd(x);
        else seekTo(x);
      };
      startNum.addEventListener("change", () => setStart(+startNum.value || 0));
      endNum.addEventListener("change", () => setEnd(+endNum.value || 0));
      startCur.addEventListener("click", () => setStart(t.playhead));
      endCur.addEventListener("click", () => setEnd(t.playhead));

      let raf = 0;
      const tick = () => {
        t.playhead = Math.max(0, media.currentTime - t.offset);
        layout();
        if (t.stopAt != null && media.currentTime >= t.offset + t.stopAt) { stop(); return; }
        if (!media.paused) raf = requestAnimationFrame(tick);
      };
      function stop() {
        cancelAnimationFrame(raf);
        if (!media.paused) media.pause();
        t.stopAt = null;
      }
      const play = (from, to, muted = t.muted) => {
        stop();
        t.stopAt = to;
        media.muted = muted;
        seeking = false; pendingSeek = null;
        media.currentTime = t.offset + from;
        return media.play().then(() => { raf = requestAnimationFrame(tick); }).catch((e) => log("warn", "ui:reproducir", e));
      };
      const toggleRange = (from, to) => { if (!media.paused) stop(); else play(from, to); };
      playSegBtn.addEventListener("click", () => toggleRange(t.start, t.end));
      playCurBtn.addEventListener("click", () => toggleRange(t.playhead >= t.start && t.playhead < t.end - 0.1 ? t.playhead : t.start, t.end));
      playEndBtn.addEventListener("click", () => toggleRange(Math.max(t.start, t.end - 2), t.end));

      const load = (file, { duration, offset = 0, peaks = null, peakMax = 1 }) => {
        stop();
        if (t.url) URL.revokeObjectURL(t.url);
        t.url = URL.createObjectURL(file);
        Object.assign(t, { dur: duration, offset, start: 0, end: Math.min(duration, DEFAULT_LEN), playhead: 0, peaks, peakMax });
        media.addEventListener("loadeddata", () => seekTo(0), { once: true });
        media.src = t.url;
        changed();
        requestAnimationFrame(drawWave);
      };
      const clear = () => {
        stop();
        if (t.url) URL.revokeObjectURL(t.url);
        t.url = null;
        media.removeAttribute("src");
        Object.assign(t, { dur: 0, start: 0, end: 0, playhead: 0, peaks: null });
        changed();
        drawWave();
      };

      layout();
      return {
        el, media, load, clear, stop, play, layout, drawWave,
        onChange: (fn) => listeners.add(fn),
        setMuted: (m) => { t.muted = m; media.muted = m; },
        get loaded() { return t.dur > 0; },
        get start() { return t.start; },
        get end() { return t.end; },
        get len() { return t.end - t.start; },
        get key() { return t.start.toFixed(3) + "-" + t.end.toFixed(3); },
      };
    }

    // ============================================================ secciones comunes
    const section = (n, title, ...body) => h("section", { class: "off" }, h("h3", null, h("span", { class: "n" }, String(n)), title), ...body);

    // Selector de archivo: botón + texto de estado.
    function makePicker(label, accept, hint) {
      const input = h("input", { type: "file", accept, style: "display:none" });
      const btn = h("button", { class: "act", textContent: label });
      const info = h("span", { class: "muted" }, hint);
      btn.addEventListener("click", () => input.click());
      return {
        el: h("div", { class: "row" }, btn, info, input), btn, info,
        onPick: (fn) => input.addEventListener("change", () => {
          const f = input.files && input.files[0];
          input.value = "";
          if (f) fn(f);
        }),
      };
    }

    // Sección de audio aparte (pestañas 2 y 3): archivo + tramo con forma de onda.
    function makeAudioSource(onChange) {
      const picker = makePicker("Elegir audio…", "audio/*,video/*,.mp3,.m4a,.aac,.ogg,.opus,.wav,.flac,.mp4,.mov,.webm",
        "MP3, M4A, OGG, WAV… o un video del que tomar el sonido.");
      const trimmer = makeTrimmer("audio", "Elige de dónde a dónde suena. Arrastra las asas o escribe los segundos.");
      const wrap = h("div", { style: "display:none" }, trimmer.el);
      const s = { file: null };
      picker.onPick(async (f) => {
        picker.info.textContent = "Leyendo " + f.name + "…";
        await run("audio", async () => {
          try {
            const a = await WAAS.probeAudio(f);
            s.file = f;
            trimmer.load(f, { duration: a.duration, peaks: a.peaks, peakMax: a.peakMax });
            wrap.style.display = "";
            picker.info.textContent = `${f.name} · ${fmtT(a.duration)}${a.peakMax < 0.001 ? " · parece no tener sonido" : ""}`;
            log("info", "ui:audio-elegido", { nombre: f.name, tipo: f.type, bytes: f.size, duracion: a.duration, canales: a.channels, pico: +a.peakMax.toFixed(4) });
          } catch (e) {
            log("error", "ui:audio-error", e);
            picker.info.textContent = "No se pudo leer: " + e.message;
            s.file = null;
            trimmer.clear();
            wrap.style.display = "none";
          }
        });
        onChange();
      });
      trimmer.onChange(onChange);
      return {
        els: [picker.el, wrap], trimmer, picker,
        get ready() { return !!s.file && trimmer.loaded; },
        get params() { return s.file ? { file: s.file, start: trimmer.start, dur: trimmer.len } : null; },
        get key() { return s.file ? s.file.name + s.file.size + "|" + trimmer.key : ""; },
      };
    }

    // Formato: encuadre + (opcional) tabla de resolución × fps.
    // getParams() devuelve { visual, audio, fit } o null si faltan datos.
    function makeFormat({ getParams, grid = true, sync = true, still = false, extras = [] }) {
      const f = { mode: "ajustar", an: null, anKey: null, sel: null };
      const segAjustar = h("button", { textContent: "Ajustar (bordes transparentes)", class: "on" });
      const segRecortar = h("button", { textContent: "Recortar (llenar el cuadro)" });
      const calcBtn = h("button", { class: "act", textContent: "Calcular opciones" });
      const calcInfo = h("span", { class: "muted" });
      const staleLbl = h("div", { class: "stale", style: "display:none" }, "Algo cambió: vuelve a calcular las opciones.");
      // Barra propia del cálculo (la de "Crear y enviar" es solo para crear).
      const calcBarIn = h("div");
      const calcBar = h("div", { class: "bar", style: "display:none" }, calcBarIn);
      const gridHost = h("div");
      const selLbl = h("div", { class: "row", style: "font-weight:600" });
      const legend = h("div", { class: "legend" },
        h("span", null, h("i", { style: "background:var(--ok-bg);border:1px solid var(--ok)" }), "cabe"),
        h("span", null, h("i", { style: "background:var(--warn-bg);border:1px solid var(--warn)" }), "cabe bajando la calidad"),
        h("span", null, h("i", { style: "border:1px solid var(--line)" }), "✕ supera 1 MB (WhatsApp lo rechaza)"),
        h("span", null, "★ recomendada · valores en KB"));
      const syncNum = h("input", { type: "number", min: 0, max: 1000, step: 10, value: store.get("waas-compensacion-ms", 0) });
      syncNum.addEventListener("change", () => { store.set("waas-compensacion-ms", +syncNum.value || 0); api.onUpdate(); });
      const stillChk = h("input", { type: "checkbox", onchange: () => api.onUpdate() });
      const advRows = [];
      if (sync) advRows.push(h("div", { class: "row" }, h("span", null, "Compensar desfase (ms)"), syncNum,
        h("span", { class: "muted" }, "Normalmente 0. Si en algún celular el audio va atrasado, la imagen esperará este tiempo.")));
      if (still) advRows.push(h("label", { class: "chk" }, stillChk, h("span", null, "Enviar como imagen fija (experimental)",
        h("div", { class: "muted" }, "Por defecto la imagen se envía como un sticker animado de un solo cuadro que dura lo que el audio, igual que los stickers con sonido conocidos. Marca esto para probar si WhatsApp también acepta el sonido en un sticker fijo."))));
      const advanced = advRows.length ? h("details", null, h("summary", null, "Avanzado"), ...advRows) : null;

      // Clave del análisis: solo lo que cambia los cuadros del sticker. El audio se sube aparte (no
      // suma al peso del sticker), así que solo cuenta su duración cuando la animación se ajusta a él.
      const fileId = (x) => (x ? x.name + "|" + x.size + "|" + x.lastModified : "");
      const key = () => {
        const p = getParams();
        if (!p) return null;
        const v = p.visual;
        const followsAudio = v.kind === "image" || p.fit !== "independiente";
        return JSON.stringify([v.kind, fileId(v.file), v.start, v.dur, followsAudio ? p.fit : "", followsAudio ? +p.audio.dur.toFixed(3) : "", f.mode]);
      };
      // Clave del resultado: todo lo que cambia el sticker o su audio (el paquete y el autor se aplican al enviar).
      const fullKey = () => {
        const p = getParams();
        if (!p) return null;
        return JSON.stringify([key(), fileId(p.audio.file), +p.audio.start.toFixed(3), +p.audio.dur.toFixed(3), p.fit, f.sel, api.options()]);
      };
      const fresh = () => !!f.an && f.anKey === key();
      const CALC_HINT = "Lee el tramo una vez y estima el peso de cada combinación.";
      const reset = () => { if (f.an) WAAS.release(f.an); f.an = null; f.sel = null; gridHost.textContent = ""; selLbl.textContent = ""; calcInfo.textContent = CALC_HINT; };
      calcInfo.textContent = CALC_HINT;

      const renderGrid = () => {
        gridHost.textContent = "";
        const an = f.an;
        if (!an) return;
        const L = WAAS.limits;
        const thead = h("tr", null, h("th", { class: "side" }, "px \\ fps"), L.FPS_LIST.map((x) => h("th", null, String(x))));
        const rows = L.SIDES.map((side) => h("tr", null, h("th", { class: "side" }, String(side)), L.FPS_LIST.map((fps) => {
          const c = an.cells.find((x) => x.side === side && x.fps === fps);
          const isAuto = an.auto && an.auto.side === side && an.auto.fps === fps;
          const isSel = f.sel && f.sel.side === side && f.sel.fps === fps;
          const can = !!c && (c.estado === "ok" || c.estado === "baja");
          const label = !c || c.estado === "fuera" ? "·" : c.estado === "no" ? "✕" : (isAuto ? "★" : "") + Math.round(c.est / 1000);
          const title = !c || c.estado === "fuera" ? "No disponible para este tramo"
            : c.estado === "no" ? `≈ ${fmtB(c.estMin)} aun con la calidad mínima: supera 1 MB`
            : c.estado === "baja" ? `≈ ${fmtB(c.est)} con la calidad normal: se bajará la calidad para que quepa`
            : `≈ ${fmtB(c.est)}`;
          return h("td", null, h("button", {
            class: (c ? c.estado : "fuera") + (isSel ? " sel" : ""), textContent: label, title, disabled: !can,
            onclick: () => { f.sel = { side, fps }; renderGrid(); api.onUpdate(); },
          }));
        })));
        gridHost.append(h("table", { class: "grid" }, thead, rows));
        const c = f.sel && an.cells.find((x) => x.side === f.sel.side && x.fps === f.sel.fps);
        selLbl.textContent = c ? `Elegido: ${c.side} px · ${c.fps} fps · ≈ ${fmtB(c.est)}${c.estado === "baja" ? " (con menos calidad)" : ""}` : "Ninguna combinación cabe en 1 MB: acorta el tramo.";
      };

      segAjustar.addEventListener("click", () => { f.mode = "ajustar"; api.onUpdate(); });
      segRecortar.addEventListener("click", () => { f.mode = "recortar"; api.onUpdate(); });
      calcBtn.addEventListener("click", async () => {
        const p = getParams();
        if (!p) return;
        reset();
        const k = key();
        api.onUpdate();
        try {
          const an = await run("analizar", (signal) => WAAS.analyze({
            ...p, mode: f.mode, signal,
            onProgress: (txt, frac) => { calcInfo.textContent = txt; api.setProgress(frac); },
          }));
          f.an = an; f.anKey = k;
          f.sel = an.auto ? { side: an.auto.side, fps: an.auto.fps } : null;
          calcInfo.textContent = `Listo en ${(an.ms / 1000).toFixed(1)} s. Elige una casilla (★ = recomendada).`;
          renderGrid();
        } catch (e) {
          calcInfo.textContent = e.name === "AbortError" ? "Cancelado" : "Error: " + e.message;
        }
        api.setProgress(0);
        api.onUpdate();
      });

      const body = [...extras, h("div", { class: "row" }, h("span", null, "Encuadre"), h("span", { class: "seg" }, segAjustar, segRecortar))];
      if (grid) body.push(h("div", { class: "row" }, calcBtn, calcInfo), calcBar, staleLbl, gridHost, selLbl, legend);
      if (advanced) body.push(advanced);

      const api = {
        body,
        onUpdate: () => {},
        setProgress: (frac) => { calcBar.style.display = frac > 0 ? "" : "none"; calcBarIn.style.width = Math.round(frac * 100) + "%"; },
        refresh() {
          const has = !!getParams();
          const stale = !!f.an && !fresh();
          staleLbl.style.display = stale ? "" : "none";
          gridHost.style.opacity = stale ? ".35" : "";
          gridHost.style.pointerEvents = stale ? "none" : "";
          calcBtn.disabled = G.busy || !has;
          segAjustar.disabled = segRecortar.disabled = G.busy;
          segAjustar.classList.toggle("on", f.mode === "ajustar");
          segRecortar.classList.toggle("on", f.mode === "recortar");
        },
        // ¿Se puede crear ya? Con tabla hace falta haber calculado y elegido una casilla.
        get ready() { return grid ? fresh() && !!f.sel : !!getParams(); },
        reset,
        fullKey,
        options: () => ({ syncMs: sync ? +syncNum.value || 0 : 0, still: still && stillChk.checked }),
        // Devuelve el análisis y la combinación para crear. Sin tabla se analiza en el momento.
        async prepare(signal, onProgress) {
          const p = getParams();
          if (grid) return { an: f.an, side: f.sel.side, fps: f.sel.fps, audio: p.audio, temp: false };
          const an = await WAAS.analyze({ ...p, mode: f.mode, signal, onProgress });
          return { an, side: an.auto.side, fps: an.auto.fps, audio: p.audio, temp: true };
        },
      };
      return api;
    }

    // Crear y enviar: datos del paquete, creación, vista previa y envío.
    function makeCreate({ fmt, getBase }) {
      const META_KEY = "waas-metadatos";
      const packInput = h("input", { type: "text", maxLength: 128, placeholder: "Ej.: Mis memes" });
      const authorInput = h("input", { type: "text", maxLength: 128, placeholder: "Ej.: tu nombre o apodo" });
      const loadMeta = () => { const m = store.get(META_KEY, {}) || {}; packInput.value = m.paquete || ""; authorInput.value = m.autor || ""; };
      const saveMeta = () => store.set(META_KEY, { paquete: packInput.value, autor: authorInput.value });
      loadMeta();
      packInput.addEventListener("change", saveMeta);
      authorInput.addEventListener("change", saveMeta);
      const createBtn = h("button", { class: "act", textContent: "Crear sticker" });
      const cancelBtn = h("button", { class: "sec", textContent: "Cancelar", style: "display:none" });
      const barIn = h("div");
      const status = h("div", { class: "status" });
      const resImg = h("img", { class: "thumb", alt: "vista previa del sticker" });
      const resAudio = h("audio", { controls: true });
      const resInfo = h("div", { class: "muted", style: "max-width:380px" });
      const togetherBtn = h("button", { class: "sec", textContent: "▶ Ver y oír juntos" });
      const destSel = h("select", null,
        h("option", { value: "abierto-solo", textContent: "Enviar al chat abierto" }),
        h("option", { value: "abierto", textContent: "Enviar al chat abierto y abrir Reenviar" }),
        h("option", { value: "propio", textContent: "Guardar en mi chat (Tú) y abrir Reenviar" }));
      const sendBtn = h("button", { class: "act", textContent: "Enviar" });
      const dlBtn = h("button", { class: "sec", textContent: "Descargar .webp y .ogg" });
      const staleNote = h("div", { class: "stale", style: "display:none;margin-top:8px" },
        "Cambiaste algo después de crear este sticker. Pulsa «Crear sticker con los cambios»; si lo envías ahora, se envía la versión anterior.");
      const resultBox = h("div", { class: "result" }, resImg, h("div", null, resAudio, h("div", { class: "row" }, togetherBtn), resInfo));
      const result = h("div", { style: "display:none" }, staleNote, resultBox,
        h("div", { class: "row", style: "margin-top:10px" }, destSel, sendBtn, dlBtn));
      const c = { result: null, key: null, urls: [] };
      const setStatus = (txt, frac, isErr) => {
        status.textContent = txt || "";
        status.className = "status" + (isErr ? " err" : "");
        if (frac != null) barIn.style.width = Math.round(frac * 100) + "%";
      };
      const revoke = () => { c.urls.forEach((u) => URL.revokeObjectURL(u)); c.urls = []; };
      const clear = () => { c.result = null; c.key = null; result.style.display = "none"; revoke(); setStatus("", 0); };

      createBtn.addEventListener("click", async () => {
        if (!fmt.ready) return;
        result.style.display = "none";
        try {
          const k = fmt.fullKey();
          const res = await run("crear", async (signal) => {
            const p = await fmt.prepare(signal, (t, f) => setStatus(t, f * 0.3));
            try {
              return await WAAS.create(p.an, {
                side: p.side, fps: p.fps, audio: p.audio, ...fmt.options(),
                packName: packInput.value, publisher: authorInput.value,
                onProgress: (t, f) => setStatus(t, f), signal,
              });
            } finally {
              if (p.temp) WAAS.release(p.an);
            }
          });
          c.result = res;
          c.key = k;
          revoke();
          const u1 = URL.createObjectURL(res.webp), u2 = URL.createObjectURL(res.ogg);
          c.urls.push(u1, u2);
          resImg.src = u1; resAudio.src = u2;
          const i = res.info;
          const durTxt = Math.abs(i.duracion - i.duracionAudio) > 0.05 ? `imagen ${fmtT(i.duracion)} · audio ${fmtT(i.duracionAudio)}` : fmtT(i.duracion);
          resInfo.textContent = `${durTxt} · ${i.lado} px${i.fps ? " · " + i.fps + " fps" : ""}${i.isAnimated ? "" : " · fijo"} · calidad ${Math.round(i.calidad * 100)} · sticker ${fmtB(i.webpBytes)}${i.estimado ? " (estimado " + fmtB(i.estimado) + ")" : ""} · audio ${fmtB(i.oggBytes)} a ${i.audioKbps} kbps`;
          result.style.display = "";
          setStatus("Sticker listo. Revisa la vista previa y envíalo.", 1);
        } catch (e) {
          setStatus(e.name === "AbortError" ? "Cancelado" : "Error: " + e.message, 0, e.name !== "AbortError");
        }
        // run() desbloquea el panel antes de que el resultado quede guardado: se refresca otra vez aquí
        // (sin esto, el botón Enviar quedaba gris hasta cambiar de pestaña).
        api.onUpdate();
      });
      cancelBtn.addEventListener("click", () => { if (G.abort) G.abort.abort(); });

      // Reproduce la animación desde el principio junto con el audio, como al tocar el sticker.
      togetherBtn.addEventListener("click", () => {
        if (!c.result) return;
        const u = resImg.src;
        resImg.src = "";
        resImg.src = u;
        resAudio.currentTime = 0;
        resAudio.play().catch((e) => log("warn", "ui:reproducir", e));
      });

      sendBtn.addEventListener("click", async () => {
        if (!c.result) return;
        const dest = destSel.value;
        try {
          await run("enviar", async () => {
            // Aplica el paquete y el autor escritos ahora (pueden haber cambiado después de crear).
            const i = c.result.info;
            if (packInput.value.trim() !== (i.paquete || "") || authorInput.value.trim() !== (i.autor || "")) {
              c.result.webp = await WAAS.retagWebp(c.result.webp, { packName: packInput.value, publisher: authorInput.value, soundSha: i.soundSha256 });
              i.paquete = packInput.value.trim(); i.autor = authorInput.value.trim();
              log("info", "metadatos:actualizados", { paquete: i.paquete, autor: i.autor, webpBytes: c.result.webp.size });
            }
            saveMeta();
            await WAAS.wa.publish(c.result, {
              destino: dest === "propio" ? "propio" : "abierto",
              abrirReenviar: dest !== "abierto-solo",
              onProgress: (t) => setStatus(t),
            });
          });
          setStatus(dest === "abierto-solo" ? "Enviado." : "Enviado. Elige en el diálogo los chats a los que reenviarlo.", 1);
          api.onSent();
        } catch (e) {
          setStatus("Error al enviar: " + e.message, 0, true);
        }
      });
      dlBtn.addEventListener("click", () => {
        if (!c.result) return;
        const base = getBase();
        download(c.result.webp, base + ".webp");
        download(c.result.ogg, base + ".ogg");
        log("info", "ui:descarga-archivos", { base });
      });

      const api = {
        body: [
          h("div", null,
            h("div", { class: "muted" }, "Datos del sticker (opcional): WhatsApp los muestra debajo del sticker. Se recuerdan."),
            h("div", { class: "meta" }, h("span", null, "Paquete"), packInput, h("span", null, "Autor"), authorInput)),
          h("div", { class: "row" }, createBtn, cancelBtn), h("div", { class: "bar" }, barIn), status, result,
        ],
        onSent: () => {},
        onUpdate: () => {},
        setStatus, clear, loadMeta,
        get hasResult() { return !!c.result; },
        refresh() {
          const stale = !!c.result && c.key !== fmt.fullKey();
          const done = !!c.result && !stale;
          createBtn.textContent = done ? "✓ Sticker creado" : c.result ? "Crear sticker con los cambios" : "Crear sticker";
          createBtn.disabled = G.busy || !fmt.ready || done;
          staleNote.style.display = stale ? "" : "none";
          resultBox.style.opacity = stale ? ".45" : "";
          sendBtn.disabled = G.busy || !c.result;
          cancelBtn.style.display = G.busy && api.isActive() ? "" : "none";
        },
        isActive: () => true,
      };
      return api;
    }

    // Arma un formulario completo a partir de sus secciones de origen.
    function makeForm({ id, title, desc, sources, fmt, getBase, onShow, stop }) {
      const create = makeCreate({ fmt, getBase });
      let n = 0;
      const secs = sources.map((s) => section(++n, s.title, ...s.body));
      const secFmt = section(++n, "Formato", ...fmt.body);
      const secCreate = section(++n, "Crear y enviar", ...create.body);
      const el = h("div", { style: "display:none" }, h("div", { class: "muted tabdesc" }, desc), ...secs, secFmt, secCreate);
      const form = {
        id, title, el, create, stop,
        refresh() {
          sources.forEach((s, i) => secs[i].classList.toggle("off", G.busy || (s.enabled ? !s.enabled() : false)));
          secFmt.classList.toggle("off", !sources.every((s) => s.ready()));
          secCreate.classList.toggle("off", !fmt.ready && !create.hasResult);
          fmt.refresh();
          create.refresh();
        },
        show(on) { el.style.display = on ? "" : "none"; if (on) { create.loadMeta(); if (onShow) onShow(); } else stop(); },
      };
      fmt.onUpdate = () => form.refresh();
      create.onUpdate = () => form.refresh();
      create.isActive = () => el.style.display !== "none";
      create.onSent = () => close(true);
      return form;
    }

    // ============================================================ pestaña 1: video con su sonido
    function videoForm() {
      const picker = makePicker("Elegir video…", "video/*,.mov,.mp4,.webm,.mkv", "Cualquier video con sonido.");
      const trimmer = makeTrimmer("video", "Arrastra las asas verdes: el video muestra el cuadro donde quedan. Haz clic o arrastra en la barra para mover el cursor.");
      const s = { file: null };
      const changed = () => form.refresh();
      const fmt = makeFormat({
        getParams: () => (s.file && trimmer.loaded ? {
          visual: { kind: "video", file: s.file, start: trimmer.start, dur: trimmer.len },
          audio: { file: s.file, start: trimmer.start, dur: trimmer.len }, fit: "repetir",
        } : null),
      });
      picker.onPick(async (f) => {
        trimmer.stop();
        fmt.reset();
        form.create.clear();
        picker.info.textContent = "Leyendo " + f.name + "…";
        await run("video", async () => {
          try {
            const meta = await WAAS.probeVideo(f);
            s.file = f;
            trimmer.load(f, meta);
            picker.info.textContent = `${f.name} · ${fmtT(meta.duration)} · ${meta.width}×${meta.height}`;
            log("info", "ui:video-elegido", { nombre: f.name, tipo: f.type, bytes: f.size, ...meta });
          } catch (e) {
            log("error", "ui:video-error", e);
            picker.info.textContent = "No se pudo leer: " + e.message;
            s.file = null;
            trimmer.clear();
          }
        });
        changed();
      });
      trimmer.onChange(changed);
      const form = makeForm({
        id: "video", title: "Video",
        desc: "Un video con su propio sonido: se recorta el tramo y la imagen y el audio van juntos.",
        sources: [
          { title: "Video", body: [picker.el], ready: () => !!s.file },
          { title: "Tramo", body: [trimmer.el], ready: () => trimmer.loaded, enabled: () => trimmer.loaded },
        ],
        fmt, getBase: () => baseName(s.file), stop: () => trimmer.stop(),
      });
      return form;
    }

    // ============================================================ pestaña 2: imagen + audio
    function imageForm() {
      const picker = makePicker("Elegir imagen…", "image/*", "JPG, PNG, WebP… (una foto, un dibujo, un meme).");
      const thumb = h("img", { class: "thumb", alt: "imagen elegida", style: "display:none" });
      const animNote = h("div", { class: "note", style: "display:none" }, "Es una imagen animada: aquí solo se usa su primer cuadro. Para que se mueva, usa la pestaña «Video o GIF + audio».");
      const s = { file: null, url: null };
      const changed = () => form.refresh();
      const audio = makeAudioSource(changed);
      const fmt = makeFormat({
        grid: false, sync: false, still: true,
        getParams: () => (s.file && audio.ready ? { visual: { kind: "image", file: s.file }, audio: audio.params, fit: "repetir" } : null),
      });
      picker.onPick(async (f) => {
        form.create.clear();
        picker.info.textContent = "Leyendo " + f.name + "…";
        await run("imagen", async () => {
          try {
            const meta = await WAAS.probeImage(f);
            s.file = f;
            if (s.url) URL.revokeObjectURL(s.url);
            s.url = URL.createObjectURL(f);
            thumb.src = s.url; thumb.style.display = "";
            animNote.style.display = meta.animated ? "" : "none";
            picker.info.textContent = `${f.name} · ${meta.width}×${meta.height}`;
            log("info", "ui:imagen-elegida", { nombre: f.name, tipo: f.type, bytes: f.size, ...meta });
          } catch (e) {
            log("error", "ui:imagen-error", e);
            picker.info.textContent = "No se pudo leer: " + e.message;
            s.file = null; thumb.style.display = "none"; animNote.style.display = "none";
          }
        });
        changed();
      });
      const form = makeForm({
        id: "imagen", title: "Imagen + audio",
        desc: "Una imagen fija con el sonido que elijas. El sticker dura lo que dure el tramo de audio.",
        sources: [
          { title: "Imagen", body: [picker.el, thumb, animNote], ready: () => !!s.file },
          { title: "Audio", body: audio.els, ready: () => audio.ready },
        ],
        fmt, getBase: () => baseName(s.file), stop: () => audio.trimmer.stop(),
      });
      return form;
    }

    // ============================================================ pestaña 3: video o GIF + otro audio
    function mixForm() {
      const picker = makePicker("Elegir video o GIF…", "video/*,image/gif,image/webp,image/apng,.gif,.mov,.mp4,.webm,.mkv",
        "Un video (su sonido se ignora) o un GIF animado.");
      const vTrim = makeTrimmer("video", "Elige el tramo de la animación. El sonido de este video no se usa.");
      vTrim.setMuted(true);
      const vWrap = h("div", { style: "display:none" }, vTrim.el);
      const gifImg = h("img", { class: "thumb", alt: "GIF elegido", style: "display:none" });
      const s = { file: null, kind: null, gif: null, url: null };
      const changed = () => { updateSummary(); form.refresh(); };
      const audio = makeAudioSource(changed);

      const FIT_KEY = "waas-ajuste-duracion-v2";
      const fitSel = h("select", null,
        h("option", { value: "independiente", textContent: "Que cada uno dure lo suyo" }),
        h("option", { value: "repetir", textContent: "Repetir la animación hasta que termine el audio" }),
        h("option", { value: "congelar", textContent: "Dejar quieto el último cuadro hasta que termine el audio" }));
      fitSel.value = store.get(FIT_KEY, "independiente");
      fitSel.addEventListener("change", () => { store.set(FIT_KEY, fitSel.value); changed(); });
      const summary = h("div", { class: "summary" });
      const fitRow = h("div", null, h("div", { class: "row" }, h("span", null, "Si duran distinto"), fitSel));
      const testBtn = h("button", { class: "sec", textContent: "▶ Probar juntos" });
      const vDur = () => (s.kind === "gif" ? s.gif.duration : vTrim.len);

      function updateSummary() {
        summary.textContent = "";
        if (!s.file || !audio.ready) { summary.append("Elige la animación y el audio para ver cuánto durará el sticker."); fitRow.style.display = "none"; return; }
        const v = vDur(), a = audio.params.dur;
        const same = Math.abs(v - a) < 0.05;
        fitRow.style.display = same ? "none" : "";
        if (same) { summary.append(`Animación y audio duran lo mismo: ${fmtT(a)}.`); return; }
        const fit = fitSel.value;
        let txt = `Animación ${fmtT(v)} · audio ${fmtT(a)}. `;
        if (fit === "independiente") txt += "Cada uno conserva su duración: al tocar el sticker, la animación y el audio empiezan juntos y terminan por separado. Cambiar el audio no obliga a recalcular las opciones.";
        else if (v > a) txt += `La animación es más larga: se corta a ${fmtT(a)}.`;
        else if (fit === "repetir") txt += `El sticker dura ${fmtT(a)}: la animación se repite ${(a / v).toFixed(1).replace(".", ",")} veces.`;
        else txt += `El sticker dura ${fmtT(a)}: la animación se detiene en su último cuadro a los ${fmtT(v)}.`;
        summary.append(txt);
      }

      testBtn.addEventListener("click", () => {
        if (!s.file || !audio.ready) return;
        const a = audio.params;
        if (s.kind === "gif") { const u = gifImg.src; gifImg.src = ""; gifImg.src = u; }
        else vTrim.play(vTrim.start, vTrim.end, true);
        audio.trimmer.play(a.start, a.start + a.dur, false);
      });

      const fmt = makeFormat({
        extras: [summary, fitRow, h("div", { class: "row" }, testBtn, h("span", { class: "muted" }, "Vista previa: la animación se reproduce una vez, como en WhatsApp."))],
        getParams: () => {
          if (!s.file || !audio.ready) return null;
          const visual = s.kind === "gif" ? { kind: "gif", file: s.file, start: 0, dur: s.gif.duration } : { kind: "video", file: s.file, start: vTrim.start, dur: vTrim.len };
          return { visual, audio: audio.params, fit: fitSel.value };
        },
      });

      picker.onPick(async (f) => {
        vTrim.stop();
        fmt.reset();
        form.create.clear();
        picker.info.textContent = "Leyendo " + f.name + "…";
        await run("animacion", async () => {
          try {
            if (s.url) { URL.revokeObjectURL(s.url); s.url = null; }
            if (WAAS.isAnimationFile(f) || (f.type || "").startsWith("image/")) {
              const g = await WAAS.probeAnimation(f);
              if (!g.animated) throw new Error("es una imagen fija. Usa la pestaña «Imagen + audio».");
              s.file = f; s.kind = "gif"; s.gif = g;
              s.url = URL.createObjectURL(f);
              gifImg.src = s.url; gifImg.style.display = ""; vWrap.style.display = "none";
              vTrim.clear();
              picker.info.textContent = `${f.name} · ${fmtT(g.duration)} · ${g.frames} cuadros · ${g.width}×${g.height} · se usa completo`;
              log("info", "ui:gif-elegido", { nombre: f.name, tipo: f.type, bytes: f.size, cuadros: g.frames, duracion: g.duration, ancho: g.width, alto: g.height });
            } else {
              const meta = await WAAS.probeVideo(f);
              s.file = f; s.kind = "video"; s.gif = null;
              vTrim.load(f, meta);
              vWrap.style.display = ""; gifImg.style.display = "none";
              picker.info.textContent = `${f.name} · ${fmtT(meta.duration)} · ${meta.width}×${meta.height}`;
              log("info", "ui:video-elegido", { nombre: f.name, tipo: f.type, bytes: f.size, sinSuAudio: true, ...meta });
            }
          } catch (e) {
            log("error", "ui:animacion-error", e);
            picker.info.textContent = "No se pudo usar: " + e.message;
            s.file = null; s.kind = null; s.gif = null;
            vTrim.clear(); vWrap.style.display = "none"; gifImg.style.display = "none";
          }
        });
        changed();
      });
      vTrim.onChange(changed);
      updateSummary();

      const form = makeForm({
        id: "mezcla", title: "Video o GIF + audio",
        desc: "Una animación (video sin su sonido, o un GIF) con el audio que elijas.",
        sources: [
          { title: "Animación", body: [picker.el, vWrap, gifImg], ready: () => !!s.file },
          { title: "Audio", body: audio.els, ready: () => audio.ready },
        ],
        fmt, getBase: () => baseName(s.file),
        stop: () => { vTrim.stop(); audio.trimmer.stop(); },
      });
      return form;
    }

    // ============================================================ panel
    G.forms = [videoForm(), imageForm(), mixForm()];
    const TAB_KEY = "waas-pestana";
    let active = Math.max(0, G.forms.findIndex((f) => f.id === store.get(TAB_KEY, "video")));
    const tabBtns = G.forms.map((f, i) => h("button", { textContent: f.title, onclick: () => selectTab(i) }));
    function selectTab(i) {
      if (G.busy) return;
      active = i;
      store.set(TAB_KEY, G.forms[i].id);
      G.forms.forEach((f, j) => f.show(j === i));
      G.refreshTabs();
      G.forms[i].refresh();
      log("info", "ui:pestana", { pestana: G.forms[i].id });
    }
    G.refreshTabs = () => tabBtns.forEach((b, i) => { b.classList.toggle("on", i === active); b.disabled = G.busy && i !== active; });

    const logBox = h("div", { class: "logbox" });
    const tools = h("details", null, h("summary", null, "Herramientas de prueba (registro, diagnóstico)"),
      h("div", { class: "row" },
        h("button", { class: "sec", textContent: "Diagnóstico", onclick: () => WAAS.wa.diagnose().catch((e) => log("error", "diagnostico:error", e)) }),
        h("button", { class: "sec", textContent: "Guardar registro…", onclick: () => saveLog().catch((e) => log("error", "registro:error", e)) }),
        h("button", { class: "sec", textContent: "Ver registro completo", onclick: () => window.postMessage({ __waas: "openLog" }, location.origin) })),
      logBox);

    const closeBtn = h("button", { class: "close", title: "Cerrar", textContent: "×" });
    const panel = h("div", { class: "panel" },
      h("header", null, h("div", null, h("h2", null, "Stickers con sonido"), h("div", { class: "muted" }, "Versión " + WAAS.VERSION + " · todo se procesa en tu navegador · máximo 1 MB por sticker")),
        h("div", { class: "hdr-r" },
          h("a", { class: "kofi", href: "https://ko-fi.com/shirtep", target: "_blank", rel: "noopener", title: "La extensión es gratis. Si te sirve, puedes apoyar el proyecto en Ko-fi.", textContent: "☕ Apoyar" }),
          closeBtn)),
      h("nav", { class: "tabs" }, tabBtns),
      G.forms.map((f) => f.el), tools);
    const overlay = h("div", { class: "overlay", style: "display:none" }, panel);
    root.append(overlay);

    const showEntry = (e) => {
      logBox.append(h("div", { class: e.level === "error" ? "e" : e.level === "warn" ? "w" : "" },
        e.ts.slice(11, 19) + " " + e.event + (e.data != null ? " " + JSON.stringify(e.data).slice(0, 260) : "")));
      while (logBox.childNodes.length > 200) logBox.firstChild.remove();
      logBox.scrollTop = logBox.scrollHeight;
    };
    WAAS.entries.slice(-30).forEach(showEntry);
    WAAS.onLog(showEntry);

    const open = () => {
      panel.classList.toggle("dark", isDark());
      overlay.style.display = "";
      G.forms[active].show(true);
      log("info", "ui:abrir", null);
    };
    function close(force) {
      if (G.busy && force !== true) return;
      G.forms.forEach((f) => f.stop());
      overlay.style.display = "none";
    }
    const toggle = () => (overlay.style.display === "none" ? open() : close());
    window.addEventListener("message", (ev) => {
      if (ev.source === window && ev.data && ev.data.__waas === "togglePanel") toggle();
    });
    WAAS.togglePanel = toggle;
    closeBtn.addEventListener("click", close);
    overlay.addEventListener("click", (e) => { if (e.target === overlay) close(); });

    selectTab(active);
    log("info", "ui:montada", null);
    window.postMessage({ __waas: "uiReady" }, location.origin);
  }

  WAAS.mountUI = mount;
})();
