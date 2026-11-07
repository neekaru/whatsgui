package profile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Settings are the per-profile preferences applied to the profile's WhatsApp
// pages. They live beside the profile's WebView2 data and go away with it.
type Settings struct {
	Theme       string `json:"theme"`
	PrivacyBlur bool   `json:"privacyBlur"`
}

// Themes of WhatsApp itself.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// DefaultSettings keeps WhatsApp's own theme and shows everything, as before.
var DefaultSettings = Settings{Theme: ThemeSystem}

// DataDir is the directory of a profile, named after it: WebView2's user data
// folder and the settings both live there.
func DataDir(name string) string {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, name)
}

// LoadSettings reads the settings of the profile kept in dir. A profile that
// never saved any gets DefaultSettings.
func LoadSettings(dir string) (Settings, error) {
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, errors.New("settings.json profil tidak valid")
	}
	return NormalizeSettings(settings)
}

// SaveSettings writes the settings of a profile atomically, so a crash never
// leaves a half-written file behind.
func SaveSettings(dir string, settings Settings) error {
	settings, err := NormalizeSettings(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "settings-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dir, "settings.json"))
}

// NormalizeSettings rejects a theme WhatsApp does not have. An empty theme
// means the default, so an older file keeps working.
func NormalizeSettings(settings Settings) (Settings, error) {
	switch strings.ToLower(strings.TrimSpace(settings.Theme)) {
	case "", ThemeSystem:
		settings.Theme = ThemeSystem
	case ThemeLight:
		settings.Theme = ThemeLight
	case ThemeDark:
		settings.Theme = ThemeDark
	default:
		return Settings{}, errors.New("tema profil harus system, light, atau dark")
	}
	return settings, nil
}
