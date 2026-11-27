const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'passive_events.js'), 'utf8');

test('React root and portal handlers retain inherited once, abort and capture lifetimes', () => {
  // Node supplies real dispatch/removal/AbortSignal behavior. Chromium smoke
  // additionally checks passive cancellation, which Node does not implement.
  class Target extends EventTarget {}
  const context = { EventTarget: Target, location: { origin: 'https://web.whatsapp.com' } };
  vm.runInNewContext(script, context);
  vm.runInNewContext(script, context);
  for (const marker of ['__reactContainer$test', '_reactListeningtest']) {
    for (const type of ['wheel', 'touchstart', 'touchmove']) {
      const root = new Target();
      root[marker] = true;
      const once = Object.create({ passive: true, once: true });
      root.addEventListener(type, event => event.preventDefault(), once);
      const first = new Event(type, { cancelable: true });
      root.dispatchEvent(first);
      assert.equal(first.defaultPrevented, true);
      const second = new Event(type, { cancelable: true });
      root.dispatchEvent(second);
      assert.equal(second.defaultPrevented, false, 'once listener must not cancel subsequent input');
      assert.equal(once.passive, true);

      const controller = new AbortController();
      const options = Object.create({ passive: true, capture: true, signal: controller.signal });
      const cancel = event => event.preventDefault();
      root.addEventListener(type, cancel, options);
      controller.abort();
      const aborted = new Event(type, { cancelable: true });
      root.dispatchEvent(aborted);
      assert.equal(aborted.defaultPrevented, false, 'aborted listener must not consume input');

      root.addEventListener(type, cancel, { passive: true, capture: true });
      root.removeEventListener(type, cancel, { capture: true });
      const removed = new Event(type, { cancelable: true });
      root.dispatchEvent(removed);
      assert.equal(removed.defaultPrevented, false, 'removed capture listener must not consume input');
    }
  }
});
