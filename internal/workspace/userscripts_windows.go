package workspace

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo"
	exampleuserscripts "whatsappgui-new/examples/userscripts"
	"whatsappgui-new/internal/profile"
)

const (
	exampleUserscriptFilename    = "privacy-blur-toggle.user.js"
	audioStickersExampleFilename = "wa-audio-stickers.user.js"
)

var bundledExamples = map[string]string{
	exampleUserscriptFilename:    exampleuserscripts.PrivacyBlurExample,
	audioStickersExampleFilename: exampleuserscripts.AudioStickersExample,
}

// Userscript describes a stored user script and its current enabled state.
type Userscript struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	RunAt       string   `json:"runAt"`
	Matches     []string `json:"matches"`
	Requires    []string `json:"requires"`
	Grants      []string `json:"grants"`
	MatchesApp  bool     `json:"matchesApp"`
	Enabled     bool     `json:"enabled"`
	Code        string   `json:"code"`
	UpdatedAt   int64    `json:"updatedAt"`
}

// UserscriptsChanged notifies the UI whenever the userscripts list changes.
var UserscriptsChanged = mygo.NewEvent[[]Userscript]("userscriptsChanged")

type userscriptStore struct {
	mu        sync.Mutex
	dir       string
	stateFile string
}

func newUserscriptStore(dir, stateFile string) *userscriptStore {
	return &userscriptStore{dir: dir, stateFile: stateFile}
}

func validUserscriptID(id string) bool {
	if id == "" || len(id) > 256 || strings.ContainsAny(id, `/\:`+"\x00") {
		return false
	}
	if id == "." || id == ".." || filepath.Base(id) != id {
		return false
	}
	return strings.HasSuffix(strings.ToLower(id), ".js")
}

func (s *userscriptStore) loadStateLocked() (map[string]bool, error) {
	state := map[string]bool{}
	data, err := os.ReadFile(s.stateFile)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("userscripts.json tidak valid: %w", err)
	}
	return state, nil
}

func (s *userscriptStore) saveStateLocked(state map[string]bool) error {
	if err := os.MkdirAll(filepath.Dir(s.stateFile), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.stateFile, data)
}

func (s *userscriptStore) list() ([]Userscript, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLocked()
}

