(() => {
  if (location.origin !== 'https://web.whatsapp.com' || window !== window.top) return;
  const notifications = new Map();
  let sequence = 0;
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
      window.mygo.call('ProfileBridge.Notify', this.title, this.body, this.token, this.silent)
        .then(() => this.emit('show'))
        .catch((error) => { console.error('Notifikasi Windows gagal', error); this.emit('error'); });
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
      new DesktopNotification(title, options);
      return Promise.resolve();
    };
  }
  window.__workspaceNotificationClick = (token) => {
    const notification = notifications.get(token);
    if (notification) { notification.emit('click'); notification.close(); }
  };
})();
