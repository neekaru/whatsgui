(() => {
  if (!/(^|\.)whatsapp\.(net|com)$/.test(location.hostname)) return;
  if (HTMLMediaElement.prototype.play?.__workspaceSoundHooked) return;

  const MESSAGE_SOUND_FILES = new Set(['BS_BUUXbKq5.mp3']);
  const HOSTNAME = /(^|\.)whatsapp\.(net|com)$/;
  const SAMPLE_RATE = 22050;

  const PRESET_NOTES = {
    'message:chime': [
      { start: 0, dur: 0.14, freq: 659.25, gain: 0.28 },
      { start: 0.11, dur: 0.26, freq: 987.77, gain: 0.26 },
    ],
    'message:crystal': [
      { start: 0, dur: 0.12, freq: 880, gain: 0.24 },
      { start: 0.08, dur: 0.14, freq: 1174.66, gain: 0.24 },
      { start: 0.17, dur: 0.28, freq: 1567.98, gain: 0.22 },
    ],
    'message:pop': [
      { start: 0, dur: 0.09, freq: 523.25, freqEnd: 880, gain: 0.30 },
    ],
    'message:bell': [
      { start: 0, dur: 0.42, freq: 1046.5, harmonic: 2, gain: 0.26 },
    ],
    'call:classic': [
      { start: 0.0, dur: 0.14, freq: 783.99, gain: 0.28 },
      { start: 0.16, dur: 0.14, freq: 987.77, gain: 0.28 },
      { start: 0.32, dur: 0.14, freq: 783.99, gain: 0.28 },
      { start: 0.48, dur: 0.28, freq: 987.77, gain: 0.28 },
      { start: 1.0, dur: 0.05, freq: 0, gain: 0 },
    ],
    'call:marimba': [
      { start: 0.0, dur: 0.16, freq: 523.25, gain: 0.30 },
      { start: 0.14, dur: 0.16, freq: 659.25, gain: 0.28 },
      { start: 0.28, dur: 0.16, freq: 783.99, gain: 0.28 },
      { start: 0.44, dur: 0.30, freq: 1046.5, gain: 0.26 },
      { start: 1.15, dur: 0.05, freq: 0, gain: 0 },
    ],
    'call:pulse': [
      { start: 0.0, dur: 0.18, freq: 698.46, gain: 0.28 },
      { start: 0.26, dur: 0.18, freq: 698.46, gain: 0.28 },
      { start: 0.52, dur: 0.22, freq: 880.0, gain: 0.28 },
      { start: 1.15, dur: 0.05, freq: 0, gain: 0 },
    ],
    'call:gentle': [
      { start: 0.0, dur: 0.28, freq: 440, gain: 0.22 },
      { start: 0.24, dur: 0.30, freq: 554.37, gain: 0.22 },
      { start: 0.50, dur: 0.42, freq: 659.25, gain: 0.20 },
      { start: 1.35, dur: 0.05, freq: 0, gain: 0 },
    ],
  };

  const presetUrlCache = new Map();
  const customUrlCache = new Map();
  const channelAudios = { message: null, call: null };
  const activeCustom = new WeakMap();

  const defaultConfig = () => ({
    message: { mode: 'default', preset: 'chime', dataUrl: '', volume: 100 },
    call: { mode: 'default', preset: 'classic', dataUrl: '', volume: 100 },
  });

  const config = defaultConfig();

  function encodeWavUrl(notes) {
    let totalSec = 0.2;
    for (const n of notes) totalSec = Math.max(totalSec, n.start + n.dur);
    const totalSamples = Math.max(1, Math.ceil(totalSec * SAMPLE_RATE));
    const buffer = new ArrayBuffer(44 + totalSamples * 2);
    const view = new DataView(buffer);
    const writeStr = (offset, text) => {
      for (let i = 0; i < text.length; i++) view.setUint8(offset + i, text.charCodeAt(i));
    };
    writeStr(0, 'RIFF');
    view.setUint32(4, 36 + totalSamples * 2, true);
    writeStr(8, 'WAVE');
    writeStr(12, 'fmt ');
    view.setUint32(16, 16, true);
    view.setUint16(20, 1, true);
    view.setUint16(22, 1, true);
    view.setUint32(24, SAMPLE_RATE, true);
    view.setUint32(28, SAMPLE_RATE * 2, true);
    view.setUint16(32, 2, true);
    view.setUint16(34, 16, true);
    writeStr(36, 'data');
    view.setUint32(40, totalSamples * 2, true);

    for (let i = 0; i < totalSamples; i++) {
      const t = i / SAMPLE_RATE;
      let sample = 0;
      for (const note of notes) {
        if (!note.freq || t < note.start || t > note.start + note.dur) continue;
        const local = t - note.start;
        const progress = local / note.dur;
        const env = Math.min(1, local / 0.012) * Math.exp(-3.4 * progress);
        const freq = note.freqEnd
          ? note.freq + (note.freqEnd - note.freq) * progress
          : note.freq;
        sample += Math.sin(2 * Math.PI * freq * local) * note.gain * env;
        if (note.harmonic) {
          sample += Math.sin(2 * Math.PI * freq * note.harmonic * local) * (note.gain * 0.35) * env;
        }
      }
      const clamped = Math.max(-1, Math.min(1, sample));
      view.setInt16(44 + i * 2, Math.round(clamped * 32767), true);
    }
    return URL.createObjectURL(new Blob([buffer], { type: 'audio/wav' }));
  }

  function dataUrlToBlobUrl(dataUrl) {
    if (customUrlCache.has(dataUrl)) return customUrlCache.get(dataUrl);
    const match = /^data:([^;,]*)(?:;[^,]*)?;base64,(.*)$/s.exec(dataUrl);
    if (!match) throw new Error('Not a base64 data URL');
    const binary = atob(match[2]);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    const blobUrl = URL.createObjectURL(new Blob([bytes], { type: match[1] || 'audio/mpeg' }));
    customUrlCache.set(dataUrl, blobUrl);
    return blobUrl;
  }

  function resolveChannelUrl(kind, ch) {
    if (!ch || ch.mode === 'default' || ch.mode === 'mute') return '';
    if (ch.mode === 'custom' && ch.dataUrl) {
      return dataUrlToBlobUrl(ch.dataUrl);
    }
    if (ch.mode === 'preset') {
      const key = `${kind}:${ch.preset || (kind === 'call' ? 'classic' : 'chime')}`;
      if (presetUrlCache.has(key)) return presetUrlCache.get(key);
      const notes = PRESET_NOTES[key] || PRESET_NOTES[kind === 'call' ? 'call:classic' : 'message:chime'];
      const url = encodeWavUrl(notes);
      presetUrlCache.set(key, url);
      return url;
    }
    return '';
  }

  function isOutgoingCall(element, parsed) {
    const path = parsed.pathname.toLowerCase();
    if (/ringback|outgoing|dial/.test(path)) return true;
    if (typeof document === 'undefined' || !document.querySelector) return false;
    if (document.querySelector('[data-testid*="incoming"], button[aria-label*="Terima"], button[aria-label*="Tolak"], button[aria-label*="Accept"], button[aria-label*="Decline"]')) {
      return false;
    }
    if (document.querySelector('[data-testid="outgoing-call"], [data-testid*="ringing"]')) {
      return true;
    }
    return false;
  }

  function classify(element) {
    const raw = element.currentSrc || element.src;
    if (!raw) return null;
    let parsed;
    try {
      parsed = new URL(raw, location.href);
    } catch {
      return null;
    }
    if (!HOSTNAME.test(parsed.hostname)) return null;
    const file = parsed.pathname.split('/').pop() || '';
    if (!element.loop) {
      if (MESSAGE_SOUND_FILES.has(file) || /^(?:notification|incoming[_-]?message)/i.test(file)) {
        return { kind: 'message', url: raw };
      }
      return { kind: null, url: raw };
    }
    if (isOutgoingCall(element, parsed)) {
      return { kind: null, url: raw };
    }
    return { kind: 'call', url: raw };
  }

  function clampVolume(vol) {
    const n = Number(vol);
    if (!Number.isFinite(n) || n <= 0) return 1;
    return Math.min(1, Math.max(0.05, n / 100));
  }

  function stopChannelAudio(kind) {
    const audio = channelAudios[kind];
    if (!audio) return;
    try {
      originalPause.call(audio);
      audio.currentTime = 0;
    } catch {}
  }

  function stopAllCustomSounds() {
    stopChannelAudio('message');
    stopChannelAudio('call');
  }

  function ensureChannelAudio(kind) {
    const ch = config[kind];
    if (!ch || ch.mode === 'default' || ch.mode === 'mute') {
      stopChannelAudio(kind);
      channelAudios[kind] = null;
      return null;
    }
    const url = resolveChannelUrl(kind, ch);
    if (!url) {
      channelAudios[kind] = null;
      return null;
    }
    let audio = channelAudios[kind];
    if (!audio) {
      audio = new Audio(url);
      channelAudios[kind] = audio;
    } else if (audio.src !== url) {
      stopChannelAudio(kind);
      audio.src = url;
    }
    audio.loop = kind === 'call';
    audio.volume = clampVolume(ch.volume);
    return audio;
  }

  function applyConfig(next) {
    if (!next || typeof next !== 'object') return;
    for (const kind of ['message', 'call']) {
      if (!next[kind] || typeof next[kind] !== 'object') continue;
      const incoming = next[kind];
      const prevDataUrl = config[kind]?.dataUrl;
      config[kind] = {
        mode: incoming.mode || 'default',
        preset: incoming.preset || (kind === 'call' ? 'classic' : 'chime'),
        dataUrl: incoming.dataUrl || '',
        volume: incoming.volume ?? 100,
      };
      if (prevDataUrl && prevDataUrl !== config[kind].dataUrl && customUrlCache.has(prevDataUrl)) {
        const stillUsed = config.message.dataUrl === prevDataUrl || config.call.dataUrl === prevDataUrl;
        if (!stillUsed) {
          URL.revokeObjectURL?.(customUrlCache.get(prevDataUrl));
          customUrlCache.delete(prevDataUrl);
        }
      }
      if (config[kind].mode === 'default' || config[kind].mode === 'mute') {
        stopChannelAudio(kind);
        channelAudios[kind] = null;
      } else {
        try {
          ensureChannelAudio(kind);
        } catch (err) {
          console.error('[Workspace Sound] invalid custom sound:', err);
          channelAudios[kind] = null;
        }
      }
    }
  }

  const originalPlay = HTMLMediaElement.prototype.play;
  const originalPause = HTMLMediaElement.prototype.pause;

  function playWrapper(...args) {
    try {
      const sound = classify(this);
      if (sound && sound.kind) {
        const ch = config[sound.kind];
        if (ch?.mode === 'mute') {
          return Promise.resolve();
        }
        if (ch && ch.mode !== 'default') {
          const custom = ensureChannelAudio(sound.kind);
          if (custom) {
            custom.currentTime = 0;
            custom.loop = sound.kind === 'call';
            custom.volume = clampVolume(ch.volume);
            if (sound.kind === 'call') {
              activeCustom.set(this, custom);
            }
            return originalPlay.call(custom).catch(() => {
              if (sound.kind === 'call') activeCustom.delete(this);
              return originalPlay.apply(this, args);
            });
          }
        }
      } else if (sound && sound.url) {
        console.debug('[Workspace Sound] unmatched WhatsApp audio:', sound.url);
      }
    } catch {
      // Fall back to WhatsApp's original audio playback.
    }
    return originalPlay.apply(this, args);
  }

  function pauseWrapper(...args) {
    const custom = activeCustom.get(this);
    if (custom) {
      try {
        originalPause.call(custom);
        custom.currentTime = 0;
      } catch {}
      activeCustom.delete(this);
    }
    return originalPause.apply(this, args);
  }

  Object.defineProperty(playWrapper, '__workspaceSoundHooked', { value: true });
  HTMLMediaElement.prototype.play = playWrapper;
  HTMLMediaElement.prototype.pause = pauseWrapper;

  window.__workspaceApplyNotificationSounds = applyConfig;
  window.__workspaceStopNotificationSounds = stopAllCustomSounds;

  if (window.__workspaceNotificationSounds) {
    applyConfig(window.__workspaceNotificationSounds);
  }
})();
