const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'appearance.js'), 'utf8');

function page({ theme = 'system', dark = false, ready = true } = {}) {
  const values = new Map();
  const timers = new Map();
  const observers = [];
  const errors = [];
  let sequence = 0;
  let mediaChanged;
  let available = ready;
  const state = { theme: 'light', system: true };
  let pending = {};
  const context = {
    // This value can lag behind preferences while bootstrapping React.
    theme: 'light', systemThemeMode: true,
    setTheme(value) {
      if (state.theme === value) return;
      pending.theme = value;
      values.set('theme', JSON.stringify(value));
    },
    setSystemThemeMode(value) {
      pending.system = value;
      values.set('system-theme-mode', JSON.stringify(value));
    },
  };
  const element = { __reactFiber$test: { memoizedProps: { value: context }, return: null } };
  const media = { matches: dark, addEventListener: (_, callback) => { mediaChanged = callback; } };
  const window = {
    __workspaceAppearance: { theme, privacyBlur: false },
    matchMedia: () => media,
    require: name => name === 'ReactDOM' ? {
      flushSync(callback) { callback(); Object.assign(state, pending); pending = {}; },
    } : ({
      getTheme: () => JSON.parse(values.get('theme') || '"light"'),
      getSystemThemeMode: () => JSON.parse(values.get('system-theme-mode') || 'true'),
    }),
  };
  const document = {
    documentElement: { classList: { toggle() {}, remove() {}, contains: () => true } },
    querySelectorAll: () => available ? [element] : [],
    getElementById: () => null,
  };
  vm.runInNewContext(script, {
    window, document, location: { origin: 'https://web.whatsapp.com' },
    localStorage: { setItem: (key, value) => values.set(key, value) },
    MutationObserver: class {
      constructor(callback) { this.callback = callback; observers.push(this); }
      observe(_, options) { this.active = true; this.children = options.childList; }
      disconnect() { this.active = false; }
    },
    setTimeout: (callback, delay) => { const id = ++sequence; timers.set(id, { callback, delay }); return id; },
    clearTimeout: id => timers.delete(id),
    console: { error: (...args) => errors.push(args.join(' ')) },
  });
  return {
    state, values, errors,
    apply: theme => window.__workspaceApplyAppearance({ theme }),
    osTheme(dark) { media.matches = dark; mediaChanged(); },
    mount() {
      available = true;
      for (const observer of observers.filter(o => o.active && o.children)) observer.callback();
      for (const [id, timer] of timers) {
        if (timer.delay !== 100) continue;
        timers.delete(id);
        timer.callback();
      }
    },
    searching: () => observers.some(o => o.active && o.children),
  };
}

test('updates rendered theme even when boot preferences already match; explicit mode overrides OS', () => {
  const p = page({ theme: 'dark' });
  assert.equal(p.state.theme, 'dark');
  assert.equal(p.state.system, false);
  p.osTheme(true);
  p.apply('light');
  assert.equal(p.state.theme, 'light');
  p.osTheme(false);
  p.osTheme(true);
  assert.equal(p.state.theme, 'light');
  p.apply('system');
  assert.equal(p.state.theme, 'dark');
  p.osTheme(false);
  assert.equal(p.state.theme, 'light');
  assert.deepEqual(p.errors, []);
});

test('uses the latest selection when WhatsApp mounts after theme changes', () => {
  const p = page({ theme: 'light', ready: false });
  p.apply('dark');
  assert.equal(p.searching(), true);
  p.mount();
  assert.equal(p.state.theme, 'dark');
  assert.equal(p.searching(), false);
  assert.equal(p.values.get('theme'), '"dark"');
  assert.deepEqual(p.errors, []);
});
