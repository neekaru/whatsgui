const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'media.js'), 'utf8');

test('selected devices apply to later calls and popup pages', async () => {
  const storage = new Map();
  const requests = [];
  const createPage = () => {
    const mediaDevices = {
      enumerateDevices: async () => [
        { kind: 'audioinput', deviceId: 'mic', label: 'Mic' },
        { kind: 'videoinput', deviceId: 'cam', label: 'Camera' },
      ],
      getUserMedia: async (constraints) => {
        requests.push(constraints);
        if (constraints.video?.deviceId?.exact === 'missing') throw Object.assign(new Error('unavailable'), { name: 'OverconstrainedError' });
        return { getTracks: () => [] };
      },
    };
    const window = {};
    window.top = window;
    const localStorage = { getItem: (key) => storage.get(key) ?? null, setItem: (key, value) => storage.set(key, value) };
    vm.runInNewContext(script, { window, localStorage, navigator: { mediaDevices }, location: { origin: 'https://web.whatsapp.com' } });
    return { window, mediaDevices };
  };
  const page = createPage();
  page.window.__workspaceSetMediaDevices('mic', 'cam');
  // No MyGo bridge exists in a popup's first document.
  const popup = createPage();
  await popup.mediaDevices.getUserMedia({ audio: { echoCancellation: true }, video: true });
  assert.equal(requests[0].audio.deviceId.exact, 'mic');
  assert.equal(requests[0].audio.echoCancellation, true);
  assert.equal(requests[0].video.deviceId.exact, 'cam');
  await popup.mediaDevices.getUserMedia({ audio: false, video: true });
  assert.equal(requests[1].audio, false);
  page.window.__workspaceSetMediaDevices('', 'missing');
  await assert.rejects(popup.mediaDevices.getUserMedia({ video: true }), { name: 'OverconstrainedError' });
  const list = await popup.window.__workspaceMediaDevices();
  assert.equal(list.videoId, 'missing');
  assert.equal(list.audioId, '');
});
