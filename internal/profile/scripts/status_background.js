(() => {
  if (location.origin !== 'https://web.whatsapp.com' || window !== window.top) return;

  const statusVideo = video => video instanceof HTMLVideoElement &&
    video.closest('[data-animate-status-viewer], [data-testid="status-video"]');
  let pending = new Set();
  let expiry;

  const rememberPlaying = () => {
    clearTimeout(expiry);
    for (const video of document.querySelectorAll('video')) {
      if (statusVideo(video) && !video.paused && !video.ended) pending.add(video);
    }
    // ponytail: only the immediate background pause is resumed; not a later user pause.
    expiry = setTimeout(() => pending.clear(), 1500);
  };
  window.addEventListener('blur', rememberPlaying);
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) rememberPlaying();
  });
  window.addEventListener('focus', () => {
    clearTimeout(expiry);
    pending.clear();
  });
  document.addEventListener('pause', event => {
    const video = event.target;
    if (!pending.delete(video)) return;
    setTimeout(() => {
      if (!document.hasFocus() && video.isConnected && statusVideo(video) && video.paused && !video.ended) {
        video.play().catch(error => console.warn('Tidak dapat melanjutkan status di background:', error));
      }
    }, 0);
  }, true);
})();
