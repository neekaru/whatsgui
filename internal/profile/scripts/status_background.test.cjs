const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'status_background.js'), 'utf8');

test('only a playing status paused by backgrounding is resumed once', async () => {
  const listeners = { window: {}, document: {} };
  const timers = new Map();
  let next = 0;
  let focused = true;
  class Video {
    constructor(status, paused = false) { this.status = status; this.paused = paused; this.ended = false; this.isConnected = true; this.calls = 0; }
    closest() { return this.status ? {} : null; }
    play() { this.calls++; this.paused = false; return Promise.resolve(); }
  }
  const status = new Video(true);
  const other = new Video(false);
  const paused = new Video(true, true);
  const videos = [status, other, paused];
  const window = { addEventListener: (name, fn) => { listeners.window[name] = fn; } };
  window.top = window;
  const document = {
    hidden: false,
    hasFocus: () => focused,
    querySelectorAll: () => videos,
    addEventListener: (name, fn) => { listeners.document[name] = fn; },
  };
  vm.runInNewContext(script, {
    window, document, location: { origin: 'https://web.whatsapp.com' }, HTMLVideoElement: Video,
    setTimeout: (fn, ms) => { timers.set(++next, { fn, ms }); return next; },
    clearTimeout: id => timers.delete(id), console,
  });
  const run = ms => { for (const [id, timer] of [...timers]) if (timer.ms === ms) { timers.delete(id); timer.fn(); } };
  focused = false;
  listeners.window.blur();
  status.paused = true;
  listeners.document.pause({ target: status });
  listeners.document.pause({ target: other });
  listeners.document.pause({ target: paused });
  run(0);
  assert.equal(status.calls, 1);
  assert.equal(other.calls, 0);
  assert.equal(paused.calls, 0);
  status.paused = true;
  listeners.document.pause({ target: status });
  run(0);
  assert.equal(status.calls, 1, 'later pause must remain paused');

  status.paused = false;
  document.hidden = true;
  listeners.document.visibilitychange();
  listeners.window.blur();
  status.paused = true;
  listeners.document.pause({ target: status });
  focused = true;
  listeners.window.focus();
  run(0);
  assert.equal(status.calls, 1, 'focus before resume must cancel it');

  status.paused = false;
  focused = false;
  listeners.window.blur();
  run(1500);
  status.paused = true;
  listeners.document.pause({ target: status });
  run(0);
  assert.equal(status.calls, 1, 'expired background transition must not resume');
});
