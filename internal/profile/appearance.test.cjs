const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'appearance.js'), 'utf8');

test('applies WhatsApp theme changes without resetting them on reload', () => {
  const values = new Map([['theme', '"dark"']]);
  const localStorage = {
    getItem: key => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
    removeItem: key => values.delete(key),
  };
  const window = {};
  const root = { classList: { toggle() {}, remove() {}, contains() { return true; } } };
  const document = { documentElement: root, readyState: 'complete', getElementById: () => null };
  const context = { window, document, localStorage, location: { origin: 'https://web.whatsapp.com' }, MutationObserver: class { observe() {} } };
  vm.runInNewContext(script, context);
  assert.equal(values.get('theme'), '"dark"');
  assert.equal(window.__workspaceApplyAppearance({ theme: 'light' }), true);
  assert.equal(values.get('theme'), '"light"');
  vm.runInNewContext(script, context);
  assert.equal(values.get('theme'), '"light"');
  assert.equal(window.__workspaceApplyAppearance({ theme: 'light' }), false);
  assert.equal(window.__workspaceApplyAppearance({ theme: 'system' }), true);
  assert.equal(values.has('theme'), false);
  assert.equal(values.get('system-theme-mode'), 'true');
});
