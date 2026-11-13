package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
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
