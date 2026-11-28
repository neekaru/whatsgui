(() => {
  if (location.origin !== 'https://web.whatsapp.com' || window !== window.top) return;
  if (window.__workspacePreloadGuardInstalled) return;
  window.__workspacePreloadGuardInstalled = true;

  const isWhatsAppStaticAsset = (rawHref) => {
    if (!rawHref) return false;
    try {
      const url = new URL(String(rawHref), location.href);
      return /(^|\.)whatsapp\.net$/i.test(url.hostname) && url.pathname.includes('/rsrc.php');
    } catch {
      return false;
    }
  };

  // Convert redundant WhatsApp static rsrc.php link preloads into prefetch before
  // insertion so Chromium does not spam unused-preload console warnings.
  const normalizePreloadLink = (node) => {
    if (!node || node.nodeType !== 1 || node.tagName !== 'LINK') return;
    const rel = typeof node.getAttribute === 'function'
      ? node.getAttribute('rel')
      : node.rel;
    if (!rel || !/\bpreload\b/i.test(rel)) return;
    const href = typeof node.getAttribute === 'function'
      ? (node.getAttribute('href') || node.href)
      : node.href;
    if (!isWhatsAppStaticAsset(href)) return;
    const nextRel = rel.replace(/\bpreload\b/gi, 'prefetch').trim();
    if (typeof node.setAttribute === 'function') {
      nativeSetAttribute.call(node, 'rel', nextRel || 'prefetch');
    } else {
      node.rel = nextRel || 'prefetch';
    }
  };

  const nativeSetAttribute = Element.prototype.setAttribute;
  Element.prototype.setAttribute = function (name, value) {
    if (
      this.tagName === 'LINK' &&
      typeof name === 'string' &&
      name.toLowerCase() === 'rel' &&
      typeof value === 'string' &&
      /\bpreload\b/i.test(value)
    ) {
      const href = this.getAttribute?.('href') || this.href;
      if (isWhatsAppStaticAsset(href)) {
        value = value.replace(/\bpreload\b/gi, 'prefetch').trim() || 'prefetch';
      }
    } else if (
      this.tagName === 'LINK' &&
      typeof name === 'string' &&
      name.toLowerCase() === 'href' &&
      isWhatsAppStaticAsset(value)
    ) {
      const rel = this.getAttribute?.('rel') || this.rel;
      if (rel && /\bpreload\b/i.test(rel)) {
        nativeSetAttribute.call(this, 'rel', rel.replace(/\bpreload\b/gi, 'prefetch').trim() || 'prefetch');
      }
    }
    return nativeSetAttribute.call(this, name, value);
  };

  if (typeof HTMLLinkElement !== 'undefined' && HTMLLinkElement.prototype) {
    const relDesc = Object.getOwnPropertyDescriptor(HTMLLinkElement.prototype, 'rel');
    if (relDesc?.set && relDesc.configurable) {
      Object.defineProperty(HTMLLinkElement.prototype, 'rel', {
        configurable: true,
        enumerable: relDesc.enumerable,
        get: relDesc.get,
        set(value) {
          if (typeof value === 'string' && /\bpreload\b/i.test(value) && isWhatsAppStaticAsset(this.href)) {
            value = value.replace(/\bpreload\b/gi, 'prefetch').trim() || 'prefetch';
          }
          return relDesc.set.call(this, value);
        },
      });
    }
    const hrefDesc = Object.getOwnPropertyDescriptor(HTMLLinkElement.prototype, 'href');
    if (hrefDesc?.set && hrefDesc.configurable) {
      Object.defineProperty(HTMLLinkElement.prototype, 'href', {
        configurable: true,
        enumerable: hrefDesc.enumerable,
        get: hrefDesc.get,
        set(value) {
          const res = hrefDesc.set.call(this, value);
          if (isWhatsAppStaticAsset(value)) normalizePreloadLink(this);
          return res;
        },
      });
    }
  }

  for (const method of ['appendChild', 'insertBefore', 'replaceChild']) {
    const nativeMethod = Node.prototype[method];
    if (typeof nativeMethod !== 'function') continue;
    Node.prototype[method] = function (node, ...rest) {
      normalizePreloadLink(node);
      return nativeMethod.call(this, node, ...rest);
    };
  }

  if (typeof Element !== 'undefined' && Element.prototype) {
    for (const method of ['append', 'prepend']) {
      const nativeMethod = Element.prototype[method];
      if (typeof nativeMethod !== 'function') continue;
      Element.prototype[method] = function (...nodes) {
        for (const node of nodes) normalizePreloadLink(node);
        return nativeMethod.apply(this, nodes);
      };
    }
  }

  if (typeof MutationObserver === 'function' && document) {
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        for (const node of mutation.addedNodes || []) {
          normalizePreloadLink(node);
          if (node?.querySelectorAll) {
            for (const link of node.querySelectorAll('link[rel~="preload"]')) {
              normalizePreloadLink(link);
            }
          }
        }
      }
    });
    const root = document.documentElement || document;
    if (root) {
      observer.observe(root, { childList: true, subtree: true });
    }
  }
})();
