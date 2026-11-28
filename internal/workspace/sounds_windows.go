package workspace

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"whatsappgui-new/internal/profile"
)

// NotificationSoundsChanged notifies the UI when notification sound settings change.
var NotificationSoundsChanged = mygo.NewEvent[profile.NotificationSoundSettings]("notificationSoundsChanged")

type notificationSoundStore struct {
	mu       sync.Mutex
	file     string
	dir      string
	settings profile.NotificationSoundSettings
	loaded   bool
	loadErr  error
}

func newNotificationSoundStore(file, dir string) *notificationSoundStore {
	return &notificationSoundStore{
		file:     file,
		dir:      dir,
		settings: profile.DefaultNotificationSoundSettings(),
	}
}

func (s *notificationSoundStore) ensureLoadedLocked() (profile.NotificationSoundSettings, error) {
	if s.loaded {
		return s.settings, s.loadErr
	}
	s.loaded = true
	data, err := os.ReadFile(s.file)
	if errors.Is(err, os.ErrNotExist) {
		s.settings = profile.DefaultNotificationSoundSettings()
		return s.settings, nil
	}
	if err != nil {
		s.settings = profile.DefaultNotificationSoundSettings()
		s.loadErr = err
		return s.settings, err
	}
	var raw profile.NotificationSoundSettings
	if err := json.Unmarshal(data, &raw); err != nil {
		s.settings = profile.DefaultNotificationSoundSettings()
		s.loadErr = fmt.Errorf("notification-sounds.json tidak valid: %w", err)
		return s.settings, s.loadErr
	}
	normalized, err := profile.NormalizeNotificationSoundSettings(raw)
	if err != nil {
		s.settings = profile.DefaultNotificationSoundSettings()
		s.loadErr = fmt.Errorf("notification-sounds.json tidak valid: %w", err)
		return s.settings, s.loadErr
	}
	// If a custom file was removed externally, keep the app working cleanly.
	if normalized.Message.Mode == profile.SoundModeCustom && s.customFilePathLocked(profile.SoundKindMessage, normalized.Message.FileName) == "" {
		normalized.Message.Mode = profile.SoundModeDefault
		normalized.Message.FileName = ""
		normalized.Message.MimeType = ""
	}
	if normalized.Call.Mode == profile.SoundModeCustom && s.customFilePathLocked(profile.SoundKindCall, normalized.Call.FileName) == "" {
		normalized.Call.Mode = profile.SoundModeDefault
		normalized.Call.FileName = ""
		normalized.Call.MimeType = ""
	}
	s.settings = normalized
	return s.settings, nil
}

func (s *notificationSoundStore) customFilePathLocked(kind, fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != "" {
		candidate := filepath.Join(s.dir, kind+ext)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return ""
	}
	prefix := kind + "."
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), prefix) {
			return filepath.Join(s.dir, entry.Name())
		}
	}
	return ""
}

