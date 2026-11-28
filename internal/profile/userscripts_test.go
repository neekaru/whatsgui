package profile

import (
	"path/filepath"
	"testing"
)

func TestParseUserscriptMetadata(t *testing.T) {
	code := `// ==UserScript==
// @name         Chat Enhancer
// @namespace    example.ns
// @version      1.2.0
// @description  Enhance WhatsApp UI
// @author       Tester
// @run-at       document-start
// @require      ./lib/helper.js
// @grant        GM_getValue
// @grant        GM_xmlhttpRequest
// @match        https://web.whatsapp.com/*
// @match        *://*.whatsapp.com/*
// ==/UserScript==
console.log("hi");`

	meta := ParseUserscriptMetadata("fallback.user.js", code)
	if meta.Name != "Chat Enhancer" || meta.Namespace != "example.ns" || meta.Version != "1.2.0" || meta.Author != "Tester" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if meta.Description != "Enhance WhatsApp UI" || meta.RunAt != RunAtDocumentStart {
		t.Fatalf("unexpected description/runAt: %+v", meta)
	}
	if len(meta.Matches) != 2 || len(meta.Requires) != 1 || len(meta.Grants) != 2 {
		t.Fatalf("unexpected slices: %+v", meta)
	}

	plain := ParseUserscriptMetadata("custom-helper.user.js", "console.log(1);")
	if plain.Name != "custom-helper" || plain.RunAt != RunAtDocumentIdle {
		t.Fatalf("fallback = %+v", plain)
	}
}

func TestGMStorageLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "userscript-storage.json")
	st := newGMStorage(path)
	if err := st.set("demo.user.js", "count", "42"); err != nil {
		t.Fatalf("set: %v", err)
	}
	reloaded := newGMStorage(path)
	if got := string(reloaded.data["demo.user.js"]["count"]); got != "42" {
		t.Fatalf("reloaded value = %q, want 42", got)
	}
	if err := reloaded.delete("demo.user.js", "count"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(reloaded.data) != 0 {
		t.Fatalf("expected empty storage after delete, got %+v", reloaded.data)
	}
	if err := st.set("../escape.user.js", "k", "1"); err == nil {
		t.Fatal("expected invalid scriptID error")
	}
}

func TestMatchesWhatsApp(t *testing.T) {
	cases := []struct {
		matches []string
		want    bool
	}{
		{nil, true},
		{[]string{"https://web.whatsapp.com/*"}, true},
		{[]string{"*://web.whatsapp.com/*"}, true},
		{[]string{"https://*.whatsapp.com/*"}, true},
		{[]string{"*://*/*"}, true},
		{[]string{"<all_urls>"}, true},
		{[]string{"*web.whatsapp.com*"}, true},
		{[]string{"http://web.whatsapp.com/*"}, false},
		{[]string{"https://example.com/*"}, false},
		{[]string{"https://example.com/*", "https://web.whatsapp.com/*"}, true},
	}
	for _, tc := range cases {
		if got := MatchesWhatsApp(tc.matches); got != tc.want {
			t.Errorf("MatchesWhatsApp(%v) = %v, want %v", tc.matches, got, tc.want)
		}
	}
}
