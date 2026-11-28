const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'sound.js'), 'utf8');

function setup(initialSounds) {
  const playCalls = [];
  const pauseCalls = [];
  const createdBlobs = [];
  const revokedUrls = [];
  let seq = 0;

  class HTMLMediaElement {
    constructor() {
      this.src = '';
      this.currentSrc = '';
      this.loop = false;
      this.volume = 1;
      this.currentTime = 0;
    }
  }
  HTMLMediaElement.prototype.play = function () {
    playCalls.push(this);
    return Promise.resolve();
  };
  HTMLMediaElement.prototype.pause = function () {
    pauseCalls.push(this);
  };

  class Audio extends HTMLMediaElement {
    constructor(src = '') {
      super();
      this.src = src;
      this.currentSrc = src;
    }
  }

  class FakeURL extends URL {
    static createObjectURL(blob) {
      createdBlobs.push(blob);
      return `blob:https://web.whatsapp.com/${++seq}`;
    }
    static revokeObjectURL(url) {
      revokedUrls.push(url);
    }
  }

  const bridgeCalls = [];
  const window = {
    __workspaceNotificationSounds: initialSounds,
    mygo: {
      call: async (method, kind) => {
        bridgeCalls.push([method, kind]);
        if (method === 'ProfileBridge.NotificationSoundData') {
          return kind === 'call' ? CUSTOM_CALL : CUSTOM_MSG;
        }
        return '';
      },
    },
  };
  window.top = window;

  const document = {
    querySelector: () => null,
  };

  vm.runInNewContext(script, {
    window,
    document,
    HTMLMediaElement,
    Audio,
    Blob,
    URL: FakeURL,
    atob: (b64) => Buffer.from(b64, 'base64').toString('binary'),
    location: { hostname: 'web.whatsapp.com', origin: 'https://web.whatsapp.com', href: 'https://web.whatsapp.com/' },
    console: { debug: () => {}, error: () => {} },
  });

  const makeAudio = (src, { loop = false } = {}) => {
    const el = new Audio(src);
    el.loop = loop;
    return el;
  };

  return { window, document, makeAudio, playCalls, pauseCalls, createdBlobs, revokedUrls, bridgeCalls, HTMLMediaElement };
}

const DEFAULT_MSG = 'https://static.whatsapp.net/rsrc.php/yv/r/BS_BUUXbKq5.mp3';
const CALL_RING = 'https://static.whatsapp.net/rsrc.php/yv/r/call_ringtone.mp3';
const CUSTOM_MSG = 'data:audio/mpeg;base64,' + Buffer.from('msg-audio').toString('base64');
const CUSTOM_CALL = 'data:audio/ogg;base64,' + Buffer.from('call-audio').toString('base64');

test('passes WhatsApp audio through untouched when mode is default', async () => {
  const { makeAudio, playCalls } = setup();
  const msg = makeAudio(DEFAULT_MSG);
  await msg.play();
  assert.deepEqual(playCalls, [msg]);
});

test('replaces incoming message chime with custom audio and respects non-WhatsApp hosts', async () => {
  const { makeAudio, playCalls, createdBlobs } = setup({
    message: { mode: 'custom', dataUrl: CUSTOM_MSG, volume: 80 },
    call: { mode: 'default', volume: 100 },
  });

  const msg = makeAudio(DEFAULT_MSG);
  await msg.play();
  assert.equal(playCalls.length, 1);
  assert.notEqual(playCalls[0], msg);
  assert.equal(playCalls[0].loop, false);
  assert.equal(playCalls[0].volume, 0.8);
  assert.equal(createdBlobs[0].type, 'audio/mpeg');
  assert.equal(await createdBlobs[0].text(), 'msg-audio');

  const external = makeAudio('https://example.com/BS_BUUXbKq5.mp3');
  await external.play();
  assert.equal(playCalls[1], external);
});

test('replaces looping incoming call ringtone and stops when WhatsApp pauses it', async () => {
  const { makeAudio, playCalls, pauseCalls } = setup({
    message: { mode: 'default', volume: 100 },
    call: { mode: 'custom', dataUrl: CUSTOM_CALL, volume: 65 },
  });

  const ring = makeAudio(CALL_RING, { loop: true });
  await ring.play();
  assert.equal(playCalls.length, 1);
  const customRing = playCalls[0];
  assert.notEqual(customRing, ring);
  assert.equal(customRing.loop, true);
  assert.equal(customRing.volume, 0.65);

  ring.pause();
  assert.deepEqual(pauseCalls, [customRing, ring]);
});

test('leaves outgoing call ringback untouched when outgoing call marker is shown', async () => {
  const { document, makeAudio, playCalls } = setup({
    message: { mode: 'default', volume: 100 },
    call: { mode: 'preset', preset: 'classic', volume: 100 },
  });
  document.querySelector = (sel) => (sel.includes('outgoing-call') ? {} : null);

  const ringback = makeAudio('https://static.whatsapp.net/rsrc.php/yv/r/ringtone_out.mp3', { loop: true });
  await ringback.play();
  assert.deepEqual(playCalls, [ringback]);
});

test('synthesizes built-in WAV presets and supports mute mode + live updates', async () => {
  const { window, makeAudio, playCalls, createdBlobs } = setup({
    message: { mode: 'preset', preset: 'crystal', volume: 90 },
    call: { mode: 'mute', volume: 100 },
  });

  const msg = makeAudio(DEFAULT_MSG);
  await msg.play();
  assert.equal(createdBlobs[0].type, 'audio/wav');
  const wavHeader = Buffer.from(await createdBlobs[0].arrayBuffer()).subarray(0, 4).toString('ascii');
  assert.equal(wavHeader, 'RIFF');

  const ring = makeAudio(CALL_RING, { loop: true });
  await ring.play();
  // Call channel is muted: neither original nor custom audio should play.
  assert.equal(playCalls.length, 1);

  window.__workspaceApplyNotificationSounds({
    message: { mode: 'default', volume: 100 },
    call: { mode: 'preset', preset: 'marimba', volume: 70 },
  });
  await msg.play();
  assert.equal(playCalls[1], msg);

  await ring.play();
  assert.equal(playCalls.length, 3);
  assert.notEqual(playCalls[2], ring);
  assert.equal(playCalls[2].loop, true);
});

test('fetches custom audio lazily via ProfileBridge when only customKey is supplied', async () => {
  const { makeAudio, playCalls, createdBlobs, bridgeCalls } = setup({
    message: { mode: 'custom', customKey: 'abc123', volume: 75 },
    call: { mode: 'default', volume: 100 },
  });

  const msg = makeAudio(DEFAULT_MSG);
  await msg.play();
  await msg.play();

  assert.deepEqual(bridgeCalls, [['ProfileBridge.NotificationSoundData', 'message']]);
  assert.equal(playCalls.length, 2);
  assert.equal(playCalls[0].volume, 0.75);
  assert.equal(createdBlobs.length, 1);
  assert.equal(await createdBlobs[0].text(), 'msg-audio');
});
