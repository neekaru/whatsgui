package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneLegacySkeletonKeepsData(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	name := "workspace-profile-00112233445566778899aabbccddeeff"
	root := filepath.Join(local, name)
	if err := os.MkdirAll(filepath.Join(root, "WebView2"), 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "WebView2", "Cookies")
	if err := os.WriteFile(marker, []byte("session"), 0o600); err != nil {
		t.Fatal(err)
	}
	pruneLegacySkeleton(name)
	if got, err := os.ReadFile(marker); err != nil || string(got) != "session" {
		t.Fatalf("cleanup removed a real session: %q, %v", got, err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	pruneLegacySkeleton(name)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("empty legacy profile was not removed: %v", err)
	}
}