func (s *userscriptStore) listLocked() ([]Userscript, error) {
	state, err := s.loadStateLocked()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Userscript{}, nil
	}
	if err != nil {
		return nil, err
	}
	scripts := make([]Userscript, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !validUserscriptID(name) || strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(s.dir, name)
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > profile.MaxUserscriptBytes {
			continue
		}
		raw, err := os.ReadFile(full)
		if err != nil || !utf8.Valid(raw) {
			continue
		}
		code := string(raw)
		meta := profile.ParseUserscriptMetadata(name, code)
		enabled, ok := state[name]
		if !ok {
			enabled = true
		}
		matches := meta.Matches
		if matches == nil {
			matches = []string{}
		}
		requires := meta.Requires
		if requires == nil {
			requires = []string{}
		}
		grants := meta.Grants
		if grants == nil {
			grants = []string{}
		}
		scripts = append(scripts, Userscript{
			ID:          name,
			Name:        meta.Name,
			Namespace:   meta.Namespace,
			Description: meta.Description,
			Version:     meta.Version,
			Author:      meta.Author,
			RunAt:       meta.RunAt,
			Matches:     matches,
			Requires:    requires,
			Grants:      grants,
			MatchesApp:  profile.MatchesWhatsApp(meta.Matches),
			Enabled:     enabled,
			Code:        code,
			UpdatedAt:   info.ModTime().UnixMilli(),
		})
	}
	slices.SortFunc(scripts, func(a, b Userscript) int {
		if c := cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return scripts, nil
}

func (s *userscriptStore) resolveRequiresLocked(requires []string, mainCode string) (string, error) {
	if len(requires) == 0 {
		return mainCode, nil
	}
	baseAbs, err := filepath.Abs(s.dir)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, rawReq := range requires {
		req := strings.TrimSpace(rawReq)
		if req == "" || strings.ContainsAny(req, "\x00") || strings.Contains(req, "://") || strings.HasPrefix(req, "//") {
			return "", fmt.Errorf("@require hanya mendukung jalur file lokal di dalam folder userscripts: %q", rawReq)
		}
		rel := filepath.FromSlash(req)
		if filepath.IsAbs(rel) {
			return "", fmt.Errorf("@require tidak boleh berupa jalur absolut: %q", rawReq)
		}
		targetAbs := filepath.Clean(filepath.Join(baseAbs, rel))
		within, err := filepath.Rel(baseAbs, targetAbs)
		if err != nil || within == "." || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("@require keluar dari folder userscripts: %q", rawReq)
		}
		if !strings.HasSuffix(strings.ToLower(targetAbs), ".js") {
			return "", fmt.Errorf("@require harus merujuk ke berkas .js: %q", rawReq)
		}
		info, err := os.Stat(targetAbs)
		if err != nil {
			return "", fmt.Errorf("gagal memuat @require %q: %w", rawReq, err)
		}
		if !info.Mode().IsRegular() || info.Size() > profile.MaxUserscriptBytes {
			return "", fmt.Errorf("berkas @require %q tidak valid atau melebihi batas ukuran", rawReq)
		}
		raw, err := os.ReadFile(targetAbs)
		if err != nil {
			return "", fmt.Errorf("gagal membaca @require %q: %w", rawReq, err)
		}
		if !utf8.Valid(raw) {
			return "", fmt.Errorf("berkas @require %q bukan teks UTF-8", rawReq)
		}
		out.WriteString("// --- @require ")
		out.WriteString(req)
		out.WriteString(" ---\n")
		out.Write(raw)
		out.WriteString("\n;\n")
	}
	out.WriteString(mainCode)
	resolved := out.String()
	if len(resolved) > profile.MaxResolvedUserscriptBytes {
		return "", errors.New("total ukuran userscript dan @require melebihi batas 4 MB")
	}
	return resolved, nil
}

func (s *userscriptStore) activePayloads() ([]profile.UserscriptPayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.listLocked()
	if err != nil {
		return nil, err
	}
	out := make([]profile.UserscriptPayload, 0, len(list))
	for _, item := range list {
		if !item.Enabled || !item.MatchesApp {
			continue
		}
		resolved, err := s.resolveRequiresLocked(item.Requires, item.Code)
		if err != nil {
			return nil, fmt.Errorf("userscript %q: %w", item.Name, err)
		}
		out = append(out, profile.UserscriptPayload{
			ID:          item.ID,
			Name:        item.Name,
			Version:     item.Version,
			Description: item.Description,
			Author:      item.Author,
			Namespace:   item.Namespace,
			RunAt:       item.RunAt,
			Grants:      item.Grants,
			Matches:     item.Matches,
			Code:        resolved,
			Hash:        profile.HashUserscript(resolved),
		})
	}
	return out, nil
}

func (s *userscriptStore) save(id, code string) ([]Userscript, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("kode userscript tidak boleh kosong")
	}
	if len(code) > profile.MaxUserscriptBytes {
		return nil, errors.New("ukuran userscript melebihi batas 1 MB")
	}
	if !utf8.ValidString(code) {
		return nil, errors.New("kode userscript harus berupa teks UTF-8")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	state, err := s.loadStateLocked()
	if err != nil {
		return nil, err
	}

	target := id
	if target == "" {
		meta := profile.ParseUserscriptMetadata("userscript.user.js", code)
		target = uniqueUserscriptFilename(s.dir, slugifyUserscriptName(meta.Name)+".user.js")
		state[target] = true
	} else {
		if !validUserscriptID(target) {
			return nil, errors.New("nama berkas userscript tidak valid")
		}
		if _, err := os.Stat(filepath.Join(s.dir, target)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, errors.New("userscript tidak ditemukan")
			}
			return nil, err
		}
	}
	if err := writeFileAtomic(filepath.Join(s.dir, target), []byte(code)); err != nil {
		return nil, err
	}
	if err := s.saveStateLocked(state); err != nil {
		return nil, err
	}
	return s.listLocked()
}

