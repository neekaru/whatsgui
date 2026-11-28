package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows/registry"
)

// profileDir keeps browser data local, not in roaming AppData with tabs.json.
// Each profile remains a single removable tree, independent of its tab label.
func (w *Workspace) profileDir(id string) string {
	return filepath.Join(legacyProfileDirectory(mygo.App.Name()), "profiles", id)
}

// legacyProfileDirectory resolves MyGo's native app-name directory. Old
// workers used a profile-specific name; new profiles nest under the app name.
func legacyProfileDirectory(name string) string {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, name)
}

// MyGo's old PathUserData used roaming AppData for downloads and preferences,
// independently of the native WebView2 directory in LocalAppData.
func legacySettingsDirectory(name string) string {
	if dir := os.Getenv("APPDATA"); dir != "" {
		return filepath.Join(dir, name)
	}
	return ""
}

// profileDataDirs includes both old storage locations for pending deletions.
func (w *Workspace) profileDataDirs(id string) []string {
	name := profileName(id)
	dirs := []string{w.profileDir(id)}
	for _, root := range []string{legacyProfileDirectory(name), legacySettingsDirectory(name)} {
		if root == "" {
			continue
		}
		duplicate := false
		for _, dir := range dirs {
			if strings.EqualFold(dir, root) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			dirs = append(dirs, root)
		}
	}
	return dirs
}

// migrateProfile moves a profile's legacy data into the canonical root with an
// atomic rename. It must run before the worker starts: a failure (locked files,
// or data in both places) stops the tab instead of silently opening a fresh
// WhatsApp session.
func (w *Workspace) migrateProfile(id string) error {
	oldRoot := legacyProfileDirectory(profileName(id))
	newRoot := w.profileDir(id)

	oldHas, err := dirHasData(oldRoot)
	if err != nil {
		return fmt.Errorf("profil %s: %w", id, err)
	}
	if oldHas {
		newHas, err := dirHasData(newRoot)
		if err != nil {
			return fmt.Errorf("profil %s: %w", id, err)
		}
		if newHas {
			return fmt.Errorf("profil %s memiliki data di %s dan %s sekaligus; pindahkan salah satunya", id, oldRoot, newRoot)
		}
		if err := os.MkdirAll(filepath.Dir(newRoot), 0o700); err != nil {
			return fmt.Errorf("profil %s: %w", id, err)
		}
		// Only remove an empty target; never overwrite a session.
		if err := os.RemoveAll(newRoot); err != nil {
			return fmt.Errorf("profil %s: %w", id, err)
		}
		if err := os.Rename(oldRoot, newRoot); err != nil {
			return fmt.Errorf("memindahkan data profil %s: %w", id, err)
		}
	} else if err := os.RemoveAll(oldRoot); err != nil {
		return fmt.Errorf("membersihkan folder kosong profil %s: %w", id, err)
	}
	settings := legacySettingsDirectory(profileName(id))
	if settings == "" || strings.EqualFold(settings, oldRoot) || strings.EqualFold(settings, newRoot) {
		return nil
	}
	return migrateProfileSettings(settings, newRoot)
}

