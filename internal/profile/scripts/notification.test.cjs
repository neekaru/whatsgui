const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'notification.js'), 'utf8');

function page(call) {
  const calls = [];
  const images = [];
  let timeout;
  const window = { mygo: { call: (...args) => { calls.push(args); return call?.(...args) ?? Promise.resolve(); } } };
  window.top = window;
  class TestImage {
    constructor() { images.push(this); }
  }
  class ServiceWorkerRegistration {}
  vm.runInNewContext(script, {
    window, Image: TestImage, ServiceWorkerRegistration, EventTarget, Event, URL,
    location: { origin: 'https://web.whatsapp.com', href: 'https://web.whatsapp.com/' },
    setTimeout: (fn) => { timeout = fn; return 1; }, clearTimeout: () => {},
    console: { error: () => {} },
  });
  return { window, calls, images, ServiceWorkerRegistration, expire: () => timeout() };
}

for (const failure of ['load failure', 'timeout']) {
  test(`photo ${failure} still delivers a text notification`, async () => {
    const p = page();
    const notification = new p.window.Notification('Contact', { body: 'New message', icon: 'blob:https://web.whatsapp.com/photo' });
    let shown = 0;
    notification.onshow = () => shown++;
    if (failure === 'timeout') p.expire(); else p.images[0].onerror();
    await notification.ready;
    assert.deepEqual(p.calls.map((args) => [args[0], args[2], args[5]]), [['ProfileBridge.Notify', 'New message', '']]);
    assert.equal(shown, 1);
  });
}

test('closing while the photo loads never shows a ghost notification', async () => {
  const p = page();
  const notification = new p.window.Notification('Contact', { icon: 'blob:https://web.whatsapp.com/photo' });
  let shown = 0;
  notification.onshow = () => shown++;
  notification.close();
  p.images[0].onerror();
  await notification.ready;
  assert.deepEqual(p.calls.map((args) => args[0]), ['ProfileBridge.Dismiss']);
  assert.equal(shown, 0);
});

test('service-worker delivery waits for the native result and reports errors', async () => {
  let entered;
  const called = new Promise((resolve) => { entered = resolve; });
  let reject;
  const p = page(() => new Promise((resolve, fail) => { reject = fail; entered(); }));
  const registration = new p.ServiceWorkerRegistration();
  let settled = false;
  const result = registration.showNotification('Contact', {});
  result.catch(() => { settled = true; });
  await called;
  assert.equal(settled, false);
  reject(new Error('native delivery failed'));
  await assert.rejects(result, /native delivery failed/);
});
