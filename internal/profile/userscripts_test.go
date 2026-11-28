package profile

import "testing"

func TestParseUserscriptMetadata(t *testing.T) {
	code := `// ==UserScript==
// @name         Chat Enhancer
// @version      1.2.0
// @description  Enhance WhatsApp UI
// @author       Tester
// @match        https://web.whatsapp.com/*
// @match        *://*.whatsapp.com/*
// ==/UserScript==
console.log("hi");`

	meta := ParseUserscriptMetadata("fallback.user.js", code)
	if meta.Name != "Chat Enhancer" || meta.Version != "1.2.0" || meta.Author != "Tester" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
	if meta.Description != "Enhance WhatsApp UI" {
		t.Fatalf("unexpected description: %q", meta.Description)
	}
	if len(meta.Matches) != 2 {
		t.Fatalf("matches = %v", meta.Matches)
	}

	plain := ParseUserscriptMetadata("custom-helper.user.js", "console.log(1);")
	if plain.Name != "custom-helper" {
		t.Fatalf("fallback name = %q, want custom-helper", plain.Name)
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