func (s *notificationSoundStore) removeKindFilesLocked(kind string) error {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	prefix := kind + "."
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(strings.ToLower(entry.Name()), prefix) {
			if err := os.Remove(filepath.Join(s.dir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (s *notificationSoundStore) saveLocked(settings profile.NotificationSoundSettings) error {
	if err := os.MkdirAll(filepath.Dir(s.file), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.file, data); err != nil {
		return err
	}
	s.settings = settings
	s.loaded = true
	s.loadErr = nil
	return nil
}

func (s *notificationSoundStore) get() (profile.NotificationSoundSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureLoadedLocked()
}

func (s *notificationSoundStore) set(next profile.NotificationSoundSettings) (profile.NotificationSoundSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, _ := s.ensureLoadedLocked()
	// Preserve existing custom file metadata when switching modes or adjusting volume.
	if next.Message.FileName == "" && current.Message.FileName != "" {
		next.Message.FileName = current.Message.FileName
		next.Message.MimeType = current.Message.MimeType
	}
	if next.Call.FileName == "" && current.Call.FileName != "" {
		next.Call.FileName = current.Call.FileName
		next.Call.MimeType = current.Call.MimeType
	}
	normalized, err := profile.NormalizeNotificationSoundSettings(next)
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if normalized.Message.Mode == profile.SoundModeCustom && s.customFilePathLocked(profile.SoundKindMessage, normalized.Message.FileName) == "" {
		return profile.NotificationSoundSettings{}, errors.New("berkas audio kustom pesan masuk belum tersedia")
	}
	if normalized.Call.Mode == profile.SoundModeCustom && s.customFilePathLocked(profile.SoundKindCall, normalized.Call.FileName) == "" {
		return profile.NotificationSoundSettings{}, errors.New("berkas audio kustom telepon masuk belum tersedia")
	}
	if err := s.saveLocked(normalized); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	return normalized, nil
}

func (s *notificationSoundStore) importFile(kind, srcPath string) (profile.NotificationSoundSettings, error) {
	if !profile.ValidSoundKind(kind) {
		return profile.NotificationSoundSettings{}, errors.New("jenis suara notifikasi tidak valid")
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	info, err := os.Stat(srcPath)
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if info.IsDir() {
		return profile.NotificationSoundSettings{}, errors.New("tidak dapat memilih folder sebagai suara notifikasi")
	}
	if info.Size() == 0 {
		return profile.NotificationSoundSettings{}, errors.New("berkas audio kosong")
	}
	if info.Size() > profile.MaxNotificationSoundBytes {
		return profile.NotificationSoundSettings{}, errors.New("ukuran berkas audio melebihi batas 7 MB")
	}
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if len(raw) > profile.MaxNotificationSoundBytes {
		return profile.NotificationSoundSettings{}, errors.New("ukuran berkas audio melebihi batas 7 MB")
	}
	mimeType, ok := profile.SoundMimeTypeForExt(ext)
	if !ok {
		detected := http.DetectContentType(raw[:min(512, len(raw))])
		switch {
		case strings.HasPrefix(detected, "audio/"):
			mimeType = detected
		case detected == "application/ogg":
			mimeType = "audio/ogg"
		default:
			return profile.NotificationSoundSettings{}, errors.New("format audio harus MP3, WAV, OGG, OPUS, M4A, AAC, FLAC, atau WEBM")
		}
		if ext == "" {
			ext = ".bin"
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current, _ := s.ensureLoadedLocked()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if err := s.removeKindFilesLocked(kind); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	target := filepath.Join(s.dir, kind+ext)
	if err := writeFileAtomic(target, raw); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	choice := profile.NotificationSoundChoice{
		Mode:     profile.SoundModeCustom,
		Preset:   current.Message.Preset,
		FileName: filepath.Base(srcPath),
		MimeType: mimeType,
		Volume:   current.Message.Volume,
	}
	if kind == profile.SoundKindCall {
		choice.Preset = current.Call.Preset
		choice.Volume = current.Call.Volume
		current.Call = choice
	} else {
		current.Message = choice
	}
	normalized, err := profile.NormalizeNotificationSoundSettings(current)
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if err := s.saveLocked(normalized); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	return normalized, nil
}

func (s *notificationSoundStore) clearCustom(kind string) (profile.NotificationSoundSettings, error) {
	if !profile.ValidSoundKind(kind) {
		return profile.NotificationSoundSettings{}, errors.New("jenis suara notifikasi tidak valid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, _ := s.ensureLoadedLocked()
	if err := s.removeKindFilesLocked(kind); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	reset := func(c profile.NotificationSoundChoice) profile.NotificationSoundChoice {
		if c.Mode == profile.SoundModeCustom {
			c.Mode = profile.SoundModeDefault
		}
		c.FileName = ""
		c.MimeType = ""
		return c
	}
	if kind == profile.SoundKindCall {
		current.Call = reset(current.Call)
	} else {
		current.Message = reset(current.Message)
	}
	normalized, err := profile.NormalizeNotificationSoundSettings(current)
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if err := s.saveLocked(normalized); err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	return normalized, nil
}

func (s *notificationSoundStore) customDataURL(kind string) (string, error) {
	if !profile.ValidSoundKind(kind) {
		return "", errors.New("jenis suara notifikasi tidak valid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, _ := s.ensureLoadedLocked()
	choice := settings.Message
	if kind == profile.SoundKindCall {
		choice = settings.Call
	}
	if choice.FileName == "" || choice.MimeType == "" {
		return "", nil
	}
	path := s.customFilePathLocked(kind, choice.FileName)
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(raw) > profile.MaxNotificationSoundBytes {
		return "", errors.New("berkas audio melebihi batas 7 MB")
	}
	return "data:" + choice.MimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func (s *notificationSoundStore) payload() profile.NotificationSoundsPayload {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, _ := s.ensureLoadedLocked()

	build := func(kind string, choice profile.NotificationSoundChoice) profile.NotificationSoundChannel {
		ch := profile.NotificationSoundChannel{
			Mode:   choice.Mode,
			Preset: choice.Preset,
			Volume: choice.Volume,
		}
		if choice.Mode == profile.SoundModeCustom {
			if path := s.customFilePathLocked(kind, choice.FileName); path != "" {
				if raw, err := os.ReadFile(path); err == nil && len(raw) <= profile.MaxNotificationSoundBytes {
					ch.DataURL = "data:" + choice.MimeType + ";base64," + base64.StdEncoding.EncodeToString(raw)
					return ch
				}
			}
			ch.Mode = profile.SoundModeDefault
		}
		return ch
	}

	return profile.NotificationSoundsPayload{
		Message: build(profile.SoundKindMessage, settings.Message),
		Call:    build(profile.SoundKindCall, settings.Call),
	}
}

// GetNotificationSounds returns the workspace notification sound configuration.
func (w *Workspace) GetNotificationSounds() (profile.NotificationSoundSettings, error) {
	if w.sounds == nil {
		return profile.DefaultNotificationSoundSettings(), nil
	}
	return w.sounds.get()
}

// SetNotificationSounds saves and applies notification sound settings to all profiles.
func (w *Workspace) SetNotificationSounds(settings profile.NotificationSoundSettings) error {
	if w.sounds == nil {
		return errors.New("penyimpanan suara notifikasi belum siap")
	}
	saved, err := w.sounds.set(settings)
	if err != nil {
		return err
	}
	return w.broadcastNotificationSounds(saved)
}

// ChooseNotificationSound opens a file picker to import a custom audio file for
// incoming messages ("message") or incoming calls ("call").
func (w *Workspace) ChooseNotificationSound(kind string) (profile.NotificationSoundSettings, error) {
	if !profile.ValidSoundKind(kind) {
		return profile.NotificationSoundSettings{}, errors.New("jenis suara notifikasi tidak valid")
	}
	if w.sounds == nil {
		return profile.NotificationSoundSettings{}, errors.New("penyimpanan suara notifikasi belum siap")
	}
	title := "Pilih suara notifikasi pesan masuk"
	if kind == profile.SoundKindCall {
		title = "Pilih nada dering telepon masuk"
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      w.window,
		Title:       title,
		ButtonLabel: "Pilih Audio",
		Filters: []mygo.FileFilter{
			{Name: "Audio (*.mp3, *.wav, *.ogg, *.opus, *.m4a, *.aac, *.flac, *.webm)", Extensions: []string{"mp3", "wav", "ogg", "oga", "opus", "m4a", "mp4", "aac", "flac", "webm"}},
			{Name: "Semua Berkas (*.*)", Extensions: []string{"*"}},
		},
	})
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	if len(paths) == 0 {
		return w.sounds.get()
	}
	saved, err := w.sounds.importFile(kind, paths[0])
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	return saved, w.broadcastNotificationSounds(saved)
}

// ClearNotificationSound removes a custom audio file and restores the default sound.
func (w *Workspace) ClearNotificationSound(kind string) (profile.NotificationSoundSettings, error) {
	if w.sounds == nil {
		return profile.NotificationSoundSettings{}, errors.New("penyimpanan suara notifikasi belum siap")
	}
	saved, err := w.sounds.clearCustom(kind)
	if err != nil {
		return profile.NotificationSoundSettings{}, err
	}
	return saved, w.broadcastNotificationSounds(saved)
}

// NotificationSoundPreview returns a data URL for testing the saved custom audio file.
func (w *Workspace) NotificationSoundPreview(kind string) (string, error) {
	if w.sounds == nil {
		return "", nil
	}
	return w.sounds.customDataURL(kind)
}

func (w *Workspace) broadcastNotificationSounds(settings profile.NotificationSoundSettings) error {
	if w.window != nil && !w.window.IsDestroyed() {
		_ = NotificationSoundsChanged.Emit(w.window, settings)
	}
	payload := w.sounds.payload()
	w.mu.Lock()
	procs := make([]*profileProcess, 0, len(w.processes))
	for _, p := range w.processes {
		procs = append(procs, p)
	}
	w.mu.Unlock()
	var errs []error
	for _, p := range procs {
		if err := w.sendCommand(p, profile.Command{Action: "sounds", Sounds: &payload}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
