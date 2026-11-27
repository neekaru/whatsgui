(() => {
  if (location.origin !== 'https://web.whatsapp.com' || window !== window.top) return;
  const media = navigator.mediaDevices;
  if (!media || typeof media.getUserMedia !== 'function') return;

  // localStorage is profile-scoped and shared with popups, including their
  // first document where WebView2 cannot deliver MyGo bridge replies.
  const key = '__workspace_media_devices_v1';
  const selection = () => JSON.parse(localStorage.getItem(key) || '{"audio":"","video":""}');
  window.__workspaceSetMediaDevices = (audio, video) => {
    localStorage.setItem(key, JSON.stringify({ audio, video }));
    return true;
  };

  // Chromium treats a bare deviceId as a preference and may open another
  // device, so the selected one is always exact.
  const constrain = (value, id) => {
    if (!id || value === undefined || value === null || value === false) return null;
    if (value === true) return { deviceId: { exact: id } };
    if (typeof value !== 'object') return null;
    return { ...value, deviceId: { exact: id } };
  };

  const original = media.getUserMedia.bind(media);
  media.getUserMedia = async (constraints) => {
    if (!constraints || typeof constraints !== 'object') return original(constraints);
    const chosen = selection();
    const request = { ...constraints };
    const audio = constrain(request.audio, chosen.audio);
    const video = constrain(request.video, chosen.video);
    if (!audio && !video) return original(constraints);
    if (audio) request.audio = audio;
    if (video) request.video = video;
    return original(request);
  };

  window.__workspaceMediaDevices = async () => {
    let list = Array.from(await media.enumerateDevices());
    // Request only kinds whose labels are hidden, then stop the probe.
    for (const kind of ['audio', 'video']) {
      const devices = list.filter((device) => device.kind === `${kind}input`);
      if (devices.length && devices.every((device) => !device.label)) {
        try {
          const stream = await original({ [kind]: true });
          stream.getTracks().forEach((track) => track.stop());
        } catch {}
      }
    }
    list = Array.from(await media.enumerateDevices());
    const audio = [];
    const video = [];
    for (const device of list) {
      if (!device.deviceId) continue;
      if (device.kind === 'audioinput') audio.push({ id: device.deviceId, label: device.label });
      else if (device.kind === 'videoinput') video.push({ id: device.deviceId, label: device.label });
    }
    const chosen = selection();
    return { audio, video, audioId: chosen.audio || '', videoId: chosen.video || '' };
  };
})();
