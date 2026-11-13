package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateProfileMovesLegacyData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	appData := t.TempDir()
	w := &Workspace{file: filepath.Join(appData, "tabs.json")}
	id := newTab("x").ID

	legacy := legacyProfileDirectory(profileName(id))
	cookies := filepath.Join(legacy, "WebView2", "EBWebView", "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookies), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookies, []byte("session"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.migrateProfile(id); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(w.profileDir(id), "WebView2", "EBWebView", "Cookies"))
	if err != nil || string(data) != "session" {
		t.Fatalf("sesi tidak pindah utuh: %q %v", data, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("data lama masih tersisa: %v", err)
	}
}

func TestMigrateProfileConflictFails(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	appData := t.TempDir()
	w := &Workspace{file: filepath.Join(appData, "tabs.json")}
	id := newTab("x").ID

	legacy := legacyProfileDirectory(profileName(id))
	newRoot := w.profileDir(id)
	for _, dir := range []string{legacy, newRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "marker"), []byte(dir), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.migrateProfile(id); err == nil {
		t.Fatal("data di dua tempat diterima tanpa konflik")
	}
	for _, dir := range []string{legacy, newRoot} {
		data, err := os.ReadFile(filepath.Join(dir, "marker"))
		if err != nil || string(data) != dir {
			t.Fatalf("data %s berubah: %q %v", dir, data, err)
		}
	}
}

func TestMigrateProfileFreshIsNoop(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	appData := t.TempDir()
	w := &Workspace{file: filepath.Join(appData, "tabs.json")}
	id := newTab("x").ID
	if err := w.migrateProfile(id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.profileDir(id)); !os.IsNotExist(err) {
		t.Fatalf("profil baru dibuat sebelum waktunya: %v", err)
	}
}

func TestMigrateProfileAlreadyMigratedKeepsNewData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	appData := t.TempDir()
	w := &Workspace{file: filepath.Join(appData, "tabs.json")}
	id := newTab("x").ID
	newRoot := w.profileDir(id)
	if err := os.MkdirAll(newRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newRoot, "settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.migrateProfile(id); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(newRoot, "settings.json")); err != nil || string(data) != "{}" {
		t.Fatalf("data yang sudah pindah berubah: %q %v", data, err)
	}
}

func TestMigrateProfileIgnoresEmptyLegacySkeleton(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	appData := t.TempDir()
	w := &Workspace{file: filepath.Join(appData, "tabs.json")}
	id := newTab("x").ID

	// A worker already moved the profile: real data at the canonical root, and
	// MyGo's empty WebView2 skeleton left behind at the legacy path.
	newRoot := w.profileDir(id)
	webview := filepath.Join(newRoot, "WebView2", "EBWebView")
	if err := os.MkdirAll(webview, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webview, "Cookies"), []byte("session"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := legacyProfileDirectory(profileName(id))
	if err := os.MkdirAll(filepath.Join(legacy, "WebView2"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Two startups must both read the skeleton as empty, never as a conflict.
	for i := range 2 {
		if err := w.migrateProfile(id); err != nil {
			t.Fatalf("migrasi ke-%d gagal: %v", i+1, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(newRoot, "WebView2", "EBWebView", "Cookies")); err != nil || string(data) != "session" {
		t.Fatalf("sesi profil hilang: %q %v", data, err)
	}
}

func TestProfileDataDirsIsolateProfiles(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	w := &Workspace{file: filepath.Join(t.TempDir(), "tabs.json")}
	first := newTab("a").ID
	second := newTab("b").ID

	seen := map[string]bool{}
	for _, id := range []string{first, second} {
		for _, dir := range w.profileDataDirs(id) {
			if seen[dir] {
				t.Fatalf("direktori profil dipakai dua akun: %s", dir)
			}
			seen[dir] = true
		}
	}
}

func TestMigrateProfileSettingsConflictKeepsData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	w := &Workspace{file: filepath.Join(t.TempDir(), "tabs.json")}
	id := newTab("x").ID
	old := legacySettingsDirectory(profileName(id))
	target := w.profileDir(id)
	for _, root := range []string{old, target} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "downloads.json"), []byte(root), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.migrateProfile(id); err == nil {
		t.Fatal("conflicting download lists were silently accepted")
	}
	for _, root := range []string{old, target} {
		if got, err := os.ReadFile(filepath.Join(root, "downloads.json")); err != nil || string(got) != root {
			t.Fatalf("download list changed: %q, %v", got, err)
		}
	}
}
