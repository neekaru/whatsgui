package workspace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"whatsappgui-new/internal/profile"
)

func TestNotificationSoundStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	store := newNotificationSoundStore(filepath.Join(dir, "notification-sounds.json"), filepath.Join(dir, "sounds"))

	got, err := store.get()
	if err != nil {
		t.Fatalf("default get failed: %v", err)
	}
	if got.Message.Mode != profile.SoundModeDefault || got.Call.Mode != profile.SoundModeDefault {
		t.Fatalf("unexpected defaults: %+v", got)
	}

	srcDir := t.TempDir()
	mp3Path := filepath.Join(srcDir, "ding.mp3")
	if err := os.WriteFile(mp3Path, []byte("fake-mp3-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	updated, err := store.importFile(profile.SoundKindMessage, mp3Path)
	if err != nil {
		t.Fatalf("importFile message failed: %v", err)
	}
	if updated.Message.Mode != profile.SoundModeCustom || updated.Message.FileName != "ding.mp3" || updated.Message.MimeType != "audio/mpeg" {
		t.Fatalf("unexpected imported message choice: %+v", updated.Message)
	}

	preview, err := store.customDataURL(profile.SoundKindMessage)
	if err != nil || !strings.HasPrefix(preview, "data:audio/mpeg;base64,") {
		t.Fatalf("customDataURL = %q, %v", preview, err)
	}

	updated.Message.Volume = 75
	updated.Call.Mode = profile.SoundModePreset
	updated.Call.Preset = "marimba"
	updated.Call.Volume = 60
	if _, err := store.set(updated); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	reloaded := newNotificationSoundStore(filepath.Join(dir, "notification-sounds.json"), filepath.Join(dir, "sounds"))
	payload := reloaded.payload()
	if payload.Message.Mode != profile.SoundModeCustom || payload.Message.Volume != 75 || !strings.HasPrefix(payload.Message.DataURL, "data:audio/mpeg;base64,") {
		t.Fatalf("reloaded message payload = %+v", payload.Message)
	}
	if payload.Call.Mode != profile.SoundModePreset || payload.Call.Preset != "marimba" || payload.Call.Volume != 60 {
		t.Fatalf("reloaded call payload = %+v", payload.Call)
	}

	cleared, err := reloaded.clearCustom(profile.SoundKindMessage)
	if err != nil {
		t.Fatalf("clearCustom failed: %v", err)
	}
	if cleared.Message.Mode != profile.SoundModeDefault || cleared.Message.FileName != "" {
		t.Fatalf("message not cleared: %+v", cleared.Message)
	}
	if dataURL, err := reloaded.customDataURL(profile.SoundKindMessage); err != nil || dataURL != "" {
		t.Fatalf("cleared preview still present: %q, %v", dataURL, err)
	}
}

func TestNotificationSoundValidation(t *testing.T) {
	dir := t.TempDir()
	store := newNotificationSoundStore(filepath.Join(dir, "notification-sounds.json"), filepath.Join(dir, "sounds"))

	badExt := filepath.Join(t.TempDir(), "not-audio.txt")
	if err := os.WriteFile(badExt, []byte("text"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.importFile(profile.SoundKindMessage, badExt); err == nil {
		t.Fatal("accepted unsupported extension")
	}

	tooBig := filepath.Join(t.TempDir(), "huge.mp3")
	if err := os.WriteFile(tooBig, bytes.Repeat([]byte("A"), profile.MaxNotificationSoundBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.importFile(profile.SoundKindCall, tooBig); err == nil {
		t.Fatal("accepted oversized audio file")
	}
}
