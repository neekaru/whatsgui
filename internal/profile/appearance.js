(() => {
  if (location.origin !== 'https://web.whatsapp.com') return;
  const config = window.__workspaceAppearance || { theme: 'system', privacyBlur: false };

  // WhatsApp keeps its theme in localStorage, where its own Settings → Theme
  // writes it, and its bootloader reads it before any page script runs: this
  // preload therefore has to write it first. "system" follows Windows through
  // prefers-color-scheme, which is what system-theme-mode asks for.
  const syncTheme = (theme) => {
    const wanted = theme === 'dark' ? '"dark"' : theme === 'light' ? '"light"' : null;
    const state = () => `${localStorage.getItem('theme')}|${localStorage.getItem('system-theme-mode')}`;
    const before = state();
    if (wanted === null) {
      localStorage.removeItem('theme');
      localStorage.setItem('system-theme-mode', 'true');
    } else {
      localStorage.setItem('theme', wanted);
      localStorage.removeItem('system-theme-mode');
    }
    return before !== state();
  };

  const STYLE_ID = '__workspace_privacy_blur';
  // #pane-side holds the chat list with names and message previews, #main's
  // header the open chat's name. Each part reveals itself on hover, so the
  // window can be shared without showing who wrote what.
  const blurCSS = `
html.__workspace-blur #pane-side,
html.__workspace-blur #side header,
html.__workspace-blur #main header { filter: blur(7px); transition: filter .12s ease-out; }
html.__workspace-blur #pane-side:hover,
html.__workspace-blur #side header:hover,
html.__workspace-blur #main header:hover { filter: none; }
html.__workspace-blur-fallback #app { filter: blur(12px); transition: filter .12s ease-out; }
html.__workspace-blur-fallback #app:hover { filter: none; }`;

  const applyBlur = (on) => {
    const root = document.documentElement;
    if (!root) return;
    root.classList.toggle('__workspace-blur', Boolean(on));
    if (!on) root.classList.remove('__workspace-blur-fallback');
    let style = document.getElementById(STYLE_ID);
    if (!on) {
      style?.remove();
      return;
    }
    if (!style) {
      style = document.createElement('style');
      style.id = STYLE_ID;
      (document.head || root).appendChild(style);
    }
    style.textContent = blurCSS;
  };

  const apply = (next) => {
    Object.assign(config, next || {});
    const changed = syncTheme(config.theme);
    applyBlur(config.privacyBlur);
    // At document start WhatsApp has not read its theme yet, so a change there
    // needs no reload; a change on a page it already rendered does.
    return changed && document.readyState !== 'loading';
  };

  // 'pending' means there is no signed-in UI yet, so nothing is exposed.
  window.__workspaceBlurState = () => {
    if (!config.privacyBlur) return 'off';
    const pane = document.querySelector('#pane-side');
    if (!pane) return document.querySelector('#main') ? 'drift' : 'pending';
    const filtered = getComputedStyle(pane).filter;
    return filtered && filtered !== 'none' ? 'ok' : 'drift';
  };
  // Fail closed: when WhatsApp's layout moved, the whole app blurs instead of
  // pretending the previews are hidden.
  window.__workspaceBlurFallback = () => {
    document.documentElement?.classList.add('__workspace-blur-fallback');
    return true;
  };
  window.__workspaceApplyAppearance = apply;

  const start = () => {
    applyBlur(config.privacyBlur);
    // WhatsApp rewrites <html>'s class as it switches its own theme; keep ours.
    new MutationObserver(() => {
      if (config.privacyBlur && !document.documentElement.classList.contains('__workspace-blur')) {
        applyBlur(true);
      }
    }).observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
  };

  apply(config);
  if (document.documentElement) start();
  else document.addEventListener('DOMContentLoaded', start, { once: true });
})();