func (s *userscriptStore) importFiles(paths []string) ([]Userscript, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	state, err := s.loadStateLocked()
	if err != nil {
		return nil, err
	}
	for _, src := range paths {
		info, err := os.Stat(src)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			return nil, errors.New("tidak dapat mengimpor folder sebagai userscript")
		}
		if info.Size() > profile.MaxUserscriptBytes {
			return nil, fmt.Errorf("berkas %s melebihi batas 1 MB", filepath.Base(src))
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(raw) {
			return nil, fmt.Errorf("berkas %s bukan teks UTF-8 yang valid", filepath.Base(src))
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			return nil, fmt.Errorf("berkas %s kosong", filepath.Base(src))
		}
		target := uniqueUserscriptFilename(s.dir, normalizeImportedFilename(filepath.Base(src)))
		if err := writeFileAtomic(filepath.Join(s.dir, target), raw); err != nil {
			return nil, err
		}
		state[target] = true
	}
	if err := s.saveStateLocked(state); err != nil {
		return nil, err
	}
	return s.listLocked()
}

func (s *userscriptStore) setEnabled(id string, enabled bool) ([]Userscript, error) {
	if !validUserscriptID(id) {
		return nil, errors.New("ID userscript tidak valid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(filepath.Join(s.dir, id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("userscript tidak ditemukan")
		}
		return nil, err
	}
	state, err := s.loadStateLocked()
	if err != nil {
		return nil, err
	}
	state[id] = enabled
	if err := s.saveStateLocked(state); err != nil {
		return nil, err
	}
	return s.listLocked()
}

func (s *userscriptStore) remove(id string) ([]Userscript, error) {
	if !validUserscriptID(id) {
		return nil, errors.New("ID userscript tidak valid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(filepath.Join(s.dir, id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	state, stateErr := s.loadStateLocked()
	if stateErr != nil {
		return nil, stateErr
	}
	if _, ok := state[id]; ok {
		delete(state, id)
		if err := s.saveStateLocked(state); err != nil {
			return nil, err
		}
	}
	return s.listLocked()
}

func (s *userscriptStore) installExample(filename, code string) ([]Userscript, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	if filename == audioStickersExampleFilename {
		if err := s.installAudioStickerAssetsLocked(); err != nil {
			return nil, err
		}
	}
	state, err := s.loadStateLocked()
	if err != nil {
		return nil, err
	}
	target := filepath.Join(s.dir, filename)
	if err := writeFileAtomic(target, []byte(code)); err != nil {
		return nil, err
	}
	state[filename] = true
	if err := s.saveStateLocked(state); err != nil {
		return nil, err
	}
	return s.listLocked()
}

func (s *userscriptStore) installAudioStickerAssetsLocked() error {
	assetDir := filepath.Join(s.dir, "wa-audio-stickers")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(exampleuserscripts.AudioStickersAssets, "wa-audio-stickers", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		target := filepath.Join(s.dir, filepath.FromSlash(path))
		data, readErr := exampleuserscripts.AudioStickersAssets.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return writeFileAtomic(target, data)
	})
}

func normalizeImportedFilename(base string) string {
	lower := strings.ToLower(base)
	var stem, ext string
	switch {
	case strings.HasSuffix(lower, ".user.js"):
		stem = base[:len(base)-len(".user.js")]
		ext = ".user.js"
	case strings.HasSuffix(lower, ".js"):
		stem = base[:len(base)-len(".js")]
		ext = ".js"
	default:
		stem = base
		ext = ".user.js"
	}
	return slugifyUserscriptName(stem) + ext
}

func slugifyUserscriptName(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if (unicode.IsSpace(r) || r == '-' || r == '_' || r == '.') && !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "userscript"
	}
	if len(out) > 64 {
		out = strings.Trim(out[:64], "-")
	}
	return out
}

