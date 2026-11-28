package profile

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	maxGMKeyLen        = 256
	maxGMKeysPerScript = 500
	maxGMValueBytes    = 512 << 10
	maxGMRequestBytes  = 2 << 20
	maxGMResponseBytes = 10 << 20
)

// gmStorage persists per-userscript key-value pairs as JSON inside the profile root.
type gmStorage struct {
	mu   sync.Mutex
	file string
	data map[string]map[string]json.RawMessage
}

func newGMStorage(file string) *gmStorage {
	s := &gmStorage{
		file: file,
		data: map[string]map[string]json.RawMessage{},
	}
	raw, err := os.ReadFile(file)
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		var loaded map[string]map[string]json.RawMessage
		if json.Unmarshal(raw, &loaded) == nil && loaded != nil {
			s.data = loaded
		}
	}
	return s
}

func (s *gmStorage) snapshotJSON() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(s.data)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func (s *gmStorage) set(scriptID, key, jsonValue string) error {
	if err := validGMTarget(scriptID, key); err != nil {
		return err
	}
	if len(jsonValue) > maxGMValueBytes || !json.Valid([]byte(jsonValue)) {
		return errors.New("nilai penyimpanan userscript tidak valid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.data[scriptID]
	if bucket == nil {
		bucket = map[string]json.RawMessage{}
		s.data[scriptID] = bucket
	}
	if _, exists := bucket[key]; !exists && len(bucket) >= maxGMKeysPerScript {
		return errors.New("batas kunci penyimpanan userscript tercapai")
	}
	bucket[key] = json.RawMessage(jsonValue)
	return s.saveLocked()
}

func (s *gmStorage) delete(scriptID, key string) error {
	if err := validGMTarget(scriptID, key); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket := s.data[scriptID]
	if bucket == nil {
		return nil
	}
	if _, ok := bucket[key]; !ok {
		return nil
	}
	delete(bucket, key)
	if len(bucket) == 0 {
		delete(s.data, scriptID)
	}
	return s.saveLocked()
}

func (s *gmStorage) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.file), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.file), "gm-storage-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.file)
}

func validGMTarget(scriptID, key string) error {
	if scriptID == "" || len(scriptID) > 256 || strings.ContainsAny(scriptID, `/\:`+"\x00") {
		return errors.New("ID userscript tidak valid")
	}
	if key == "" || len(key) > maxGMKeyLen || strings.ContainsRune(key, 0) {
		return errors.New("kunci penyimpanan userscript tidak valid")
	}
	return nil
}

type gmHTTPRequest struct {
	Method       string            `json:"method"`
	URL          string            `json:"url"`
	Headers      map[string]string `json:"headers,omitempty"`
	Data         string            `json:"data,omitempty"`
	Timeout      int               `json:"timeout,omitempty"`
	ResponseType string            `json:"responseType,omitempty"`
}

type gmHTTPResponse struct {
	Status          int    `json:"status"`
	StatusText      string `json:"statusText"`
	FinalURL        string `json:"finalUrl"`
	ResponseHeaders string `json:"responseHeaders"`
	ResponseText    string `json:"responseText,omitempty"`
	ResponseBase64  string `json:"responseBase64,omitempty"`
}

var gmHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
}

