const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'unread.js'), 'utf8');

function run({ title, hasChatList = true }) {
  const calls = [];
  let tick = null;
  const window = {
    addEventListener: () => {},
    mygo: { call: (method, count) => { calls.push([method, count]); return Promise.resolve(); } },
  };
  window.top = window;
  const document = {
    title,
    querySelector: () => (hasChatList ? {} : null),
    addEventListener: () => {},
  };
  vm.runInNewContext(script, {
    window,
    document,
    location: { origin: 'https://web.whatsapp.com' },
    setInterval: fn => { tick = fn; return 1; },
    console,
  });
  return { calls, report: () => tick() };
}

test('reports the title aggregate, and nothing on the login page', () => {
  const login = run({ title: 'WhatsApp', hasChatList: false });
  assert.deepEqual(login.calls, [], 'login page must not report a zero');

  const page = run({ title: '(3) WhatsApp' });
  assert.deepEqual(page.calls, [['ProfileBridge.SetUnread', 3]]);

  page.report();
  assert.equal(page.calls.length, 1, 'an unchanged count is not sent again');

  page.calls.length = 0;
  assert.deepEqual(run({ title: '(1.234) WhatsApp' }).calls, [['ProfileBridge.SetUnread', 1234]]);
  assert.deepEqual(run({ title: 'WhatsApp' }).calls, [['ProfileBridge.SetUnread', 0]]);
});
