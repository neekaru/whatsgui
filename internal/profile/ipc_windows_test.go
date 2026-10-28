package profile

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

func TestProfileProtocolAndOrigins(t *testing.T) {
	conn := newConnection(strings.NewReader("{\"action\":\"start\"}\n{\"action\":\"reload\"}\n{\"action\":\"devtools\"}\n{\"action\":\"quit\"}\n"), &bytes.Buffer{})
	for _, action := range []string{"start", "reload", "devtools", "quit"} {
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
		"https://web.whatsapp.com/":                true,
		"https://web.whatsapp.com/?app=desktop":    true,
		"https://web.whatsapp.com/call/popout":     true,
		"https://web.whatsapp.com/call/popout?x=1": true,
		"https://web.whatsapp.com.attacker.test/":  false,
		"https://web.whatsapp.com@attacker.test/":  false,
		"http://web.whatsapp.com/":                 false,
		"javascript:alert(1)":                      false,
	} {
		if got := isWhatsApp(raw); got != allowed {
			t.Fatalf("origin %q = %v, ingin %v", raw, got, allowed)
		}
	}
}

func TestMediaPermissionsAreLimitedToWhatsApp(t *testing.T) {
	for _, permissions := range [][]mygo.Permission{
		{mygo.PermissionMicrophone},
		{mygo.PermissionCamera},
		{mygo.PermissionMicrophone, mygo.PermissionCamera},
	} {
		request := mygo.PermissionRequest{Origin: whatsappOrigin, Permissions: permissions}
		if !requestPermission(request) {
			t.Fatalf("WhatsApp media permissions denied: %v", permissions)
		}
		request.Origin = "https://example.com"
		if requestPermission(request) {
			t.Fatalf("media permissions granted to a different origin: %v", permissions)
		}
		request.Origin = whatsappOrigin
		request.Permissions = append(append([]mygo.Permission(nil), permissions...), mygo.PermissionGeolocation)
		if requestPermission(request) {
			t.Fatalf("unrelated permission granted with %v", permissions)
		}
	}
}

func TestMediaCommands(t *testing.T) {
	for _, input := range []string{
		"{\"action\":\"setmedia\",\"audioId\":\"" + strings.Repeat("x", 257) + "\"}\n",
		"{\"action\":\"media\",\"id\":\"" + strings.Repeat("x", 65) + "\"}\n",
	} {
		if _, err := newConnection(strings.NewReader(input), io.Discard).receive(); err == nil {
			t.Fatalf("input media tidak valid diterima: %q", input)
		}
	}
}
