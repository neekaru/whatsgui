(() => {
  if (location.origin !== 'https://web.whatsapp.com' || window !== window.top) return;
  const notifications = new Map();
  let sequence = 0;
  // Keep PNG bytes <= 24 KiB, so even escaped text fits the 64 KiB IPC line.
  const notificationImage = (source) => new Promise((resolve) => {
    if (!source || source.length > 512 * 1024) return resolve('');
    let url;
    try { url = new URL(source, location.href); } catch { return resolve(''); }
    if (url.username || url.password ||
        !['blob:', 'data:', 'https:'].includes(url.protocol) ||
        (url.protocol === 'blob:' && url.origin !== location.origin) ||
        (url.protocol === 'data:' && !/^data:image\/(?:png|jpeg|webp);base64,/i.test(url.href))) return resolve('');
    const image = new Image();
    const finish = (data = '') => {
      clearTimeout(timer);
      image.onload = image.onerror = null;
      image.src = '';
      resolve(data);
    };
    const timer = setTimeout(() => finish(), 1500);
    image.crossOrigin = 'anonymous';
    image.referrerPolicy = 'no-referrer';
    image.onerror = () => finish();
    image.onload = () => {
      try {
        if (!image.naturalWidth || !image.naturalHeight) return finish();
        const canvas = document.createElement('canvas');
        for (const size of [128, 64]) {
          canvas.width = canvas.height = size;
          const scale = Math.min(size / image.naturalWidth, size / image.naturalHeight);
          const width = image.naturalWidth * scale;
          const height = image.naturalHeight * scale;
          canvas.getContext('2d').drawImage(image, (size - width) / 2, (size - height) / 2, width, height);
          const data = canvas.toDataURL('image/png').split(',')[1];
          if (data && data.length <= 32 * 1024) return finish(data);
        }
      } catch {
        // CSP/CORS, a revoked blob or a failed decode must not hide the text.
      }
      finish();
    };
    image.src = url.href;
  });
  class DesktopNotification extends EventTarget {
    static get permission() { return 'granted'; }
    static get maxActions() { return 0; }
    static requestPermission(callback) {
      callback?.('granted');
      return Promise.resolve('granted');
    }
    constructor(title, options = {}) {
      super();
      this.title = String(title);
      this.body = String(options.body ?? '');
      this.tag = String(options.tag ?? '');
      this.icon = String(options.icon ?? '');
      this.data = options.data ?? null;
      this.silent = Boolean(options.silent);
      this.onclick = null;
      this.onclose = null;
      this.onerror = null;
      this.onshow = null;
      this.token = `${Date.now()}-${++sequence}`;
      notifications.set(this.token, this);
      this.ready = notificationImage(this.icon)
        .then((image) => {
          if (notifications.has(this.token)) {
            return window.mygo.call('ProfileBridge.Notify', this.title, this.body, this.token, this.silent, image);
          }
        })
        .then(() => { if (notifications.has(this.token)) this.emit('show'); });
      this.ready.catch((error) => {
        notifications.delete(this.token);
        console.error('Notifikasi Windows gagal', error);
        this.emit('error');
      });
    }
    emit(type) {
      const event = new Event(type);
      this.dispatchEvent(event);
      this[`on${type}`]?.call(this, event);
    }
    close() {
      notifications.delete(this.token);
      window.mygo.call('ProfileBridge.Dismiss', this.token).catch(console.error);
      this.emit('close');
    }
  }
  Object.defineProperty(window, 'Notification', { configurable: true, writable: true, value: DesktopNotification });
  // WhatsApp versions that request notifications through a service-worker registration
  // still use the same native desktop channel while this app is running.
  if (typeof ServiceWorkerRegistration !== 'undefined') {
    ServiceWorkerRegistration.prototype.showNotification = function(title, options) {
      return new DesktopNotification(title, options).ready;
    };
  }
  window.__workspaceNotificationClick = (token) => {
    const notification = notifications.get(token);
    if (notification) { notification.emit('click'); notification.close(); }
  };
})();
