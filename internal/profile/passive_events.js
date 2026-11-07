(() => {
  if (location.origin !== 'https://web.whatsapp.com') return;

  const native = EventTarget.prototype.addEventListener;
  // Idempotent: this preload may be loaded by more than one document/iframe.
  if (native.__workspaceNonPassiveReact) return;

  // React delegates these three to its root container with `passive: true`, so
  // a component's onWheel/onTouchStart/onTouchMove cannot cancel the default
  // (Chrome warns "Unable to preventDefault inside passive event listener").
  const install = function (type, listener, options) {
    if (
      (type === 'wheel' || type === 'touchstart' || type === 'touchmove') &&
      options?.passive === true &&
      // React marks both application and portal containers before registration.
      Object.keys(this).some(key => key.startsWith('_reactListening') || key.startsWith('__reactContainer$'))
    ) {
      // Preserve inherited capture/once/signal options as well as listener identity.
      options = Object.create(options, { passive: { value: false } });
    }
    return native.call(this, type, listener, options);
  };

  Object.defineProperty(install, '__workspaceNonPassiveReact', { value: true });

  EventTarget.prototype.addEventListener = install;
})();