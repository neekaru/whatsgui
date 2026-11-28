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
		"https://web.whatsapp.com/":                           true,
		"https://web.whatsapp.com/?app=desktop":               true,
		"https://web.whatsapp.com/call/popout":                true,
		"https://web.whatsapp.com/call/popout?x=1":            true,
		"https://webtp.whatsapp.net/pdf-viewer/?locale=id_ID": true,
		"https://flows.whatsapp.net/flows/cache_management/":  true,
		"https://WEB.WHATSAPP.COM:443/":                       true,
		"https://webtp.whatsapp.net:443/pdf-viewer/":          true,
		"https://webtp.whatsapp.net:444/pdf-viewer/":          false,
		"https://webtp.whatsapp.net.evil.test/pdf-viewer/":    false,
		"https://user@webtp.whatsapp.net/pdf-viewer/":         false,
		"http://webtp.whatsapp.net/pdf-viewer/":               false,
		"https://web.whatsapp.com.attacker.test/":             false,
		"https://web.whatsapp.com@attacker.test/":             false,
		"http://web.whatsapp.com/":                            false,
		"javascript:alert(1)":                                 false,
	} {
		if got := isAppNavigation(raw); got != allowed {
			t.Fatalf("origin %q = %v, ingin %v", raw, got, allowed)
		}
	}
	if isWhatsApp("https://webtp.whatsapp.net/pdf-viewer/") || !isWhatsApp("https://WEB.WHATSAPP.COM:443/") {
		t.Fatal("main tab navigation does not preserve the primary WhatsApp origin")
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
		request.Origin = "https://webtp.whatsapp.net"
		if requestPermission(request) {
			t.Fatalf("media permissions granted to auxiliary PDF viewer: %v", permissions)
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
		"{\"action\":\"downloadreply\"}\n",
		"{\"action\":\"downloadreply\",\"id\":\"abc\",\"name\":\"" + strings.Repeat("x", 257) + "\"}\n",
	} {
		if _, err := newConnection(strings.NewReader(input), io.Discard).receive(); err == nil {
			t.Fatalf("input media tidak valid diterima: %q", input)
		}
	}
	cmd, err := newConnection(strings.NewReader("{\"action\":\"downloadreply\",\"id\":\"dl1\",\"accept\":true,\"name\":\"baru.pdf\"}\n"), io.Discard).receive()
	if err != nil || cmd.Action != "downloadreply" || !cmd.Accept || cmd.Name != "baru.pdf" {
		t.Fatalf("downloadreply valid ditolak: %+v %v", cmd, err)
	}

	largeCode := strings.Repeat("a", 80*1024)
	cmd, err = newConnection(strings.NewReader(`{"action":"userscripts","userscripts":[{"id":"big.user.js","name":"Big","code":"`+largeCode+`"}]}`+"\n"), io.Discard).receive()
	if err != nil || cmd.Action != "userscripts" || len(cmd.Userscripts) != 1 {
		t.Fatalf("userscripts payload besar ditolak: %+v %v", cmd, err)
	}

	if _, err := newConnection(strings.NewReader("{\"action\":\"sounds\"}\n"), io.Discard).receive(); err == nil {
		t.Fatal("sounds tanpa payload diterima")
	}
	cmd, err = newConnection(strings.NewReader(`{"action":"sounds","sounds":{"message":{"mode":"preset","preset":"chime","volume":90},"call":{"mode":"mute","volume":100}}}`+"\n"), io.Discard).receive()
	if err != nil || cmd.Action != "sounds" || cmd.Sounds == nil || cmd.Sounds.Message.Preset != "chime" || cmd.Sounds.Call.Mode != SoundModeMute {
		t.Fatalf("perintah sounds valid ditolak: %+v %v", cmd, err)
	}
}
