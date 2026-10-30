package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cacheDirectories are the WebView2 paths that hold downloaded web content
// only. Cookies, Local Storage, Session Storage, IndexedDB and service
// worker registrations live elsewhere and must survive a cache clear, or
// the user would be signed out of WhatsApp.
var cacheDirectories = []string{
	filepath.Join("Default", "Cache"),
	filepath.Join("Default", "Code Cache"),
	filepath.Join("Default", "GPUCache"),
	filepath.Join("Default", "DawnGraphiteCache"),
	filepath.Join("Default", "DawnWebGPUCache"),
	"ShaderCache",
	"GrShaderCache",
	"GPUPersistentCache",
}

// clearCache deletes the downloaded content of one profile. Its worker must
// be stopped first: WebView2 keeps these files mapped while it runs. Sharing
// violations are retried for up to three seconds and otherwise returned.
func clearCache(dir string) error {
	// MyGo hands WebView2 the profile's WebView2 directory; the runtime keeps
	// the browser profile in EBWebView below it.
	base := filepath.Join(dir, "WebView2")
	if _, err := os.Stat(filepath.Join(base, "EBWebView")); err == nil {
		base = filepath.Join(base, "EBWebView")
	}
	// Deleting nothing because the profile moved would report a false success.
	if _, err := os.Stat(filepath.Join(base, "Default")); err != nil {
		return fmt.Errorf("folder data profil tidak ditemukan: %w", err)
	}
	var failures []error
	deadline := time.Now().Add(3 * time.Second)
	for _, name := range cacheDirectories {
		path := filepath.Join(base, name)
		for {
			err := os.RemoveAll(path)
			if err == nil {
				break
			}
			if !fileBusy(err) || !time.Now().Before(deadline) {
				failures = append(failures, fmt.Errorf("%s: %w", name, err))
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	return errors.Join(failures...)
}

// The profile worker is stopped before this is called.
func clearProfileCache(dir string) error {
	err := clearCache(dir)
	if !fileBusy(err) {
		return err
	}
	// Old WebView2 browsers may outlive their worker and keep cache open.
	if recoveryErr := stopOrphanBrowser(filepath.Join(dir, "WebView2")); recoveryErr != nil {
		return errors.Join(err, recoveryErr)
	}
	return clearCache(dir)
}
