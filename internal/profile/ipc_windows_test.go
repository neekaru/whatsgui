package profile

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestProfileProtocolAndOrigins(t *testing.T) {
	conn := newConnection(strings.NewReader("{\"action\":\"start\"}\n{\"action\":\"reload\"}\n{\"action\":\"quit\"}\n"), &bytes.Buffer{})
	for _, action := range []string{"start", "reload", "quit"} {
		cmd, err := conn.receive()
		if err != nil || cmd.Action != action {
			t.Fatalf("perintah terbuffer hilang: %+v %v", cmd, err)
		}
	}
	if _, err := conn.receive(); !errors.Is(err, io.EOF) {
		t.Fatalf("EOF: %v", err)
	}
	for _, input := range []string{"not-json\n", "{\"action\":\"execute\"}\n", "{\"action\":\"click\",\"token\":\"" + strings.Repeat("x", 129) + "\"}\n"} {
		if _, err := newConnection(strings.NewReader(input), io.Discard).receive(); err == nil {
			t.Fatalf("input tidak valid diterima: %q", input)
		}
	}
	for raw, allowed := range map[string]bool{
		"https://web.whatsapp.com/":               true,
		"https://web.whatsapp.com/?app=desktop":   true,
		"https://web.whatsapp.com.attacker.test/": false,
		"https://web.whatsapp.com@attacker.test/": false,
		"http://web.whatsapp.com/":                false,
		"javascript:alert(1)":                     false,
	} {
		if got := isWhatsApp(raw); got != allowed {
			t.Fatalf("origin %q = %v, ingin %v", raw, got, allowed)
		}
	}
}
