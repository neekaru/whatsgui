// ==UserScript==
// @name         WhatsApp Privacy Blur Toggle
// @namespace    whatsgui
// @version      1.1.0
// @description  Menambahkan tombol cepat dan pintasan Alt+B untuk memburamkan daftar chat serta header agar privasi terjaga saat berbagi layar.
// @author       WhatsApp GUI Example
// @match        https://web.whatsapp.com/*
// @run-at       document-idle
// ==/UserScript==

(() => {
  if (window.__waExamplePrivacyBlurLoaded) return;
  window.__waExamplePrivacyBlurLoaded = true;

  const STORAGE_KEY = "wagui_example_privacy_blur";
  const STYLE_ID = "wagui-example-privacy-blur-style";
  const BTN_ID = "wagui-example-privacy-blur-btn";

  let enabled = localStorage.getItem(STORAGE_KEY) === "1";

  const css = `
    html.wagui-privacy-blur #pane-side,
    html.wagui-privacy-blur #side header,
    html.wagui-privacy-blur #main header {
      filter: blur(7px);
      transition: filter 0.15s ease-out;
    }
    html.wagui-privacy-blur #pane-side:hover,
    html.wagui-privacy-blur #side header:hover,
    html.wagui-privacy-blur #main header:hover {
      filter: none;
    }
    #${BTN_ID} {
      position: fixed;
      top: 12px;
      right: 96px;
      z-index: 99999;
      display: inline-flex;
      align-items: center;
      gap: 6px;
      height: 26px;
      padding: 0 10px;
      border: 1px solid rgba(134, 150, 160, 0.45);
      border-radius: 3px;
      background: rgba(22, 27, 33, 0.92);
      color: #e6edf3;
      font: 600 11.5px/1 "Segoe UI", Tahoma, sans-serif;
      cursor: pointer;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
    }
    #${BTN_ID}:hover {
      background: rgba(32, 44, 51, 0.95);
    }
    #${BTN_ID}[data-active="true"] {
      border-color: #00a884;
      background: rgba(0, 168, 132, 0.92);
      color: #ffffff;
    }
  `;

  function ensureStyle() {
    if (document.getElementById(STYLE_ID)) return;
    const style = document.createElement("style");
    style.id = STYLE_ID;
    style.textContent = css;
    (document.head || document.documentElement).appendChild(style);
  }

  function applyState() {
    document.documentElement.classList.toggle("wagui-privacy-blur", enabled);
    const btn = document.getElementById(BTN_ID);
    if (btn) {
      btn.setAttribute("data-active", enabled ? "true" : "false");
      btn.textContent = enabled ? "Blur: Aktif (Alt+B)" : "Blur: Mati (Alt+B)";
      btn.title = "Klik atau tekan Alt+B untuk menyalakan/mematikan blur privasi";
    }
  }

  function toggle() {
    enabled = !enabled;
    localStorage.setItem(STORAGE_KEY, enabled ? "1" : "0");
    applyState();
  }

  function mountButton() {
    ensureStyle();
    if (!document.body || document.getElementById(BTN_ID)) {
      applyState();
      return;
    }
    const btn = document.createElement("button");
    btn.id = BTN_ID;
    btn.type = "button";
    btn.addEventListener("click", toggle);
    document.body.appendChild(btn);
    applyState();
  }

  window.addEventListener("keydown", (event) => {
    if (event.altKey && !event.ctrlKey && !event.metaKey && event.key.toLowerCase() === "b") {
      event.preventDefault();
      toggle();
    }
  });

  if (document.body) {
    mountButton();
  } else {
    document.addEventListener("DOMContentLoaded", mountButton, { once: true });
  }
})();
