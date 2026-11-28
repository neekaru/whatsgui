// ==UserScript==
// @name         Stiker Audio WhatsApp (WA-audio-stickers)
// @namespace    whatsappgui-new
// @version      0.7.2
// @description  Menambahkan pilihan Stiker Biasa atau Stiker Audio saat menekan tombol Buat (ic-add) di panel stiker WhatsApp Web.
// @author       Shirtep (extension) & WhatsApp GUI
// @match        https://web.whatsapp.com/*
// @run-at       document-idle
// @require      ./wa-audio-stickers/logger.js
// @require      ./wa-audio-stickers/convert.js
// @require      ./wa-audio-stickers/wa.js
// @require      ./wa-audio-stickers/ui.js
// @require      ./wa-audio-stickers/main.js
// ==/UserScript==
//
// Required Notice: Copyright Shirtep (https://github.com/Shirtep/WA-audio-stickers)
// License: PolyForm Noncommercial License 1.0.0 (https://polyformproject.org/licenses/noncommercial/1.0.0)

(() => {
  if (location.origin !== "https://web.whatsapp.com" || window !== window.top) return;
  if (window.__waStickerChooserLoaded) return;
  window.__waStickerChooserLoaded = true;

  let bypassIntercept = false;
  let activeButton = null;
  let host = null;
  let menu = null;

  function isDarkTheme() {
    try {
      return document.body.classList.contains("dark") ||
        document.documentElement.classList.contains("dark") ||
        window.matchMedia("(prefers-color-scheme: dark)").matches;
    } catch (_) {
      return false;
    }
  }

  function ensureChooser() {
    if (host && host.isConnected && menu) return;
    host = document.createElement("div");
    host.id = "wagui-sticker-chooser-root";
    const shadow = host.attachShadow({ mode: "open" });

    const style = document.createElement("style");
    style.textContent = `
      :host { all: initial; }
      * { box-sizing: border-box; font-family: "Segoe UI", system-ui, sans-serif; }
      .menu {
        position: fixed;
        z-index: 2147483002;
        min-width: 248px;
        max-width: 290px;
        padding: 6px;
        border-radius: 12px;
        background: #ffffff;
        color: #111b21;
        border: 1px solid #d1d7db;
        box-shadow: 0 10px 30px rgba(0, 0, 0, 0.22);
        display: none;
        flex-direction: column;
        gap: 4px;
      }
      .menu.dark {
        background: #233138;
        color: #e9edef;
        border-color: #2f3e46;
        box-shadow: 0 10px 30px rgba(0, 0, 0, 0.45);
      }
      .title {
        padding: 6px 10px 4px;
        font-size: 11px;
        font-weight: 700;
        letter-spacing: 0.03em;
        text-transform: uppercase;
        color: #667781;
      }
      .menu.dark .title {
        color: #8696a0;
      }
      .item {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        gap: 2px;
        width: 100%;
        padding: 9px 10px;
        border: none;
        border-radius: 8px;
        background: transparent;
        color: inherit;
        text-align: left;
        cursor: pointer;
        transition: background-color 0.12s ease;
      }
      .item:hover {
        background: #f0f2f5;
      }
      .menu.dark .item:hover {
        background: #182229;
      }
      .label {
        font-size: 13px;
        font-weight: 600;
      }
      .sub {
        font-size: 11.5px;
        color: #667781;
        line-height: 1.3;
      }
      .menu.dark .sub {
        color: #8696a0;
      }
    `;

    menu = document.createElement("div");
    menu.className = "menu";
    menu.setAttribute("role", "menu");
    menu.setAttribute("aria-label", "Pilih jenis stiker");

    const titleEl = document.createElement("div");
    titleEl.className = "title";
    titleEl.textContent = "Buat Stiker";

    function makeItem(labelText, subText, onSelect) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "item";
      btn.setAttribute("role", "menuitem");

      const lbl = document.createElement("span");
      lbl.className = "label";
      lbl.textContent = labelText;

      const sub = document.createElement("span");
      sub.className = "sub";
      sub.textContent = subText;

      btn.append(lbl, sub);
      btn.addEventListener("click", (ev) => {
        ev.stopPropagation();
        onSelect();
      });
      return btn;
    }

    const normalBtn = makeItem(
      "Stiker Biasa",
      "Buat stiker gambar bawaan WhatsApp",
      () => {
        const target = activeButton;
        closeChooser();
        if (!target || !target.isConnected) return;
        bypassIntercept = true;
        try {
          target.click();
        } finally {
          setTimeout(() => { bypassIntercept = false; }, 0);
        }
      },
    );

    const audioBtn = makeItem(
      "Stiker Audio",
      "Buat stiker bersuara dari video, GIF, atau gambar",
      () => {
        closeChooser();
        if (window.WAAS && typeof window.WAAS.togglePanel === "function") {
          window.WAAS.togglePanel();
        } else {
          window.postMessage({ __waas: "togglePanel" }, location.origin);
        }
      },
    );

    menu.append(titleEl, normalBtn, audioBtn);
    shadow.append(style, menu);
    document.body.append(host);
  }

  function closeChooser() {
    if (menu) menu.style.display = "none";
    activeButton = null;
  }

  function openChooser(buttonEl) {
    ensureChooser();
    activeButton = buttonEl;
    menu.classList.toggle("dark", isDarkTheme());
    menu.style.display = "flex";
    menu.style.visibility = "hidden";

    const rect = buttonEl.getBoundingClientRect();
    const mw = menu.offsetWidth || 260;
    const mh = menu.offsetHeight || 132;
    let left = rect.left;
    if (left + mw > window.innerWidth - 12) left = window.innerWidth - mw - 12;
    if (left < 12) left = 12;
    let top = rect.top - chOrDefault(mh) - 8;
    if (top < 12) top = Math.min(window.innerHeight - chOrDefault(mh) - 12, rect.bottom + 8);
    menu.style.left = Math.round(left) + "px";
    menu.style.top = Math.round(Math.max(12, top)) + "px";
    menu.style.visibility = "visible";
  }

  function chOrDefault(v) {
    return v > 0 ? v : 132;
  }

  function findCreateStickerButton(target) {
    let el = target && target.nodeType === 1 ? target : target && target.parentElement;
    for (let depth = 0; el && depth < 6; depth++, el = el.parentElement) {
      const title = el.querySelector("svg > title");
      if (title && title.textContent && title.textContent.trim() === "ic-add") {
        const text = (el.textContent || "").replace(/ic-add/g, "").trim();
        if (/^(Buat|Create|Crear)$/i.test(text)) {
          return el;
        }
      }
    }
    return null;
  }

  document.addEventListener("click", (ev) => {
    if (bypassIntercept) return;
    if (host && (ev.target === host || host.contains(ev.target))) return;

    const btn = findCreateStickerButton(ev.target);
    if (btn) {
      ev.preventDefault();
      ev.stopPropagation();
      ev.stopImmediatePropagation();
      openChooser(btn);
      return;
    }

    if (menu && menu.style.display !== "none") {
      closeChooser();
    }
  }, true);

  window.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape" && menu && menu.style.display !== "none") {
      closeChooser();
    }
  }, true);
})();
