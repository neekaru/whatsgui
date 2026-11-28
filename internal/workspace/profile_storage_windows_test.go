package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
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

func TestLegacyAppNameResolution(t *testing.T) {
	cases := map[string]string{
		"WhatsGUI":            "whatsappgui-new",
		"WhatsGUI Dev":        "whatsappgui-new Dev",
		"whatsappgui-new":     "",
		"whatsappgui-new Dev": "",
	}
	for current, want := range cases {
		if got := legacyAppName(current); got != want {
			t.Fatalf("legacyAppName(%q) = %q, ingin %q", current, got, want)
		}
	}
}

func TestMigrateAppStorageMovesRoamingAndLocalProfiles(t *testing.T) {
	roamingRoot := t.TempDir()
	localRoot := t.TempDir()
	oldUserData := filepath.Join(roamingRoot, "whatsappgui-new")
	newUserData := filepath.Join(roamingRoot, "WhatsGUI")
	oldLocalData := filepath.Join(localRoot, "whatsappgui-new")
	newLocalData := filepath.Join(localRoot, "WhatsGUI")

	// Empty PathUserData target may already exist on startup.
	if err := os.MkdirAll(filepath.Join(newUserData, "userscripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(oldUserData, "userscripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldUserData, "tabs.json"), []byte(`{"tabs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldUserData, "userscripts", "custom.user.js"), []byte("// ==UserScript=="), 0o600); err != nil {
		t.Fatal(err)
	}

	oldCookies := filepath.Join(oldLocalData, "profiles", "p1", "WebView2", "EBWebView", "Cookies")
	if err := os.MkdirAll(filepath.Dir(oldCookies), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldCookies, []byte("wa-cookie"), 0o600); err != nil {
		t.Fatal(err)
	}

	migrated, err := migrateAppStorage(oldUserData, newUserData, oldLocalData, newLocalData)
	if err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Fatal("migrasi harus melaporkan migrated=true")
	}
	if got, err := os.ReadFile(filepath.Join(newUserData, "tabs.json")); err != nil || string(got) != `{"tabs":[]}` {
		t.Fatalf("tabs.json tidak termigrasi: %q %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(newUserData, "userscripts", "custom.user.js")); err != nil || string(got) != "// ==UserScript==" {
		t.Fatalf("userscript tidak termigrasi: %q %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(newLocalData, "profiles", "p1", "WebView2", "EBWebView", "Cookies")); err != nil || string(got) != "wa-cookie" {
		t.Fatalf("profil lokal tidak termigrasi: %q %v", got, err)
	}
	if _, err := os.Stat(oldUserData); !os.IsNotExist(err) {
		t.Fatalf("folder lama roaming masih ada: %v", err)
	}
	if _, err := os.Stat(oldLocalData); !os.IsNotExist(err) {
		t.Fatalf("folder lama lokal masih ada: %v", err)
	}
}

func TestMigrateAppStorageSkipsExistingNewData(t *testing.T) {
	roamingRoot := t.TempDir()
	localRoot := t.TempDir()
	oldUserData := filepath.Join(roamingRoot, "whatsappgui-new")
	newUserData := filepath.Join(roamingRoot, "WhatsGUI")
	oldLocalData := filepath.Join(localRoot, "whatsappgui-new")
	newLocalData := filepath.Join(localRoot, "WhatsGUI")

	for _, pair := range []struct {
		dir     string
		content string
	}{
		{oldUserData, "old-tabs"},
		{newUserData, "new-tabs"},
		{filepath.Join(oldLocalData, "profiles", "p1"), "old-cookie"},
		{filepath.Join(newLocalData, "profiles", "p1"), "new-cookie"},
	} {
		if err := os.MkdirAll(pair.dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pair.dir, "marker"), []byte(pair.content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	migrated, err := migrateAppStorage(oldUserData, newUserData, oldLocalData, newLocalData)
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Fatal("migrasi tidak boleh melaporkan migrated saat tujuan sudah berisi")
	}
	if got, err := os.ReadFile(filepath.Join(newUserData, "marker")); err != nil || string(got) != "new-tabs" {
		t.Fatalf("data roaming baru tertimpa: %q %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(newLocalData, "profiles", "p1", "marker")); err != nil || string(got) != "new-cookie" {
		t.Fatalf("data lokal baru tertimpa: %q %v", got, err)
	}
}

func TestOpenWorkspaceMigratesLegacyAppOnBoot(t *testing.T) {
	roamingRoot := t.TempDir()
	localRoot := t.TempDir()
	t.Setenv("LOCALAPPDATA", localRoot)
	t.Setenv("APPDATA", roamingRoot)

	oldName := legacyAppName(mygo.App.Name())
	if oldName == "" {
		t.Skip("test binary already uses legacy app name")
	}

	tab := newTab("Kerja")
	oldUserData := filepath.Join(roamingRoot, oldName)
	if err := os.MkdirAll(oldUserData, 0o755); err != nil {
		t.Fatal(err)
	}
	tabsJSON := `{"tabs":[{"id":"` + tab.ID + `","name":"Kerja"}],"activeId":"` + tab.ID + `"}`
	if err := os.WriteFile(filepath.Join(oldUserData, "tabs.json"), []byte(tabsJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	oldCookies := filepath.Join(localRoot, oldName, "profiles", tab.ID, "WebView2", "Cookies")
	if err := os.MkdirAll(filepath.Dir(oldCookies), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldCookies, []byte("session-ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	newUserData := filepath.Join(roamingRoot, mygo.App.Name())
	mygo.App.SetPath(mygo.PathUserData, newUserData)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathUserData, "") })

	w, err := openWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if len(w.data.Tabs) != 1 || w.data.Tabs[0].ID != tab.ID || w.data.Tabs[0].Name != "Kerja" {
		t.Fatalf("tab lama tidak termigrasi saat boot: %+v", w.data.Tabs)
	}
	if got, err := os.ReadFile(filepath.Join(w.profileDir(tab.ID), "WebView2", "Cookies")); err != nil || string(got) != "session-ok" {
		t.Fatalf("cookies profil tidak termigrasi saat boot: %q %v", got, err)
	}
}

func TestMaybeUninstallLegacyApp(t *testing.T) {
	setupInstalled := func(t *testing.T) (newExe, legacyUninst, oldUser, newUser, oldLocal string, reg map[string]uninstallEntry) {
		t.Helper()
		programs := t.TempDir()
		newDir := filepath.Join(programs, "WhatsGUI")
		oldDir := filepath.Join(programs, "whatsappgui-new")
		for _, dir := range []string{newDir, oldDir} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "Uninstall.exe"), []byte("uninst"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		newExe = filepath.Join(newDir, "WhatsGUI.exe")
		if err := os.WriteFile(newExe, []byte("exe"), 0o600); err != nil {
			t.Fatal(err)
		}
		legacyUninst = filepath.Join(oldDir, "Uninstall.exe")
		roaming := t.TempDir()
		local := t.TempDir()
		oldUser = filepath.Join(roaming, "whatsappgui-new")
		newUser = filepath.Join(roaming, "WhatsGUI")
		oldLocal = filepath.Join(local, "whatsappgui-new")
		if err := os.MkdirAll(newUser, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(newUser, "tabs.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		reg = map[string]uninstallEntry{
			newWindowsIdentifier: {
				InstallLocation:      newDir,
				UninstallString:      `"` + filepath.Join(newDir, "Uninstall.exe") + `"`,
				QuietUninstallString: `"` + filepath.Join(newDir, "Uninstall.exe") + `" /S`,
			},
			legacyWindowsIdentifier: {
				InstallLocation:      oldDir,
				UninstallString:      `"` + legacyUninst + `"`,
				QuietUninstallString: `"` + legacyUninst + `" /S`,
			},
		}
		return
	}

	t.Run("uninstalls legacy installer after migration", func(t *testing.T) {
		newExe, legacyUninst, oldUser, newUser, oldLocal, reg := setupInstalled(t)
		var ran string
		ok := maybeUninstallLegacyApp(newExe, true, oldUser, newUser, oldLocal,
			func(id string) (uninstallEntry, bool) { v, hit := reg[id]; return v, hit },
			func(path string) error { ran = path; return nil },
		)
		if !ok || ran != legacyUninst {
			t.Fatalf("uninstaller lama tidak dijalankan: ok=%v ran=%q want=%q", ok, ran, legacyUninst)
		}
	})

	t.Run("uninstalls legacy installer on later boot when data already migrated", func(t *testing.T) {
		newExe, legacyUninst, oldUser, newUser, oldLocal, reg := setupInstalled(t)
		var ran string
		ok := maybeUninstallLegacyApp(newExe, false, oldUser, newUser, oldLocal,
			func(id string) (uninstallEntry, bool) { v, hit := reg[id]; return v, hit },
			func(path string) error { ran = path; return nil },
		)
		if !ok || ran != legacyUninst {
			t.Fatalf("uninstaller lama tidak dijalankan saat data sudah di versi baru: ok=%v ran=%q", ok, ran)
		}
	})

	t.Run("skips when current app is portable", func(t *testing.T) {
		_, _, oldUser, newUser, oldLocal, reg := setupInstalled(t)
		portableDir := t.TempDir()
		portableExe := filepath.Join(portableDir, "WhatsGUI.exe")
		if err := os.WriteFile(portableExe, []byte("exe"), 0o600); err != nil {
			t.Fatal(err)
		}
		var ran string
		ok := maybeUninstallLegacyApp(portableExe, true, oldUser, newUser, oldLocal,
			func(id string) (uninstallEntry, bool) { v, hit := reg[id]; return v, hit },
			func(path string) error { ran = path; return nil },
		)
		if ok || ran != "" {
			t.Fatalf("versi portable tidak boleh meng-uninstall versi lama: ok=%v ran=%q", ok, ran)
		}
	})

	t.Run("skips when legacy app was portable", func(t *testing.T) {
		newExe, _, oldUser, newUser, oldLocal, reg := setupInstalled(t)
		delete(reg, legacyWindowsIdentifier)
		var ran string
		ok := maybeUninstallLegacyApp(newExe, true, oldUser, newUser, oldLocal,
			func(id string) (uninstallEntry, bool) { v, hit := reg[id]; return v, hit },
			func(path string) error { ran = path; return nil },
		)
		if ok || ran != "" {
			t.Fatalf("versi lama portable seharusnya tidak ada uninstaller: ok=%v ran=%q", ok, ran)
		}
	})

	t.Run("skips when legacy data still unmigrated", func(t *testing.T) {
		newExe, _, oldUser, newUser, oldLocal, reg := setupInstalled(t)
		if err := os.MkdirAll(oldUser, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(oldUser, "tabs.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		var ran string
		ok := maybeUninstallLegacyApp(newExe, false, oldUser, newUser, oldLocal,
			func(id string) (uninstallEntry, bool) { v, hit := reg[id]; return v, hit },
			func(path string) error { ran = path; return nil },
		)
		if ok || ran != "" {
			t.Fatalf("tidak boleh uninstall jika data lama masih belum termigrasi: ok=%v ran=%q", ok, ran)
		}
	})
}
