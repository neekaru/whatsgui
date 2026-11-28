// Antarmuka: panel modern berbasis Shadow DOM untuk membuat stiker audio WhatsApp.
// Tanpa innerHTML (WhatsApp Web memakai Trusted Types): seluruh DOM dibuat dengan createElement.
(() => {
  const WAAS = (window.WAAS = window.WAAS || {});
  const log = (lvl, ev, data) => WAAS.log(lvl, ev, data);
  const FRAME = 1 / 30;
  const MIN_LEN = 0.3;
  const DEFAULT_LEN = 10;

  const CSS = `
  :host { all: initial; }
  * { box-sizing: border-box; font-family: "Segoe UI Variable Text", "Segoe UI", system-ui, -apple-system, sans-serif; }

  .overlay {
    position: fixed;
    inset: 0;
    z-index: 2147483001;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 20px;
    background: rgba(0, 0, 0, 0.58);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    animation: waasFade 0.16s ease-out;
  }
  @keyframes waasFade { from { opacity: 0; } to { opacity: 1; } }
  @keyframes waasScale { from { transform: scale(0.97); opacity: 0; } to { transform: scale(1); opacity: 1; } }

  .panel {
    --bg: #ffffff;
    --panel-2: #f7f9fa;
    --panel-3: #f0f2f5;
    --hover: #e9ecef;
    --fg: #111b21;
    --muted: #54656f;
    --subtle: #8696a0;
    --line: #e2e5e8;
    --line-strong: #cfd4dc;
    --accent: #00a884;
    --accent-hover: #029071;
    --accent-light: #e7f7f4;
    --accent-fg: #ffffff;
    --ok: #00a884;
    --ok-bg: rgba(0, 168, 132, 0.14);
    --warn: #c48400;
    --warn-bg: rgba(240, 180, 0, 0.18);
    --no: #8696a0;
    --err: #ea4335;
    --err-bg: #fde8e7;
    --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.06);
    --shadow-lg: 0 18px 42px rgba(0, 0, 0, 0.24), 0 0 0 1px rgba(0, 0, 0, 0.06);

    display: flex;
    flex-direction: column;
    width: min(700px, 96vw);
    max-height: calc(100vh - 40px);
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--line-strong);
    border-radius: 14px;
    box-shadow: var(--shadow-lg);
    font-size: 13px;
    line-height: 1.45;
    overflow: hidden;
    animation: waasScale 0.18s cubic-bezier(0.16, 1, 0.3, 1);
  }
  .panel.dark {
    --bg: #111b21;
    --panel-2: #182229;
    --panel-3: #202c33;
    --hover: #202c33;
    --fg: #e9edef;
    --muted: #8696a0;
    --subtle: #667781;
    --line: #222d34;
    --line-strong: #2a3942;
    --accent: #00a884;
    --accent-hover: #06cf9c;
    --accent-light: rgba(0, 168, 132, 0.16);
    --ok-bg: rgba(0, 168, 132, 0.22);
    --warn: #f0b400;
    --warn-bg: rgba(240, 180, 0, 0.2);
    --err: #f15c5c;
    --err-bg: rgba(241, 92, 92, 0.15);
    --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.35);
    --shadow-lg: 0 18px 44px rgba(0, 0, 0, 0.65), 0 0 0 1px rgba(255, 255, 255, 0.08);
  }

  .hdr {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 15px 20px;
    border-bottom: 1px solid var(--line);
    background: var(--bg);
    flex: none;
  }
  .hdr-info { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .hdr-badge {
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    border-radius: 10px;
    background: var(--accent-light);
    color: var(--accent);
    flex-shrink: 0;
  }
  h2 { margin: 0; font-size: 15px; font-weight: 600; color: var(--fg); }
  .subtitle { margin: 2px 0 0; font-size: 12px; color: var(--muted); }
  .hdr-r { display: flex; align-items: center; gap: 8px; flex: none; }
  .credit-link {
    padding: 4px 10px;
    border-radius: 9999px;
    border: 1px solid var(--line);
    background: var(--panel-2);
    color: var(--muted);
    font-size: 11.5px;
    font-weight: 500;
    text-decoration: none;
    transition: all 0.15s ease;
  }
  .credit-link:hover { border-color: var(--line-strong); background: var(--hover); color: var(--fg); }
  .close {
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--muted);
    font-size: 20px;
    line-height: 1;
    cursor: pointer;
    transition: all 0.12s;
  }
  .close:hover { background: var(--hover); color: var(--fg); }

  .tabs {
    display: flex;
    gap: 4px;
    padding: 8px 16px;
    border-bottom: 1px solid var(--line);
    background: var(--panel-2);
    overflow-x: auto;
    flex: none;
  }
  .tabs button {
    display: inline-flex;
    align-items: center;
    height: 32px;
    padding: 0 14px;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-size: 12.5px;
    font-weight: 500;
    white-space: nowrap;
    cursor: pointer;
    transition: all 0.12s;
  }
  .tabs button:hover:not(:disabled) { background: var(--hover); color: var(--fg); }
  .tabs button.on {
    background: var(--bg);
    color: var(--accent);
    font-weight: 600;
    box-shadow: var(--shadow-sm);
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 16px 20px 20px;
  }
  .tabdesc {
    margin: 0 0 12px;
    padding: 10px 12px;
    border-radius: 8px;
    border: 1px solid var(--line);
    background: var(--panel-2);
    color: var(--muted);
    font-size: 12px;
  }

  section {
    background: var(--panel-2);
    border: 1px solid var(--line);
    border-radius: 10px;
    padding: 14px 16px;
    margin: 0 0 12px;
    transition: opacity 0.15s ease;
  }
  section.off { opacity: 0.45; pointer-events: none; }
  h3 {
    margin: 0 0 10px;
    font-size: 13.5px;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--fg);
  }
  h3 .n {
    display: inline-flex;
    width: 22px;
    height: 22px;
    border-radius: 9999px;
    background: var(--accent);
    color: var(--accent-fg);
    align-items: center;
    justify-content: center;
    font-size: 11.5px;
    font-weight: 700;
    flex-shrink: 0;
  }

  .muted { color: var(--muted); font-size: 12px; }
  .note {
    margin-top: 8px;
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--warn-bg);
    color: var(--warn);
    font-size: 12px;
    font-weight: 500;
  }
  .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin: 8px 0; }
  .grow { flex: 1; }

  button { font: inherit; cursor: pointer; }
  button:disabled { opacity: 0.48; cursor: default; }

  .act {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 32px;
    padding: 0 14px;
    border: 1px solid var(--accent);
    border-radius: 6px;
    background: var(--accent);
    color: var(--accent-fg);
    font-size: 12.5px;
    font-weight: 600;
    box-shadow: 0 2px 6px rgba(0, 168, 132, 0.2);
    transition: all 0.15s ease;
  }
  .act:hover:not(:disabled) { border-color: var(--accent-hover); background: var(--accent-hover); }

  .sec {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 28px;
    padding: 0 11px;
    border: 1px solid var(--line);
    border-radius: 6px;
    background: var(--bg);
    color: var(--fg);
    font-size: 12px;
    font-weight: 500;
    transition: all 0.15s ease;
  }
  .sec:hover:not(:disabled) { border-color: var(--line-strong); background: var(--hover); }
  .icon { width: 28px; padding: 0; }

  input[type="text"], input[type="number"], select {
    height: 32px;
    padding: 0 10px;
    border-radius: 6px;
    border: 1px solid var(--line);
    background: var(--bg);
    color: var(--fg);
    font: inherit;
    font-size: 12.5px;
  }
  input[type="text"] { width: 220px; }
  input[type="number"] { width: 80px; font-variant-numeric: tabular-nums; }
  select { max-width: 100%; cursor: pointer; }
  input:focus, select:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-light);
  }

  .meta { display: grid; grid-template-columns: auto 1fr; gap: 8px 12px; align-items: center; margin: 6px 0 12px; }
  label.chk { display: flex; gap: 8px; align-items: flex-start; margin: 8px 0; cursor: pointer; }

  video {
    display: block;
    width: 100%;
    max-height: 250px;
    background: #000;
    border-radius: 8px;
    border: 1px solid var(--line);
  }
  .thumb {
    width: 136px;
    height: 136px;
    object-fit: contain;
    border-radius: 10px;
    border: 1px solid var(--line);
    background: repeating-conic-gradient(#cbd5e1 0 25%, #f8fafc 0 50%) 50% / 16px 16px;
  }
  .panel.dark .thumb {
    background: repeating-conic-gradient(#26333b 0 25%, #19242b 0 50%) 50% / 16px 16px;
  }

  .track {
    position: relative;
    height: 34px;
    margin: 12px 6px 6px;
    background: var(--line);
    border-radius: 6px;
    cursor: pointer;
    touch-action: none;
    user-select: none;
  }
  .track.audio { height: 50px; }
  .track canvas { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; border-radius: 6px; }
  .track .sel {
    position: absolute;
    top: 0;
    bottom: 0;
    background: rgba(0, 168, 132, 0.28);
    border-top: 2px solid var(--accent);
    border-bottom: 2px solid var(--accent);
    pointer-events: none;
  }
  .track .handle {
    position: absolute;
    top: -4px;
    bottom: -4px;
    width: 12px;
    margin-left: -6px;
    background: var(--accent);
    border-radius: 4px;
    cursor: ew-resize;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.25);
  }
  .track .handle::after {
    content: "";
    position: absolute;
    left: 4px;
    top: 8px;
    bottom: 8px;
    width: 4px;
    border-left: 1px solid rgba(255, 255, 255, 0.85);
    border-right: 1px solid rgba(255, 255, 255, 0.85);
  }
  .track .ph { position: absolute; top: -6px; bottom: -6px; width: 2px; margin-left: -1px; background: var(--fg); pointer-events: none; }
  .times { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); margin: 0 6px; font-variant-numeric: tabular-nums; }
  .trim { display: grid; grid-template-columns: auto auto 1fr; gap: 6px 10px; align-items: center; margin-top: 6px; }

  .seg {
    display: inline-flex;
    padding: 2px;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: var(--bg);
    gap: 2px;
  }
  .seg button {
    border: none;
    height: 28px;
    padding: 0 12px;
    border-radius: 6px;
    background: transparent;
    color: var(--muted);
    font-size: 12px;
    font-weight: 500;
  }
  .seg button.on { background: var(--accent); color: var(--accent-fg); font-weight: 600; }

  .summary {
    background: var(--bg);
    border: 1px solid var(--line);
    border-radius: 8px;
    padding: 9px 12px;
    margin: 8px 0;
    font-size: 12px;
    color: var(--fg);
  }

  table.grid { border-collapse: separate; border-spacing: 3px; margin: 8px 0; font-size: 11.5px; }
  table.grid th { color: var(--muted); font-weight: 600; padding: 2px 4px; text-align: center; }
  table.grid th.side { text-align: right; padding-right: 6px; }
  table.grid td button {
    width: 52px;
    height: 26px;
    padding: 0;
    border-radius: 6px;
    border: 1px solid transparent;
    font-size: 11px;
    font-weight: 500;
    background: transparent;
    color: var(--no);
    font-variant-numeric: tabular-nums;
  }
  table.grid td button.ok { background: var(--ok-bg); color: var(--fg); }
  table.grid td button.baja { background: var(--warn-bg); color: var(--fg); }
  table.grid td button.no, table.grid td button.fuera { cursor: default; opacity: 0.55; }
  table.grid td button.sel { border-color: var(--accent); box-shadow: inset 0 0 0 1px var(--accent); font-weight: 700; }

  .legend { display: flex; gap: 14px; flex-wrap: wrap; font-size: 11.5px; color: var(--muted); margin-top: 4px; }
  .legend i { display: inline-block; width: 11px; height: 11px; border-radius: 3px; vertical-align: -2px; margin-right: 5px; }
  .stale {
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--warn-bg);
    color: var(--warn);
    font-size: 12px;
    font-weight: 600;
    margin-top: 6px;
  }

  .bar { height: 6px; background: var(--line); border-radius: 9999px; overflow: hidden; margin-top: 10px; }
  .bar > div { height: 100%; width: 0; background: var(--accent); transition: width 0.2s ease; }
  .status { font-size: 12.5px; margin: 8px 0 0; min-height: 18px; color: var(--muted); }
  .status.err { color: var(--err); font-weight: 600; }

  .result {
    display: flex;
    gap: 14px;
    align-items: center;
    flex-wrap: wrap;
    margin-top: 10px;
    padding: 12px;
    border-radius: 8px;
    border: 1px solid var(--line);
    background: var(--bg);
  }

  details { margin-top: 8px; }
  summary { cursor: pointer; color: var(--muted); font-size: 12px; font-weight: 500; user-select: none; }
  .logbox {
    max-height: 150px;
    overflow: auto;
    font: 11px/1.4 Consolas, "Cascadia Mono", monospace;
    background: var(--bg);
    border: 1px solid var(--line);
    padding: 8px 10px;
    border-radius: 6px;
    white-space: pre-wrap;
    margin-top: 8px;
  }
  .logbox .e { color: var(--err); }
  .logbox .w { color: var(--warn); }
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
    const content = JSON.stringify({ diekspor: new Date().toISOString(), versi: WAAS.VERSION, ua: navigator.userAgent, entri: entries }, null, 1);
    const blob = new Blob([content], { type: "application/json" });
    const name = "log-stiker-audio-" + new Date().toISOString().replace(/[:.]/g, "-").slice(0, 19) + ".json";
    if (window.showSaveFilePicker) {
      try {
        const handle = await window.showSaveFilePicker({ suggestedName: name, types: [{ description: "Log JSON", accept: { "application/json": [".json"] } }] });
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

  const fmtT = (t) => (t >= 60 ? Math.floor(t / 60) + ":" + (t % 60).toFixed(2).padStart(5, "0") : t.toFixed(2) + " dtk");
  const fmtB = (b) => (b >= 1e6 ? (b / 1e6).toFixed(2) + " MB" : Math.round(b / 1000) + " KB");
  const isDark = () => {
    try {
      if (document.body.classList.contains("dark") || document.documentElement.classList.contains("dark")) return true;
      const saved = localStorage.getItem("theme");
      if (saved === '"light"' || saved === "light") return false;
      if (saved === '"dark"' || saved === "dark") return true;
      return matchMedia("(prefers-color-scheme: dark)").matches;
    } catch (e) { return false; }
  };
  const store = {
    get(k, def) { try { const v = localStorage.getItem(k); return v == null ? def : JSON.parse(v); } catch (e) { return def; } },
    set(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); } catch (e) { /* abaikan */ } },
  };
  const baseName = (f) => (f && f.name.replace(/\.[^.]+$/, "")) || "stiker";

  function mount() {
    if (document.getElementById("waas-root")) return;
    const host = h("div", { id: "waas-root" });
    document.body.append(host);
    const root = host.attachShadow({ mode: "open" });
    attachStyles(root);

    const G = { busy: false, forms: [], refreshTabs: () => {} };
    const setBusy = (b) => { G.busy = b; G.forms.forEach((f) => f.refresh()); G.refreshTabs(); };
    const run = async (name, fn) => {
      const ac = new AbortController();
      G.abort = ac;
      setBusy(true);
      try { return await WAAS.run(name, () => fn(ac.signal)); } finally { G.abort = null; setBusy(false); }
    };

    // ============================================================ pemotong video / audio
    function makeTrimmer(kind, hint) {
      const isVideo = kind === "video";
      const media = isVideo ? h("video", { playsInline: true, preload: "auto" }) : h("audio", { preload: "auto" });
      const wave = isVideo ? null : h("canvas");
      const selBar = h("div", { class: "sel" });
      const hS = h("div", { class: "handle", title: "Awal segmen (seret)" });
      const hE = h("div", { class: "handle", title: "Akhir segmen (seret)" });
      const ph = h("div", { class: "ph" });
      const track = h("div", { class: "track" + (isVideo ? "" : " audio"), title: "Klik atau seret untuk menggeser kursor" }, wave, selBar, ph, hS, hE);
      const tLeft = h("span"), tRight = h("span");
      const playSegBtn = h("button", { class: "sec", type: "button", textContent: "▶ Putar Segmen" });
      const playCurBtn = h("button", { class: "sec", type: "button", textContent: "▶ Dari Kursor" });
      const playEndBtn = h("button", { class: "sec", type: "button", textContent: "▶ Akhir Segmen" });
      const curLbl = h("span", { class: "muted grow", style: "text-align:right;font-variant-numeric:tabular-nums" });
      const startNum = h("input", { type: "number", min: 0, step: 0.01 });
      const endNum = h("input", { type: "number", min: 0, step: 0.01 });
      const startCur = h("button", { class: "sec", type: "button", textContent: "⤓ Kursor", title: "Gunakan posisi kursor sebagai awal" });
      const endCur = h("button", { class: "sec", type: "button", textContent: "⤓ Kursor", title: "Gunakan posisi kursor sebagai akhir" });
      const lenLbl = h("span", { class: "muted" });
      const t = { start: 0, end: 0, playhead: 0, stopAt: null, dur: 0, offset: 0, url: null, peaks: null, peakMax: 1, muted: false };
      const listeners = new Set();
      const emit = () => listeners.forEach((fn) => fn());
      const mkStep = (txt, title, fn) => h("button", { class: "sec icon", type: "button", textContent: txt, title, onclick: fn });
      const el = h("div", null,
        isVideo ? media : null, track, h("div", { class: "times" }, tLeft, tRight),
        h("div", { class: "row" }, playSegBtn, playCurBtn, playEndBtn, curLbl),
        h("div", { class: "trim" },
          h("span", null, "Mulai"),
          h("span", { class: "row", style: "margin:0" }, mkStep("−", "Mundur 1 frame", () => setStart(t.start - FRAME)), startNum, mkStep("+", "Maju 1 frame", () => setStart(t.start + FRAME))),
          h("span", { class: "row", style: "margin:0" }, startCur),
          h("span", null, "Selesai"),
          h("span", { class: "row", style: "margin:0" }, mkStep("−", "Mundur 1 frame", () => setEnd(t.end - FRAME)), endNum, mkStep("+", "Maju 1 frame", () => setEnd(t.end + FRAME))),
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
        tLeft.textContent = "0 dtk";
        tRight.textContent = fmtT(t.dur);
        curLbl.textContent = "Kursor " + fmtT(t.playhead);
        if (root.activeElement !== startNum) startNum.value = t.start.toFixed(2);
        if (root.activeElement !== endNum) endNum.value = t.end.toFixed(2);
        lenLbl.textContent = "Durasi " + fmtT(t.end - t.start);
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
        ctx.fillStyle = getComputedStyle(track).getPropertyValue("--subtle").trim() || "#8696a0";
        const n = t.peaks.length, cols = wave.width, mid = wave.height / 2, norm = 1 / (t.peakMax || 1);
        for (let x = 0; x < cols; x++) {
          const a = Math.floor((x / cols) * n), b = Math.max(a + 1, Math.floor(((x + 1) / cols) * n));
          let p = 0;
          for (let i = a; i < b && i < n; i++) if (t.peaks[i] > p) p = t.peaks[i];
          const hgt = Math.max(1, p * norm * (wave.height - 6));
          ctx.fillRect(x, mid - hgt / 2, 1, hgt);
        }
      };
      if (wave && typeof ResizeObserver !== "undefined") new ResizeObserver(drawWave).observe(track);

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

    // ============================================================ bagian umum
    const section = (n, title, ...body) => h("section", { class: "off" }, h("h3", null, h("span", { class: "n" }, String(n)), title), ...body);

    function makePicker(label, accept, hint) {
      const input = h("input", { type: "file", accept, style: "display:none" });
      const btn = h("button", { class: "act", type: "button", textContent: label });
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

    function makeAudioSource(onChange) {
      const picker = makePicker("Pilih Audio…", "audio/*,video/*,.mp3,.m4a,.aac,.ogg,.opus,.wav,.flac,.mp4,.mov,.webm",
        "MP3, M4A, OGG, WAV, atau ambil audio dari video.");
      const trimmer = makeTrimmer("audio", "Pilih rentang awal dan akhir suara. Seret gagang hijau atau isi angka detik.");
      const wrap = h("div", { style: "display:none" }, trimmer.el);
      const s = { file: null };
      picker.onPick(async (f) => {
        picker.info.textContent = "Membaca " + f.name + "…";
        await run("audio", async () => {
          try {
            const a = await WAAS.probeAudio(f);
            s.file = f;
            trimmer.load(f, { duration: a.duration, peaks: a.peaks, peakMax: a.peakMax });
            wrap.style.display = "";
            picker.info.textContent = `${f.name} · ${fmtT(a.duration)}${a.peakMax < 0.001 ? " · tampak tidak bersuara" : ""}`;
            log("info", "ui:audio-elegido", { nombre: f.name, tipo: f.type, bytes: f.size, duracion: a.duration, canales: a.channels, pico: +a.peakMax.toFixed(4) });
          } catch (e) {
            log("error", "ui:audio-error", e);
            picker.info.textContent = "Gagal membaca audio: " + e.message;
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

    function makeFormat({ getParams, grid = true, sync = true, still = false, extras = [] }) {
      const f = { mode: "ajustar", an: null, anKey: null, sel: null };
      const segAjustar = h("button", { type: "button", textContent: "Sesuaikan (tepi transparan)", class: "on" });
      const segRecortar = h("button", { type: "button", textContent: "Potong (penuhi kotak)" });
      const calcBtn = h("button", { class: "act", type: "button", textContent: "Hitung Opsi" });
      const calcInfo = h("span", { class: "muted" });
      const staleLbl = h("div", { class: "stale", style: "display:none" }, "Ada perubahan input: hitung ulang opsi terlebih dahulu.");
      const calcBarIn = h("div");
      const calcBar = h("div", { class: "bar", style: "display:none" }, calcBarIn);
      const gridHost = h("div");
      const selLbl = h("div", { class: "row", style: "font-weight:600" });
      const legend = h("div", { class: "legend" },
        h("span", null, h("i", { style: "background:var(--ok-bg);border:1px solid var(--ok)" }), "Muat"),
        h("span", null, h("i", { style: "background:var(--warn-bg);border:1px solid var(--warn)" }), "Muat (kualitas diturunkan)"),
        h("span", null, h("i", { style: "border:1px solid var(--line)" }), "✕ Melebihi 1 MB"),
        h("span", null, "★ Rekomendasi · angka dalam KB"));
      const syncNum = h("input", { type: "number", min: 0, max: 1000, step: 10, value: store.get("waas-compensacion-ms", 0) });
      syncNum.addEventListener("change", () => { store.set("waas-compensacion-ms", +syncNum.value || 0); api.onUpdate(); });
      const stillChk = h("input", { type: "checkbox", onchange: () => api.onUpdate() });
      const advRows = [];
      if (sync) advRows.push(h("div", { class: "row" }, h("span", null, "Kompensasi jeda (ms)"), syncNum,
        h("span", { class: "muted" }, "Biasanya 0. Jika di HP suara terasa telat, gambar akan menunggu jeda ini.")));
      if (still) advRows.push(h("label", { class: "chk" }, stillChk, h("span", null, "Kirim sebagai gambar statis (eksperimental)",
        h("div", { class: "muted" }, "Secara default gambar dikirim sebagai stiker animasi 1 frame sesuai durasi audio. Centang untuk mencoba format stiker statis."))));
      const advanced = advRows.length ? h("details", null, h("summary", null, "Pengaturan Lanjutan"), ...advRows) : null;

      const fileId = (x) => (x ? x.name + "|" + x.size + "|" + x.lastModified : "");
      const key = () => {
        const p = getParams();
        if (!p) return null;
        const v = p.visual;
        const followsAudio = v.kind === "image" || p.fit !== "independiente";
        return JSON.stringify([v.kind, fileId(v.file), v.start, v.dur, followsAudio ? p.fit : "", followsAudio ? +p.audio.dur.toFixed(3) : "", f.mode]);
      };
      const fullKey = () => {
        const p = getParams();
        if (!p) return null;
        return JSON.stringify([key(), fileId(p.audio.file), +p.audio.start.toFixed(3), +p.audio.dur.toFixed(3), p.fit, f.sel, api.options()]);
      };
      const fresh = () => !!f.an && f.anKey === key();
      const CALC_HINT = "Analisis segmen sekali untuk memperkirakan ukuran tiap resolusi dan FPS.";
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
          const title = !c || c.estado === "fuera" ? "Tidak tersedia untuk segmen ini"
            : c.estado === "no" ? `≈ ${fmtB(c.estMin)} bahkan di kualitas minimum: melebihi 1 MB`
            : c.estado === "baja" ? `≈ ${fmtB(c.est)} di kualitas normal: kualitas akan diturunkan agar muat`
            : `≈ ${fmtB(c.est)}`;
          return h("td", null, h("button", {
            type: "button",
            class: (c ? c.estado : "fuera") + (isSel ? " sel" : ""), textContent: label, title, disabled: !can,
            onclick: () => { f.sel = { side, fps }; renderGrid(); api.onUpdate(); },
          }));
        })));
        gridHost.append(h("table", { class: "grid" }, thead, rows));
        const c = f.sel && an.cells.find((x) => x.side === f.sel.side && x.fps === f.sel.fps);
        selLbl.textContent = c ? `Terpilih: ${c.side} px · ${c.fps} fps · ≈ ${fmtB(c.est)}${c.estado === "baja" ? " (kualitas disesuaikan)" : ""}` : "Tidak ada kombinasi yang muat di bawah 1 MB: perpendek durasi segmen.";
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
          calcInfo.textContent = `Selesai dalam ${(an.ms / 1000).toFixed(1)} dtk. Pilih sel (★ = rekomendasi).`;
          renderGrid();
        } catch (e) {
          calcInfo.textContent = e.name === "AbortError" ? "Dibatalkan" : "Error: " + e.message;
        }
        api.setProgress(0);
        api.onUpdate();
      });

      const body = [...extras, h("div", { class: "row" }, h("span", null, "Pembingkaian"), h("span", { class: "seg" }, segAjustar, segRecortar))];
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
          gridHost.style.opacity = stale ? "0.35" : "";
          gridHost.style.pointerEvents = stale ? "none" : "";
          calcBtn.disabled = G.busy || !has;
          segAjustar.disabled = segRecortar.disabled = G.busy;
          segAjustar.classList.toggle("on", f.mode === "ajustar");
          segRecortar.classList.toggle("on", f.mode === "recortar");
        },
        get ready() { return grid ? fresh() && !!f.sel : !!getParams(); },
        reset,
        fullKey,
        options: () => ({ syncMs: sync ? +syncNum.value || 0 : 0, still: still && stillChk.checked }),
        async prepare(signal, onProgress) {
          const p = getParams();
          if (grid) return { an: f.an, side: f.sel.side, fps: f.sel.fps, audio: p.audio, temp: false };
          const an = await WAAS.analyze({ ...p, mode: f.mode, signal, onProgress });
          return { an, side: an.auto.side, fps: an.auto.fps, audio: p.audio, temp: true };
        },
      };
      return api;
    }

    function makeCreate({ fmt, getBase }) {
      const META_KEY = "waas-metadatos";
      const packInput = h("input", { type: "text", maxLength: 128, placeholder: "Misal: Stiker Lucu" });
      const authorInput = h("input", { type: "text", maxLength: 128, placeholder: "Misal: nama Anda" });
      const loadMeta = () => { const m = store.get(META_KEY, {}) || {}; packInput.value = m.paquete || ""; authorInput.value = m.autor || ""; };
      const saveMeta = () => store.set(META_KEY, { paquete: packInput.value, autor: authorInput.value });
      loadMeta();
      packInput.addEventListener("change", saveMeta);
      authorInput.addEventListener("change", saveMeta);
      const createBtn = h("button", { class: "act", type: "button", textContent: "Buat Stiker" });
      const cancelBtn = h("button", { class: "sec", type: "button", textContent: "Batal", style: "display:none" });
      const barIn = h("div");
      const status = h("div", { class: "status" });
      const resImg = h("img", { class: "thumb", alt: "pratinjau stiker" });
      const resAudio = h("audio", { controls: true });
      const resInfo = h("div", { class: "muted", style: "max-width:380px" });
      const togetherBtn = h("button", { class: "sec", type: "button", textContent: "▶ Putar Bersamaan" });
      const destSel = h("select", null,
        h("option", { value: "abierto-solo", textContent: "Kirim ke chat yang terbuka" }),
        h("option", { value: "abierto", textContent: "Kirim ke chat terbuka & buka Teruskan" }),
        h("option", { value: "propio", textContent: "Simpan ke chat sendiri (Anda) & buka Teruskan" }));
      const sendBtn = h("button", { class: "act", type: "button", textContent: "Kirim Stiker" });
      const dlBtn = h("button", { class: "sec", type: "button", textContent: "Unduh .webp & .ogg" });
      const staleNote = h("div", { class: "stale", style: "display:none;margin-top:8px" },
        "Ada perubahan setelah stiker dibuat. Klik «Buat Ulang dengan Perubahan» agar perubahan ikut terkirim.");
      const resultBox = h("div", { class: "result" }, resImg, h("div", { style: "flex:1;min-width:220px" }, resAudio, h("div", { class: "row" }, togetherBtn), resInfo));
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
          const durTxt = Math.abs(i.duracion - i.duracionAudio) > 0.05 ? `gambar ${fmtT(i.duracion)} · audio ${fmtT(i.duracionAudio)}` : fmtT(i.duracion);
          resInfo.textContent = `${durTxt} · ${i.lado} px${i.fps ? " · " + i.fps + " fps" : ""}${i.isAnimated ? "" : " · statis"} · kualitas ${Math.round(i.calidad * 100)} · stiker ${fmtB(i.webpBytes)}${i.estimado ? " (perkiraan " + fmtB(i.estimado) + ")" : ""} · audio ${fmtB(i.oggBytes)} (${i.audioKbps} kbps)`;
          result.style.display = "";
          setStatus("Stiker siap. Periksa pratinjau lalu kirim.", 1);
        } catch (e) {
          setStatus(e.name === "AbortError" ? "Dibatalkan" : "Error: " + e.message, 0, e.name !== "AbortError");
        }
        api.onUpdate();
      });
      cancelBtn.addEventListener("click", () => { if (G.abort) G.abort.abort(); });

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
          setStatus(dest === "abierto-solo" ? "Terkirim." : "Terkirim. Pilih chat tujuan pada dialog Teruskan.", 1);
          api.onSent();
        } catch (e) {
          setStatus("Gagal mengirim: " + e.message, 0, true);
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
            h("div", { class: "muted" }, "Info paket stiker (opsional, tampil di bawah stiker pada WhatsApp)."),
            h("div", { class: "meta" }, h("span", null, "Nama Paket"), packInput, h("span", null, "Pembuat"), authorInput)),
          h("div", { class: "row" }, createBtn, cancelBtn), h("div", { class: "bar" }, barIn), status, result,
        ],
        onSent: () => {},
        onUpdate: () => {},
        setStatus, clear, loadMeta,
        get hasResult() { return !!c.result; },
        refresh() {
          const stale = !!c.result && c.key !== fmt.fullKey();
          const done = !!c.result && !stale;
          createBtn.textContent = done ? "✓ Stiker Dibuat" : c.result ? "Buat Ulang dengan Perubahan" : "Buat Stiker";
          createBtn.disabled = G.busy || !fmt.ready || done;
          staleNote.style.display = stale ? "" : "none";
          resultBox.style.opacity = stale ? "0.45" : "";
          sendBtn.disabled = G.busy || !c.result;
          cancelBtn.style.display = G.busy && api.isActive() ? "" : "none";
        },
        isActive: () => true,
      };
      return api;
    }

    function makeForm({ id, title, desc, sources, fmt, getBase, onShow, stop }) {
      const create = makeCreate({ fmt, getBase });
      let n = 0;
      const secs = sources.map((s) => section(++n, s.title, ...s.body));
      const secFmt = section(++n, "Format & Resolusi", ...fmt.body);
      const secCreate = section(++n, "Buat & Kirim", ...create.body);
      const el = h("div", { style: "display:none" }, h("div", { class: "tabdesc" }, desc), ...secs, secFmt, secCreate);
      const form = {
        id, title, el, create, stop,
        refresh() {
          sources.forEach((s, i) => secs[i].classList.toggle("off", G.busy || (s.enabled ? !s.enabled() : false)));
          secFmt.classList.toggle("off", !sources.every((s) => s.ready()));
          secCreate.classList.toggle("off", !fmt.ready && !create.hasResult);
          fmt.refresh();
          create.refresh();
        },
        show(v) { el.style.display = v ? "" : "none"; if (v && onShow) onShow(); if (!v) stop(); },
      };
      fmt.onUpdate = () => form.refresh();
      create.onUpdate = () => form.refresh();
      create.isActive = () => el.style.display !== "none";
      create.onSent = () => close(true);
      return form;
    }

    // ============================================================ Tab 1: Video
    function videoForm() {
      const picker = makePicker("Pilih Video…", "video/*,.mov,.mp4,.webm,.mkv", "Video apa saja yang memiliki suara.");
      const trimmer = makeTrimmer("video", "Seret gagang hijau untuk menentukan awal & akhir segmen. Klik atau seret bar untuk menggeser kursor.");
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
        picker.info.textContent = "Membaca " + f.name + "…";
        await run("video", async () => {
          try {
            const meta = await WAAS.probeVideo(f);
            s.file = f;
            trimmer.load(f, meta);
            picker.info.textContent = `${f.name} · ${fmtT(meta.duration)} · ${meta.width}×${meta.height}`;
            log("info", "ui:video-elegido", { nombre: f.name, tipo: f.type, bytes: f.size, ...meta });
          } catch (e) {
            log("error", "ui:video-error", e);
            picker.info.textContent = "Gagal membaca video: " + e.message;
            s.file = null;
            trimmer.clear();
          }
        });
        changed();
      });
      trimmer.onChange(changed);
      const form = makeForm({
        id: "video", title: "Video",
        desc: "Gunakan video beserta suara aslinya: potong segmen yang diinginkan, lalu gambar dan audio berjalan serentak.",
        sources: [
          { title: "Pilih Video", body: [picker.el], ready: () => !!s.file },
          { title: "Potong Segmen", body: [trimmer.el], ready: () => trimmer.loaded, enabled: () => trimmer.loaded },
        ],
        fmt, getBase: () => baseName(s.file), stop: () => trimmer.stop(),
      });
      return form;
    }

    // ============================================================ Tab 2: Gambar + Audio
    function imageForm() {
      const picker = makePicker("Pilih Gambar…", "image/*", "JPG, PNG, WebP… (foto, ilustrasi, atau meme).");
      const thumb = h("img", { class: "thumb", alt: "gambar terpilih", style: "display:none" });
      const animNote = h("div", { class: "note", style: "display:none" }, "Gambar ini animasi: pada tab ini hanya frame pertama yang dipakai. Gunakan tab «Video / GIF + Audio» agar tetap bergerak.");
      const s = { file: null, url: null };
      const changed = () => form.refresh();
      const audio = makeAudioSource(changed);
      const fmt = makeFormat({
        grid: false, sync: false, still: true,
        getParams: () => (s.file && audio.ready ? { visual: { kind: "image", file: s.file }, audio: audio.params, fit: "repetir" } : null),
      });
      picker.onPick(async (f) => {
        form.create.clear();
        picker.info.textContent = "Membaca " + f.name + "…";
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
            picker.info.textContent = "Gagal membaca gambar: " + e.message;
            s.file = null; thumb.style.display = "none"; animNote.style.display = "none";
          }
        });
        changed();
      });
      const form = makeForm({
        id: "imagen", title: "Gambar + Audio",
        desc: "Padukan gambar statis dengan audio pilihan Anda. Durasi stiker mengikuti panjang potongan audio.",
        sources: [
          { title: "Pilih Gambar", body: [picker.el, thumb, animNote], ready: () => !!s.file },
          { title: "Pilih Audio", body: audio.els, ready: () => audio.ready },
        ],
        fmt, getBase: () => baseName(s.file), stop: () => audio.trimmer.stop(),
      });
      return form;
    }

    // ============================================================ Tab 3: Video atau GIF + Audio lain
    function mixForm() {
      const picker = makePicker("Pilih Video atau GIF…", "video/*,image/gif,image/webp,image/apng,.gif,.mov,.mp4,.webm,.mkv",
        "Video (suara aslinya diabaikan) atau GIF animasi.");
      const vTrim = makeTrimmer("video", "Pilih segmen animasi. Suara asli dari video ini tidak digunakan.");
      vTrim.setMuted(true);
      const vWrap = h("div", { style: "display:none" }, vTrim.el);
      const gifImg = h("img", { class: "thumb", alt: "GIF terpilih", style: "display:none" });
      const s = { file: null, kind: null, gif: null, url: null };
      const changed = () => { updateSummary(); form.refresh(); };
      const audio = makeAudioSource(changed);

      const FIT_KEY = "waas-ajuste-duracion-v2";
      const fitSel = h("select", null,
        h("option", { value: "independiente", textContent: "Masing-masing sesuai durasinya sendiri" }),
        h("option", { value: "repetir", textContent: "Ulangi animasi hingga audio selesai" }),
        h("option", { value: "congelar", textContent: "Tahan frame terakhir hingga audio selesai" }));
      fitSel.value = store.get(FIT_KEY, "independiente");
      fitSel.addEventListener("change", () => { store.set(FIT_KEY, fitSel.value); changed(); });
      const summary = h("div", { class: "summary" });
      const fitRow = h("div", null, h("div", { class: "row" }, h("span", null, "Jika durasi berbeda"), fitSel));
      const testBtn = h("button", { class: "sec", type: "button", textContent: "▶ Uji Bersama" });
      const vDur = () => (s.kind === "gif" ? s.gif.duration : vTrim.len);

      function updateSummary() {
        summary.textContent = "";
        if (!s.file || !audio.ready) { summary.append("Pilih animasi dan audio untuk melihat rincian durasi stiker."); fitRow.style.display = "none"; return; }
        const v = vDur(), a = audio.params.dur;
        const same = Math.abs(v - a) < 0.05;
        fitRow.style.display = same ? "none" : "";
        if (same) { summary.append(`Animasi dan audio berdurasi sama: ${fmtT(a)}.`); return; }
        const fit = fitSel.value;
        let txt = `Animasi ${fmtT(v)} · audio ${fmtT(a)}. `;
        if (fit === "independiente") txt += "Keduanya tetap pada durasi masing-masing: saat stiker disentuh, animasi dan audio dimulai bersamaan dan selesai terpisah.";
        else if (v > a) txt += `Animasi lebih panjang: dipotong menjadi ${fmtT(a)}.`;
        else if (fit === "repetir") txt += `Stiker berdurasi ${fmtT(a)}: animasi diulang ${(a / v).toFixed(1).replace(".", ",")} kali.`;
        else txt += `Stiker berdurasi ${fmtT(a)}: animasi berhenti pada frame terakhir sejak ${fmtT(v)}.`;
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
        extras: [summary, fitRow, h("div", { class: "row" }, testBtn, h("span", { class: "muted" }, "Pratinjau sinkronisasi animasi & audio."))],
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
        picker.info.textContent = "Membaca " + f.name + "…";
        await run("animacion", async () => {
          try {
            if (s.url) { URL.revokeObjectURL(s.url); s.url = null; }
            if (WAAS.isAnimationFile(f) || (f.type || "").startsWith("image/")) {
              const g = await WAAS.probeAnimation(f);
              if (!g.animated) throw new Error("ini adalah gambar statis. Gunakan tab «Gambar + Audio».");
              s.file = f; s.kind = "gif"; s.gif = g;
              s.url = URL.createObjectURL(f);
              gifImg.src = s.url; gifImg.style.display = ""; vWrap.style.display = "none";
              vTrim.clear();
              picker.info.textContent = `${f.name} · ${fmtT(g.duration)} · ${g.frames} frame · ${g.width}×${g.height}`;
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
            picker.info.textContent = "Tidak dapat digunakan: " + e.message;
            s.file = null; s.kind = null; s.gif = null;
            vTrim.clear(); vWrap.style.display = "none"; gifImg.style.display = "none";
          }
        });
        changed();
      });
      vTrim.onChange(changed);
      updateSummary();

      const form = makeForm({
        id: "mezcla", title: "Video / GIF + Audio",
        desc: "Gabungkan animasi (video tanpa suara aslinya atau GIF) dengan audio pilihan Anda.",
        sources: [
          { title: "Pilih Animasi", body: [picker.el, vWrap, gifImg], ready: () => !!s.file },
          { title: "Pilih Audio", body: audio.els, ready: () => audio.ready },
        ],
        fmt, getBase: () => baseName(s.file),
        stop: () => { vTrim.stop(); audio.trimmer.stop(); },
      });
      return form;
    }

    // ============================================================ panel utama
    G.forms = [videoForm(), imageForm(), mixForm()];
    const TAB_KEY = "waas-pestana";
    let active = Math.max(0, G.forms.findIndex((f) => f.id === store.get(TAB_KEY, "video")));
    const tabBtns = G.forms.map((f, i) => h("button", { type: "button", textContent: f.title, onclick: () => selectTab(i) }));
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
    const tools = h("details", null, h("summary", null, "Alat Diagnostik & Log"),
      h("div", { class: "row" },
        h("button", { class: "sec", type: "button", textContent: "Jalankan Diagnostik", onclick: () => WAAS.wa.diagnose().catch((e) => log("error", "diagnostico:error", e)) }),
        h("button", { class: "sec", type: "button", textContent: "Simpan Log (.json)…", onclick: () => saveLog().catch((e) => log("error", "registro:error", e)) })),
      logBox);

    const badgeSvg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    badgeSvg.setAttribute("viewBox", "0 0 24 24");
    badgeSvg.setAttribute("width", "18");
    badgeSvg.setAttribute("height", "18");
    badgeSvg.setAttribute("fill", "none");
    badgeSvg.setAttribute("stroke", "currentColor");
    badgeSvg.setAttribute("stroke-width", "1.9");
    badgeSvg.setAttribute("stroke-linecap", "round");
    badgeSvg.setAttribute("stroke-linejoin", "round");
    const p1 = document.createElementNS("http://www.w3.org/2000/svg", "polygon");
    p1.setAttribute("points", "11 5 6 9 2 9 2 15 6 15 11 19 11 5");
    const p2 = document.createElementNS("http://www.w3.org/2000/svg", "path");
    p2.setAttribute("d", "M15.54 8.46a5 5 0 0 1 0 7.07M19.07 4.93a10 10 0 0 1 0 14.14");
    badgeSvg.append(p1, p2);

    const closeBtn = h("button", { class: "close", type: "button", title: "Tutup (Esc)", "aria-label": "Tutup panel", textContent: "×" });
    const header = h("div", { class: "hdr" },
      h("div", { class: "hdr-info" },
        h("div", { class: "hdr-badge" }, badgeSvg),
        h("div", null,
          h("h2", null, "Buat Stiker Audio"),
          h("div", { class: "subtitle" }, "Versi " + WAAS.VERSION + " · diproses lokal di perangkat Anda · maks. 1 MB"))),
      h("div", { class: "hdr-r" },
        h("a", { class: "credit-link", href: "https://github.com/Shirtep/WA-audio-stickers", target: "_blank", rel: "noopener", title: "Basis kode oleh Shirtep (PolyForm Noncommercial 1.0.0)", textContent: "Oleh Shirtep" }),
        closeBtn));

    const body = h("div", { class: "body" }, G.forms.map((f) => f.el), tools);
    const panel = h("div", { class: "panel", role: "dialog", "aria-modal": "true", "aria-label": "Buat Stiker Audio" },
      header,
      h("nav", { class: "tabs", "aria-label": "Sumber Stiker Audio" }, tabBtns),
      body);
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
    window.addEventListener("keydown", (ev) => {
      if (ev.key === "Escape" && overlay.style.display !== "none") close();
    });
    WAAS.openPanel = open;
    WAAS.closePanel = close;
    WAAS.togglePanel = toggle;
    closeBtn.addEventListener("click", close);
    overlay.addEventListener("click", (e) => { if (e.target === overlay) close(); });

    selectTab(active);
    log("info", "ui:montada", null);
  }

  WAAS.mountUI = mount;
})();
