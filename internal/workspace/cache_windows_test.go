package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestClearCacheKeepsLoginData(t *testing.T) {
	login := []string{
		filepath.Join("Default", "Network", "Cookies"),
		filepath.Join("Default", "Local Storage", "leveldb", "000003.log"),
		filepath.Join("Default", "IndexedDB", "https_web.whatsapp.com_0.indexeddb.leveldb", "000003.log"),
		filepath.Join("Default", "Session Storage", "000003.log"),
		filepath.Join("Default", "Service Worker", "Database", "000003.log"),
		filepath.Join("Default", "Service Worker", "CacheStorage", "data"),
		filepath.Join("Default", "Service Worker", "ScriptCache", "data"),
		filepath.Join("Default", "Login Data"),
	}
	// WebView2 keeps the profile in EBWebView; a runtime that does not uses
	// the user data directory itself.
	for _, layout := range []struct {
		name string
		path []string
	}{{"ebwebview", []string{"EBWebView"}}, {"user data folder", nil}} {
		t.Run(layout.name, func(t *testing.T) {
			dir := t.TempDir()
			base := filepath.Join(append([]string{dir, "WebView2"}, layout.path...)...)
			write := func(rel string) {
				path := filepath.Join(base, rel)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range cacheDirectories {
				write(filepath.Join(name, "data_0"))
			}
			for _, name := range login {
				write(name)
			}

			if err := clearCache(dir); err != nil {
				t.Fatalf("clearCache: %v", err)
			}

			for _, name := range cacheDirectories {
				if _, err := os.Stat(filepath.Join(base, name)); !os.IsNotExist(err) {
					t.Fatalf("cache %s masih ada: %v", name, err)
				}
			}
			for _, name := range login {
				if _, err := os.Stat(filepath.Join(base, name)); err != nil {
					t.Fatalf("data login %s terhapus: %v", name, err)
				}
			}
		})
	}
	if err := clearCache(t.TempDir()); err == nil {
		t.Fatal("profil tanpa data dilaporkan berhasil dibersihkan")
	}
}

func TestClearCacheRetriesLockedFiles(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "WebView2", "EBWebView", "Default")
	cache := filepath.Join(base, "Cache", "locked")
	cookie := filepath.Join(base, "Network", "Cookies")
	for _, path := range []string{cache, cookie} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path, err := windows.UTF16PtrFromString(cache)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan error, 1)
	time.AfterFunc(250*time.Millisecond, func() { released <- windows.CloseHandle(handle) })
	err = clearCache(dir)
	if closeErr := <-released; closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatalf("cache tetap terkunci: %v", err)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("cache masih ada: %v", err)
	}
	if data, err := os.ReadFile(cookie); err != nil || string(data) != "data" {
		t.Fatalf("cookie berubah: %q, %v", data, err)
	}
}
