package profile

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"testing"
)

func notificationPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestNotificationPhotoFitsProfilePipe(t *testing.T) {
	picture := notificationPNG(t, 128, 128)
	picture = append(picture, make([]byte, (24<<10)-len(picture))...)
	var pipe bytes.Buffer
	bridge := &ProfileBridge{conn: newConnection(strings.NewReader(""), &pipe)}
	// JSON expands these to six bytes each: use the worst valid text budget.
	if err := bridge.Notify(strings.Repeat("\x00", 256), strings.Repeat("\x00", 4096), strings.Repeat("\x00", 128), true, base64.StdEncoding.EncodeToString(picture)); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Dismiss("later"); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(&pipe)
	if !scanner.Scan() {
		t.Fatalf("photo notification overflowed the profile pipe: %v", scanner.Err())
	}
	var msg Message
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(msg.Image))
	if err != nil || config.Width != 128 || config.Height != 128 || !msg.Silent {
		t.Fatalf("photo or notification options lost in transport: %+v, %v", config, err)
	}
	if !scanner.Scan() || json.Unmarshal(scanner.Bytes(), &msg) != nil || msg.Kind != "dismiss" {
		t.Fatalf("photo broke later profile messages: %v", scanner.Err())
	}
}

func TestInvalidNotificationPhotoKeepsText(t *testing.T) {
	for name, encoded := range map[string]string{
		"absent":               "",
		"invalid base64":       "not base64!",
		"non image":            base64.StdEncoding.EncodeToString([]byte("not a PNG")),
		"oversized":            strings.Repeat("A", (32<<10)+1),
		"oversized dimensions": base64.StdEncoding.EncodeToString(notificationPNG(t, 129, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			var pipe bytes.Buffer
			bridge := &ProfileBridge{conn: newConnection(strings.NewReader(""), &pipe)}
			if err := bridge.Notify("Contact", "New message", "token", false, encoded); err != nil {
				t.Fatal(err)
			}
			var msg Message
			if err := json.Unmarshal(pipe.Bytes(), &msg); err != nil {
				t.Fatal(err)
			}
			if msg.Kind != "notify" || msg.Body != "New message" || len(msg.Image) != 0 {
				t.Fatalf("invalid photo suppressed or contaminated text delivery: %+v", msg)
			}
		})
	}
}
