package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
	"whatsappgui-new/internal/profile"
)

func TestTabPersistenceAndValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	mygo.App.SetPath(mygo.PathUserData, dir)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, "") })
	workspace, err := openWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	first := workspace.data.Tabs[0]
	second := newTab("Bisnis")
	if first.ID == second.ID || profileDirectory(profileName(first.ID)) == profileDirectory(profileName(second.ID)) {
		t.Fatal("dua akun harus memiliki direktori WebView2 berbeda")
	}
	workspace.data.Tabs = append(workspace.data.Tabs, second)
	workspace.data.ActiveID = second.ID
	if err := workspace.save(workspace.data); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Rename(second.ID, "  Tim penjualan  "); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.Rename(second.ID, "\n "); err == nil {
		t.Fatal("nama kosong diterima")
	}
	reopened, err := openWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if reopened.data.ActiveID != second.ID || reopened.data.Tabs[1].Name != "Tim penjualan" {
		t.Fatalf("identitas/nama/aktivasi tidak dipersist: %+v", reopened.data)
	}
	profileDir := profileDirectory(profileName(second.ID))
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "session"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := windows.UTF16PtrFromString(filepath.Join(profileDir, "session"))
	if err != nil {
		t.Fatal(err)
	}
	// Match WebView2's cache handle: readable, but not shareable for deletion.
	locked, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if locked != 0 {
			windows.CloseHandle(locked)
		}
	}()
	state, err := reopened.Remove(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state.ActiveID != first.ID || len(state.Tabs) != 1 {
		t.Fatalf("tab tetangga tidak diaktifkan: %+v", state)
	}
	reopened, err = openWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.deletedIDs) != 1 || reopened.deletedIDs[0] != second.ID || reopened.index(second.ID) >= 0 {
		t.Fatalf("profil terkunci harus tetap terhapus dan antrean cleanup dipersist: %+v", reopened)
	}
	if err := windows.CloseHandle(locked); err != nil {
		t.Fatal(err)
	}
	locked = 0
	if err := reopened.cleanupDeleted(); err != nil {
		t.Fatal(err)
	}
	afterCleanup, err := openWorkspace()
	if err != nil || len(afterCleanup.deletedIDs) != 0 {
		t.Fatalf("antrean tidak dibersihkan setelah lock lepas: %+v %v", afterCleanup, err)
	}
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("data tab yang dihapus masih ada: %v", err)
	}
	state, err = reopened.Remove(first.ID)
	if err != nil || len(state.Tabs) != 0 || state.ActiveID != "" {
		t.Fatalf("hapus tab terakhir: %+v %v", state, err)
	}
	reopened, err = openWorkspace()
	if err != nil || len(reopened.data.Tabs) != 0 {
		t.Fatalf("tab terakhir muncul kembali: %+v %v", reopened, err)
	}
	if profile.ValidID("../../profile") || profile.ValidID("AABBCCDDEEFF00112233445566778899") {
		t.Fatal("ID profil tidak aman diterima")
	}
	if err := os.WriteFile(reopened.file, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openWorkspace(); err == nil {
		t.Fatal("konfigurasi rusak ditimpa diam-diam")
	}
}

func TestWindowFitsWorkArea(t *testing.T) {
	for _, tc := range []struct {
		name               string
		bounds, area, want mygo.Rectangle
	}{
		{"short screen", mygo.Rectangle{X: 43, Y: -46, Width: 1280, Height: 860}, mygo.Rectangle{Width: 1366, Height: 728}, mygo.Rectangle{X: 43, Width: 1280, Height: 728}},
		{"restored above screen", mygo.Rectangle{X: 100, Y: -80, Width: 960, Height: 640}, mygo.Rectangle{Y: 40, Width: 1920, Height: 1040}, mygo.Rectangle{X: 100, Y: 40, Width: 960, Height: 640}},
		{"left monitor", mygo.Rectangle{X: -2000, Y: 100, Width: 1280, Height: 860}, mygo.Rectangle{X: -1920, Width: 1920, Height: 1040}, mygo.Rectangle{X: -1920, Y: 100, Width: 1280, Height: 860}},
		{"visible position preserved", mygo.Rectangle{X: 200, Y: 100, Width: 960, Height: 640}, mygo.Rectangle{Width: 1920, Height: 1040}, mygo.Rectangle{X: 200, Y: 100, Width: 960, Height: 640}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fitWorkArea(tc.bounds, tc.area); got != tc.want {
				t.Fatalf("window bounds = %+v; want %+v", got, tc.want)
			}
		})
	}
}
