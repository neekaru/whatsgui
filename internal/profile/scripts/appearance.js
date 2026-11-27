(() => {
  if (location.origin !== 'https://web.whatsapp.com') return;
  const config = window.__workspaceAppearance || { theme: 'system', privacyBlur: false };

  const systemTheme = window.matchMedia('(prefers-color-scheme: dark)');
  let themeContext;
  let themeObserver;
  let themeTimer;
  let searchTimer;

  // ThemeContext updates React, loads theme assets and persists WA preferences.
  // Writing storage alone leaves the already-rendered UI on its old theme.
  const findThemeContext = () => {
    const checked = new Set();
    for (const element of document.querySelectorAll('*')) {
      const key = Object.keys(element).find(key =>
        ['__reactFiber$', '__reactInternalInstance$', '__reactContainer$'].some(prefix => key.startsWith(prefix)));
      for (let fiber = key && element[key]; fiber && !checked.has(fiber); fiber = fiber.return) {
        checked.add(fiber);
        const value = (fiber.memoizedProps || fiber.pendingProps)?.value;
        if (typeof value?.setTheme === 'function' && typeof value?.setSystemThemeMode === 'function') {
          return value;
        }
      }
    }
  };

  const stopThemeSearch = () => {
    themeObserver?.disconnect();
    themeObserver = null;
    clearTimeout(themeTimer);
    clearTimeout(searchTimer);
    searchTimer = null;
  };

  const syncTheme = () => {
    const wanted = config.theme === 'system' ? (systemTheme.matches ? 'dark' : 'light') : config.theme;
    themeContext ||= findThemeContext();
    if (!themeContext || typeof window.require !== 'function') return false;
    let prefs;
    let reactDOM;
    try {
      prefs = window.require('WAWebUserPrefsGeneral');
      reactDOM = window.require('ReactDOM');
    } catch (_) {
      return false; // The module may not be registered until WhatsApp finishes booting.
    }
    if (typeof prefs?.getTheme !== 'function' || typeof prefs?.getSystemThemeMode !== 'function' ||
        typeof reactDOM?.flushSync !== 'function') {
      throw new Error('WhatsApp theme API is unavailable');
    }
    // Commit each selection before another can hit WA's state-based early return.
    reactDOM.flushSync(() => {
      // Resolve system mode ourselves, as ZapZap does, so OS changes update React too.
      if (prefs.getSystemThemeMode() || themeContext.systemThemeMode) themeContext.setSystemThemeMode(false);
      if (prefs.getTheme() !== wanted || themeContext.theme !== wanted) themeContext.setTheme(wanted);
    });
    stopThemeSearch();
    return true;
  };

  const applyTheme = () => {
    try {
      if (syncTheme() || themeObserver || !document.documentElement) return;
      themeObserver = new MutationObserver(() => {
        if (searchTimer) return;
        // Timers also run in background profiles, unlike animation frames.
        searchTimer = setTimeout(() => {
          searchTimer = null;
          applyTheme();
        }, 100);
      });
      themeObserver.observe(document.documentElement, { childList: true, subtree: true });
      // Call popups may never mount ThemeContext; stop scanning without an error.
      themeTimer = setTimeout(stopThemeSearch, 60000);
    } catch (error) {
      stopThemeSearch();
      console.error('[Workspace theme]', error);
    }
  };

  // Seed the bootloader before React is ready; later changes use ThemeContext.
  const initialTheme = config.theme === 'system' ? (systemTheme.matches ? 'dark' : 'light') : config.theme;
  localStorage.setItem('theme', JSON.stringify(initialTheme));
  localStorage.setItem('system-theme-mode', 'false');
  systemTheme.addEventListener('change', () => {
    if (config.theme === 'system') applyTheme();
  });

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
    applyTheme();
    applyBlur(config.privacyBlur);
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
    applyTheme();
    // WhatsApp rewrites <html>'s class as it switches its own theme; keep ours.
    new MutationObserver(() => {
      if (config.privacyBlur && !document.documentElement.classList.contains('__workspace-blur')) {
        applyBlur(true);
      }
    }).observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
  };

  if (document.documentElement) start();
  else document.addEventListener('DOMContentLoaded', start, { once: true });
})();