func performGMFetch(parent context.Context, reqJSON string) (string, error) {
	if len(reqJSON) > maxGMRequestBytes+64*1024 {
		return "", errors.New("permintaan GM_xmlhttpRequest terlalu besar")
	}
	var spec gmHTTPRequest
	if err := json.Unmarshal([]byte(reqJSON), &spec); err != nil {
		return "", fmt.Errorf("permintaan GM_xmlhttpRequest tidak valid: %w", err)
	}
	u, err := url.Parse(strings.TrimSpace(spec.URL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("URL GM_xmlhttpRequest harus http:// atau https://")
	}
	method := strings.ToUpper(strings.TrimSpace(spec.Method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
	default:
		return "", fmt.Errorf("metode HTTP tidak didukung: %s", method)
	}
	if len(spec.Data) > maxGMRequestBytes {
		return "", errors.New("body GM_xmlhttpRequest melebihi batas 2 MB")
	}

	timeout := 15 * time.Second
	if spec.Timeout > 0 {
		timeout = min(time.Duration(spec.Timeout)*time.Millisecond, 60*time.Second)
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	var body io.Reader
	if spec.Data != "" {
		body = strings.NewReader(spec.Data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return "", err
	}
	for k, v := range spec.Headers {
		if k != "" && !strings.ContainsAny(k, "\r\n") && !strings.ContainsAny(v, "\r\n") {
			req.Header.Set(k, v)
		}
	}

	resp, err := gmHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	limited, err := io.ReadAll(io.LimitReader(resp.Body, maxGMResponseBytes+1))
	if err != nil {
		return "", err
	}
	if len(limited) > maxGMResponseBytes {
		return "", errors.New("respons GM_xmlhttpRequest melebihi batas 10 MB")
	}

	var hdrBuf strings.Builder
	for k, vals := range resp.Header {
		for _, v := range vals {
			hdrBuf.WriteString(strings.ToLower(k))
			hdrBuf.WriteString(": ")
			hdrBuf.WriteString(v)
			hdrBuf.WriteString("\r\n")
		}
	}

	out := gmHTTPResponse{
		Status:          resp.StatusCode,
		StatusText:      resp.Status,
		FinalURL:        resp.Request.URL.String(),
		ResponseHeaders: hdrBuf.String(),
	}
	rt := strings.ToLower(strings.TrimSpace(spec.ResponseType))
	if rt == "arraybuffer" || rt == "blob" || !utf8.Valid(limited) {
		out.ResponseBase64 = base64.StdEncoding.EncodeToString(limited)
	} else {
		out.ResponseText = string(limited)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// gmRuntimeScript provides GM_* and GM.* APIs inside the WhatsApp page,
// backed by ProfileBridge for persistent storage, HTTP requests, clipboard,
// notifications, and external tabs.
const gmRuntimeScript = `(() => {
  if (window.__workspaceMakeGM) return;
  const store = window.__workspaceGMStorage || {};

  function cloneVal(v) {
    return v === undefined ? undefined : JSON.parse(JSON.stringify(v));
  }

  function formatResponse(raw, responseType) {
    const out = {
      status: raw.status,
      statusText: raw.statusText,
      finalUrl: raw.finalUrl,
      responseHeaders: raw.responseHeaders,
      readyState: 4,
      responseText: raw.responseText || '',
      response: raw.responseText || ''
    };
    const rt = (responseType || '').toLowerCase();
    if (raw.responseBase64) {
      const bin = atob(raw.responseBase64);
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      if (rt === 'blob') out.response = new Blob([bytes]);
      else out.response = bytes.buffer;
    } else if (rt === 'json') {
      try { out.response = JSON.parse(out.responseText); } catch (_) { out.response = null; }
    }
    return out;
  }

  window.__workspaceMakeGM = (meta) => {
    const scriptId = meta.id;
    const bucket = (store[scriptId] = store[scriptId] || {});

    const getValue = (key, defVal) => {
      if (!Object.prototype.hasOwnProperty.call(bucket, key)) return defVal;
      return cloneVal(bucket[key]);
    };
    const setValue = (key, val) => {
      const encoded = JSON.stringify(val === undefined ? null : val);
      bucket[key] = JSON.parse(encoded);
      void window.mygo.call('ProfileBridge.GMSetValue', scriptId, String(key), encoded).catch((e) => {
        console.error('[Userscript GM_setValue]', scriptId, e);
      });
    };
    const deleteValue = (key) => {
      delete bucket[key];
      void window.mygo.call('ProfileBridge.GMDeleteValue', scriptId, String(key)).catch((e) => {
        console.error('[Userscript GM_deleteValue]', scriptId, e);
      });
    };
    const listValues = () => Object.keys(bucket);

    const addStyle = (css) => {
      const el = document.createElement('style');
      el.textContent = String(css || '');
      (document.head || document.documentElement).appendChild(el);
      return el;
    };

    const setClipboard = (text) => {
      return window.mygo.call('ProfileBridge.GMSetClipboard', String(text ?? '')).catch((e) => {
        console.error('[Userscript GM_setClipboard]', e);
      });
    };

    const openInTab = (url) => {
      return window.mygo.call('ProfileBridge.GMOpenInTab', String(url ?? '')).catch((e) => {
        console.error('[Userscript GM_openInTab]', e);
      });
    };

    const notification = (detailsOrText, title, image, onclick) => {
      const opts = typeof detailsOrText === 'object' && detailsOrText !== null
        ? detailsOrText
        : { text: detailsOrText, title, image, onclick };
      const body = String(opts.text ?? '');
      const heading = String(opts.title || meta.name || 'Userscript');
      const silent = Boolean(opts.silent);
      void window.mygo.call('ProfileBridge.Notify', heading, body, '', silent, '').catch((e) => {
        console.error('[Userscript GM_notification]', e);
      });
    };

    const xmlhttpRequest = (details) => {
      const req = Object.assign({}, details || {});
      const payload = JSON.stringify({
        method: req.method || 'GET',
        url: req.url || '',
        headers: req.headers || {},
        data: typeof req.data === 'string' ? req.data : (req.data != null ? String(req.data) : ''),
        timeout: Number(req.timeout) || 0,
        responseType: req.responseType || ''
      });
      const promise = window.mygo.call('ProfileBridge.GMFetch', payload)
        .then((rawStr) => {
          const parsed = JSON.parse(rawStr);
          const res = formatResponse(parsed, req.responseType);
          if (typeof req.onload === 'function') req.onload(res);
          if (typeof req.onloadend === 'function') req.onloadend(res);
          return res;
        })
        .catch((err) => {
          const errObj = { error: String(err && err.message || err) };
          if (typeof req.onerror === 'function') req.onerror(errObj);
          if (typeof req.onloadend === 'function') req.onloadend(errObj);
          throw err;
        });
      return { abort: () => {}, then: promise.then.bind(promise), catch: promise.catch.bind(promise) };
    };

    const info = {
      scriptHandler: 'WhatsApp GUI Userscripts',
      version: '1.0.0',
      script: {
        name: meta.name || scriptId,
        namespace: meta.namespace || '',
        version: meta.version || '',
        description: meta.description || '',
        author: meta.author || '',
        matches: meta.matches || [],
        'run-at': meta.runAt || 'document-idle',
        grant: meta.grants || []
      }
    };

    return {
      GM_info: info,
      GM_getValue: getValue,
      GM_setValue: setValue,
      GM_deleteValue: deleteValue,
      GM_listValues: listValues,
      GM_addStyle: addStyle,
      GM_setClipboard: setClipboard,
      GM_openInTab: openInTab,
      GM_notification: notification,
      GM_xmlhttpRequest: xmlhttpRequest,
      GM: {
        info,
        getValue: async (k, d) => getValue(k, d),
        setValue: async (k, v) => { setValue(k, v); },
        deleteValue: async (k) => { deleteValue(k); },
        listValues: async () => listValues(),
        addStyle: async (css) => addStyle(css),
        setClipboard: (t) => setClipboard(t),
        openInTab: (u) => openInTab(u),
        notification: async (d, t, i, c) => notification(d, t, i, c),
        xmlHttpRequest: (details) => xmlhttpRequest(details)
      }
    };
  };
})();`