// Roaming settings and local browser data were separate trees. Move settings
// entries without overwriting anything; a partial move resumes on next launch.
func migrateProfileSettings(oldRoot, newRoot string) error {
	entries, err := os.ReadDir(oldRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		target := filepath.Join(newRoot, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("data profil sudah ada di %s dan %s", filepath.Join(oldRoot, entry.Name()), target)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if len(entries) > 0 {
		if err := os.MkdirAll(newRoot, 0o700); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(oldRoot, entry.Name()), filepath.Join(newRoot, entry.Name())); err != nil {
			return fmt.Errorf("memindahkan pengaturan profil %s: %w", entry.Name(), err)
		}
	}
	return os.Remove(oldRoot)
}

// dirHasData reports whether path holds a file anywhere below it. A tree of
// empty directories — such as the WebView2 skeleton MyGo pre-creates at the old
// root — counts as empty, so it is never treated as data.
func dirHasData(root string) (bool, error) {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	found := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return found, nil
}

const (
	legacyAppBaseName       = "whatsappgui-new"
	legacyWindowsIdentifier = "com.example.whatsappguinew"
	newWindowsIdentifier    = "io.github.neekaru.whatsgui"
	windowsUninstallKeyRoot = `Software\Microsoft\Windows\CurrentVersion\Uninstall\`
)

// legacyAppName resolves the pre-rename directory name matching prod or dev builds.
func legacyAppName(currentName string) string {
	if strings.EqualFold(currentName, legacyAppBaseName) || strings.EqualFold(currentName, legacyAppBaseName+" Dev") {
		return ""
	}
	if strings.HasSuffix(currentName, " Dev") {
		return legacyAppBaseName + " Dev"
	}
	return legacyAppBaseName
}

// migrateAppStorage moves roaming workspace data and local browser profiles from
// the pre-rename app directory into the new app directory on startup without
// overwriting existing data. It reports whether any data was migrated.
func migrateAppStorage(oldUserData, newUserData, oldLocalData, newLocalData string) (bool, error) {
	roamingMoved, err := migrateDirIfTargetEmpty(oldUserData, newUserData)
	if err != nil {
		return false, err
	}
	localMoved := false
	if oldLocalData != "" && newLocalData != "" {
		oldProfiles := filepath.Join(oldLocalData, "profiles")
		newProfiles := filepath.Join(newLocalData, "profiles")
		localMoved, err = migrateDirIfTargetEmpty(oldProfiles, newProfiles)
		if err != nil {
			return roamingMoved, err
		}
		_ = os.Remove(oldLocalData)
	}
	return roamingMoved || localMoved, nil
}

func migrateDirIfTargetEmpty(oldRoot, newRoot string) (bool, error) {
	if oldRoot == "" || newRoot == "" || strings.EqualFold(oldRoot, newRoot) {
		return false, nil
	}
	oldHas, err := dirHasData(oldRoot)
	if err != nil {
		return false, err
	}
	if !oldHas {
		return false, nil
	}
	newHas, err := dirHasData(newRoot)
	if err != nil {
		return false, err
	}
	if newHas {
		return false, nil
	}
	entries, err := os.ReadDir(oldRoot)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(newRoot, 0o700); err != nil {
		return false, err
	}
	for _, entry := range entries {
		target := filepath.Join(newRoot, entry.Name())
		// !newHas guarantees target holds no files, only an empty skeleton directory if present.
		if _, err := os.Lstat(target); err == nil {
			if err := os.RemoveAll(target); err != nil {
				return false, err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		if err := os.Rename(filepath.Join(oldRoot, entry.Name()), target); err != nil {
			return false, fmt.Errorf("memindahkan data %s: %w", entry.Name(), err)
		}
	}
	return true, os.Remove(oldRoot)
}

type uninstallEntry struct {
	InstallLocation      string
	UninstallString      string
	QuietUninstallString string
}

func readWindowsUninstallEntry(identifier string) (uninstallEntry, bool) {
	if identifier == "" {
		return uninstallEntry{}, false
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsUninstallKeyRoot+identifier, registry.QUERY_VALUE)
	if err != nil {
		return uninstallEntry{}, false
	}
	defer key.Close()

	var entry uninstallEntry
	entry.InstallLocation, _, _ = key.GetStringValue("InstallLocation")
	entry.UninstallString, _, _ = key.GetStringValue("UninstallString")
	entry.QuietUninstallString, _, _ = key.GetStringValue("QuietUninstallString")
	if entry.InstallLocation == "" && entry.UninstallString == "" && entry.QuietUninstallString == "" {
		return uninstallEntry{}, false
	}
	return entry, true
}

func runSilentUninstaller(uninstallerPath string) error {
	cmd := exec.Command(uninstallerPath, "/S")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// maybeUninstallLegacyApp removes the installer-based pre-rename app once its
// data has migrated safely to the new app. Portable installs are left untouched.
func maybeUninstallLegacyApp(
	exePath string,
	migrated bool,
	oldUserData, newUserData, oldLocalData string,
	readReg func(identifier string) (uninstallEntry, bool),
	runUninstaller func(uninstallerPath string) error,
) bool {
	if exePath == "" || readReg == nil || runUninstaller == nil {
		return false
	}
	currentDir := filepath.Clean(filepath.Dir(exePath))
	if !isInstalledBuild(currentDir, newWindowsIdentifier, readReg) {
		return false
	}
	if !migrated {
		newHas, err := dirHasData(newUserData)
		if err != nil || !newHas {
			return false
		}
	}
	if has, err := dirHasData(oldUserData); err != nil || has {
		return false
	}
	if oldLocalData != "" {
		if has, err := dirHasData(filepath.Join(oldLocalData, "profiles")); err != nil || has {
			return false
		}
	}
	legacyUninstaller, ok := resolveLegacyUninstaller(currentDir, readReg)
	if !ok {
		return false
	}
	return runUninstaller(legacyUninstaller) == nil
}

func isInstalledBuild(installDir, identifier string, readReg func(string) (uninstallEntry, bool)) bool {
	entry, ok := readReg(identifier)
	if !ok {
		return false
	}
	regDir := cleanRegistryDir(entry.InstallLocation)
	if regDir == "" || !strings.EqualFold(regDir, filepath.Clean(installDir)) {
		return false
	}
	return fileExistsRegular(filepath.Join(installDir, "Uninstall.exe"))
}

func resolveLegacyUninstaller(currentDir string, readReg func(string) (uninstallEntry, bool)) (string, bool) {
	entry, ok := readReg(legacyWindowsIdentifier)
	if !ok {
		return "", false
	}
	oldDir := cleanRegistryDir(entry.InstallLocation)
	if oldDir == "" || !filepath.IsAbs(oldDir) || strings.EqualFold(oldDir, currentDir) {
		return "", false
	}
	uninstaller := filepath.Join(oldDir, "Uninstall.exe")
	if !fileExistsRegular(uninstaller) {
		return "", false
	}
	if entry.QuietUninstallString != "" || entry.UninstallString != "" {
		if !matchesUninstallerCommand(entry.QuietUninstallString, uninstaller) &&
			!matchesUninstallerCommand(entry.UninstallString, uninstaller) {
			return "", false
		}
	}
	return uninstaller, true
}

func cleanRegistryDir(raw string) string {
	trimmed := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), `"`))
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func matchesUninstallerCommand(cmdLine, uninstallerPath string) bool {
	s := strings.TrimSpace(cmdLine)
	if s == "" {
		return false
	}
	var exe string
	if strings.HasPrefix(s, `"`) {
		if end := strings.IndexByte(s[1:], '"'); end >= 0 {
			exe = s[1 : 1+end]
		}
	} else {
		exe = strings.Fields(s)[0]
	}
	if exe == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(exe), filepath.Clean(uninstallerPath))
}

func fileExistsRegular(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
