const { test } = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');

const script = fs.readFileSync(path.join(__dirname, 'preload_guard.js'), 'utf8');

function setup() {
  const inserted = [];

  class Node {
    constructor(tagName) {
      this.nodeType = 1;
      this.tagName = tagName;
      this.attrs = new Map();
      this._rel = '';
      this._href = '';
      this.children = [];
    }
    setAttribute(name, value) {
      this.attrs.set(String(name).toLowerCase(), String(value));
      if (String(name).toLowerCase() === 'rel') this._rel = String(value);
      if (String(name).toLowerCase() === 'href') this._href = String(value);
    }
    getAttribute(name) {
      return this.attrs.get(String(name).toLowerCase()) ?? null;
    }
    appendChild(child) {
      this.children.push(child);
      inserted.push(child);
      return child;
    }
    insertBefore(child) {
      this.children.push(child);
      inserted.push(child);
      return child;
    }
  }

  class Element extends Node {
    append(...nodes) {
      for (const node of nodes) this.appendChild(node);
    }
    prepend(...nodes) {
      for (const node of nodes) this.appendChild(node);
    }
  }

  class HTMLLinkElement extends Element {
    constructor() {
      super('LINK');
    }
    get rel() {
      return this._rel;
    }
    set rel(v) {
      this._rel = String(v);
      this.attrs.set('rel', this._rel);
    }
    get href() {
      return this._href;
    }
    set href(v) {
      this._href = String(v);
      this.attrs.set('href', this._href);
    }
  }

  const head = new Element('HEAD');
  const window = {};
  window.top = window;

  vm.runInNewContext(script, {
    window,
    document: { documentElement: head, head },
    Node,
    Element,
    HTMLLinkElement,
    URL,
    location: { origin: 'https://web.whatsapp.com', href: 'https://web.whatsapp.com/' },
  });

  return { head, HTMLLinkElement, inserted };
}

test('rewrites static.whatsapp.net rsrc.php link preload to prefetch before insertion', () => {
  const { head, HTMLLinkElement, inserted } = setup();
  const link = new HTMLLinkElement();
  link.rel = 'preload';
  link.href = 'https://static.whatsapp.net/rsrc.php/v4/yl/r/chunk.js';
  head.appendChild(link);

  assert.equal(inserted.length, 1);
  assert.equal(link.rel, 'prefetch');
  assert.equal(link.getAttribute('rel'), 'prefetch');
});

test('rewrites setAttribute rel=preload when href is set first or after', () => {
  const { head, HTMLLinkElement } = setup();

  const first = new HTMLLinkElement();
  first.setAttribute('href', 'https://static.whatsapp.net/rsrc.php/v4/yl/r/a.js');
  first.setAttribute('rel', 'preload');
  assert.equal(first.getAttribute('rel'), 'prefetch');

  const second = new HTMLLinkElement();
  second.setAttribute('rel', 'preload');
  second.setAttribute('href', 'https://static.whatsapp.net/rsrc.php/v4/yl/r/b.js');
  head.append(second);
  assert.equal(second.getAttribute('rel'), 'prefetch');
});

test('keeps stylesheets, icons, and non-rsrc preloads untouched', () => {
  const { head, HTMLLinkElement } = setup();

  const css = new HTMLLinkElement();
  css.rel = 'stylesheet';
  css.href = 'https://static.whatsapp.net/rsrc.php/v4/style.css';
  head.appendChild(css);
  assert.equal(css.rel, 'stylesheet');

  const localFont = new HTMLLinkElement();
  localFont.rel = 'preload';
  localFont.href = 'https://web.whatsapp.com/fonts/app.woff2';
  head.appendChild(localFont);
  assert.equal(localFont.rel, 'preload');
});