func uniqueUserscriptFilename(dir, filename string) string {
	if _, err := os.Stat(filepath.Join(dir, filename)); errors.Is(err, os.ErrNotExist) {
		return filename
	}
	lower := strings.ToLower(filename)
	ext := ".js"
	if strings.HasSuffix(lower, ".user.js") {
		ext = ".user.js"
	}
	stem := filename[:len(filename)-len(ext)]
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d%s", stem, os.Getpid(), ext)
}

func (w *Workspace) activeUserscriptPayloads() ([]profile.UserscriptPayload, error) {
	if w.userscripts == nil {
		return nil, nil
	}
	return w.userscripts.activePayloads()
}

func (w *Workspace) broadcastUserscripts(list []Userscript) error {
	if w.window != nil && !w.window.IsDestroyed() {
		_ = UserscriptsChanged.Emit(w.window, list)
	}
	payloads, err := w.activeUserscriptPayloads()
	if err != nil {
		return err
	}
	w.mu.Lock()
	procs := make([]*profileProcess, 0, len(w.processes))
	for _, p := range w.processes {
		procs = append(procs, p)
	}
	w.mu.Unlock()
	var errs []error
	for _, p := range procs {
		if err := w.sendCommand(p, profile.Command{Action: "userscripts", Userscripts: payloads}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Userscripts lists all userscripts stored in the workspace directory.
func (w *Workspace) Userscripts() ([]Userscript, error) {
	return w.userscripts.list()
}

// UserscriptsDirectory returns the folder where workspace userscripts live.
func (w *Workspace) UserscriptsDirectory() string {
	return w.userscripts.dir
}

// OpenUserscriptsDirectory ensures the userscripts folder exists and opens it in Explorer.
func (w *Workspace) OpenUserscriptsDirectory() error {
	if err := os.MkdirAll(w.userscripts.dir, 0o755); err != nil {
		return err
	}
	return mygo.Shell.OpenPath(w.userscripts.dir)
}

// ImportUserscripts opens a file picker for .user.js/.js files and imports them.
func (w *Workspace) ImportUserscripts() ([]Userscript, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      w.window,
		Title:       "Pilih berkas userscript",
		ButtonLabel: "Impor",
		Multiple:    true,
		Filters: []mygo.FileFilter{
			{Name: "Userscript (*.user.js, *.js)", Extensions: []string{"js"}},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return w.userscripts.list()
	}
	list, err := w.userscripts.importFiles(paths)
	if err != nil {
		return nil, err
	}
	return list, w.broadcastUserscripts(list)
}

// SaveUserscript creates a new userscript (when id is empty) or updates an existing one.
func (w *Workspace) SaveUserscript(id, code string) ([]Userscript, error) {
	list, err := w.userscripts.save(id, code)
	if err != nil {
		return nil, err
	}
	return list, w.broadcastUserscripts(list)
}

// SetUserscriptEnabled toggles whether a userscript runs in WhatsApp profiles.
func (w *Workspace) SetUserscriptEnabled(id string, enabled bool) ([]Userscript, error) {
	list, err := w.userscripts.setEnabled(id, enabled)
	if err != nil {
		return nil, err
	}
	return list, w.broadcastUserscripts(list)
}

// DeleteUserscript deletes a userscript file from the workspace.
func (w *Workspace) DeleteUserscript(id string) ([]Userscript, error) {
	list, err := w.userscripts.remove(id)
	if err != nil {
		return nil, err
	}
	return list, w.broadcastUserscripts(list)
}

// InstallExampleUserscript installs a bundled example userscript for WhatsApp Web.
func (w *Workspace) InstallExampleUserscript(id string) ([]Userscript, error) {
	if id == "" {
		id = exampleUserscriptFilename
	}
	code, ok := bundledExamples[id]
	if !ok {
		return nil, errors.New("contoh userscript tidak ditemukan")
	}
	list, err := w.userscripts.installExample(id, code)
	if err != nil {
		return nil, err
	}
	return list, w.broadcastUserscripts(list)
}
