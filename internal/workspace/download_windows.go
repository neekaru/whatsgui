package workspace

import (
	"cmp"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"whatsappgui-new/internal/profile"
)

// Download states reported by the profile workers.
const (
	DownloadActive = "active"
	DownloadDone   = "done"
	DownloadFailed = "failed"
)

// DownloadItem is a file a profile downloaded, as the UI lists it.
type DownloadItem struct {
	ID          string `json:"id"`
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	State       string `json:"state"`
	// Received is how many bytes the profile wrote so far.
	Received int64 `json:"received"`
	// Total is the size of the file when known; 0 means the download reports
	// no total, so the UI must not show a percentage.
	Total     int64  `json:"total"`
	Error     string `json:"error,omitempty"`
	StartedAt int64  `json:"startedAt"`
}

// DownloadPrompt asks the user whether to start or cancel a download.
type DownloadPrompt struct {
	ID          string `json:"id"`
	ProfileID   string `json:"profileId"`
	ProfileName string `json:"profileName"`
	Name        string `json:"name"`
	Dir         string `json:"dir"`
	Path        string `json:"path"`
	Source      string `json:"source"`
}

// DownloadsChanged sends the downloads of every profile after any change.
var DownloadsChanged = mygo.NewEvent[[]DownloadItem]("downloadsChanged")

// DownloadPrompted asks the UI to confirm or cancel an incoming download.
var DownloadPrompted = mygo.NewEvent[DownloadPrompt]("downloadPrompted")

// mergeDownload records a download a profile reported and tells the UI.
func (w *Workspace) mergeDownload(profileID string, item profile.DownloadItem, dir string) {
	w.dlMu.Lock()
	if w.downloadItems == nil {
		w.downloadItems = map[string]DownloadItem{}
	}
	w.downloadItems[item.ID] = downloadItem(profileID, item)
	w.mergeDownloadDir(profileID, dir)
	w.dlMu.Unlock()
	go w.publishDownloads()
}

// mergeDownloads records the list a profile sends when its page opens.
func (w *Workspace) mergeDownloads(profileID string, msg profile.Message) {
	w.dlMu.Lock()
	if w.downloadItems == nil {
		w.downloadItems = map[string]DownloadItem{}
	}
	for _, item := range msg.Downloads {
		w.downloadItems[item.ID] = downloadItem(profileID, item)
	}
	w.mergeDownloadDir(profileID, msg.Dir)
	w.dlMu.Unlock()
	go w.publishDownloads()
}

// mergeDownloadDir keeps the folder of a profile; the caller holds w.dlMu.
func (w *Workspace) mergeDownloadDir(profileID, dir string) {
	if dir == "" {
		return
	}
	if w.downloadDirs == nil {
		w.downloadDirs = map[string]string{}
	}
	w.downloadDirs[profileID] = dir
}

func downloadItem(profileID string, item profile.DownloadItem) DownloadItem {
	return DownloadItem{
		ID: item.ID, ProfileID: profileID, Name: item.Name, Path: item.Path,
		State: item.State, Received: item.Received, Total: item.Total,
		Error: item.Error, StartedAt: item.StartedAt,
	}
}

// publishDownloads sends the list to the UI. It runs off the profile reader,
// which must never wait for the workspace lock.
func (w *Workspace) publishDownloads() {
	list := w.Downloads()
	if w.window == nil {
		return
	}
	_ = DownloadsChanged.Emit(w.window, list)
}

// Downloads lists the downloads of every profile, newest first.
func (w *Workspace) Downloads() []DownloadItem {
	w.mu.Lock()
	names := make(map[string]string, len(w.data.Tabs))
	for _, tab := range w.data.Tabs {
		names[tab.ID] = tab.Name
	}
	w.mu.Unlock()
	w.dlMu.Lock()
	list := make([]DownloadItem, 0, len(w.downloadItems))
	for id, item := range w.downloadItems {
		name, ok := names[item.ProfileID]
		if !ok {
			// The profile is gone; its downloads leave the list with it.
			delete(w.downloadItems, id)
			continue
		}
		item.ProfileName = name
		list = append(list, item)
	}
	w.dlMu.Unlock()
	slices.SortFunc(list, func(a, b DownloadItem) int {
		if a.StartedAt != b.StartedAt {
			return cmp.Compare(b.StartedAt, a.StartedAt)
		}
		return strings.Compare(a.ID, b.ID)
	})
	return list
}

// DownloadDirectory is the folder the active profile saves downloads to, or
// "" while the profile has not reported it yet.
func (w *Workspace) DownloadDirectory() string {
	w.mu.Lock()
	id := w.data.ActiveID
	w.mu.Unlock()
	w.dlMu.Lock()
	defer w.dlMu.Unlock()
	return w.downloadDirs[id]
}

// ChooseDownloadDirectory asks where the active profile saves downloads and
// returns the chosen folder, or "" when the user cancels. The choice is
// stored in the profile, so every later download uses it.
func (w *Workspace) ChooseDownloadDirectory() (string, error) {
	w.mu.Lock()
	id := w.data.ActiveID
	w.mu.Unlock()
	if id == "" {
		return "", errors.New("tidak ada tab aktif")
	}
	p, err := w.activeProcess()
	if err != nil {
		return "", err
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: w.window, Title: "Pilih folder unduhan", ButtonLabel: "Pilih folder",
		Directory: true, CreateDirectories: true, DefaultPath: w.DownloadDirectory(),
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	w.mu.Lock()
	still := w.data.ActiveID == id
	w.mu.Unlock()
	if !still {
		return "", errors.New("tab aktif berubah sebelum folder dipilih")
	}
	msg, err := w.request(p, profile.Command{Action: "setdownloaddir", Dir: paths[0]})
	if err != nil {
		return "", err
	}
	w.dlMu.Lock()
	w.mergeDownloadDir(id, msg.Dir)
	w.dlMu.Unlock()
	return msg.Dir, nil
}

// OpenDownload opens a finished download with the default application of its
// file type. The path comes from the profile that downloaded it, never from
// the page.
func (w *Workspace) OpenDownload(id string) error {
	item, err := w.findDownload(id)
	if err != nil {
		return err
	}
	if item.State != DownloadDone {
		return errors.New("unduhan belum selesai")
	}
	if info, err := os.Stat(item.Path); err != nil || info.IsDir() {
		return errors.New("berkas unduhan tidak ditemukan")
	}
	return mygo.Shell.OpenPath(item.Path)
}

// ShowDownload shows a download in Explorer, selected in its folder.
func (w *Workspace) ShowDownload(id string) error {
	item, err := w.findDownload(id)
	if err != nil {
		return err
	}
	if info, err := os.Stat(item.Path); err != nil || info.IsDir() {
		return errors.New("berkas unduhan tidak ditemukan")
	}
	cmd := exec.Command("explorer.exe", "/select,"+item.Path)
	if err := cmd.Start(); err != nil {
		// Explorer is not always on PATH; the folder is still worth opening.
		return mygo.Shell.OpenPath(filepath.Dir(item.Path))
	}
	go cmd.Wait()
	return nil
}

func (w *Workspace) findDownload(id string) (DownloadItem, error) {
	w.dlMu.Lock()
	defer w.dlMu.Unlock()
	item, ok := w.downloadItems[id]
	if !ok {
		return DownloadItem{}, errors.New("unduhan tidak ditemukan")
	}
	return item, nil
}

func (w *Workspace) queueDownloadPrompt(profileID string, p profile.DownloadPrompt) {
	profileName := ""
	w.mu.Lock()
	for _, tab := range w.data.Tabs {
		if tab.ID == profileID {
			profileName = tab.Name
			break
		}
	}
	w.mu.Unlock()

	prompt := DownloadPrompt{
		ID:          p.ID,
		ProfileID:   profileID,
		ProfileName: profileName,
		Name:        p.Name,
		Dir:         p.Dir,
		Path:        p.Path,
		Source:      p.Source,
	}
	w.dlMu.Lock()
	if w.downloadPrompts == nil {
		w.downloadPrompts = map[string]DownloadPrompt{}
	}
	w.downloadPrompts[prompt.ID] = prompt
	w.dlMu.Unlock()

	if w.window != nil {
		go func() {
			_ = DownloadPrompted.Emit(w.window, prompt)
		}()
	}
}

func (w *Workspace) clearDownloadPrompts(profileID string) {
	w.dlMu.Lock()
	for id, prompt := range w.downloadPrompts {
		if prompt.ProfileID == profileID {
			delete(w.downloadPrompts, id)
		}
	}
	w.dlMu.Unlock()
}

// PendingDownloads returns any download confirmations currently waiting for the user.
func (w *Workspace) PendingDownloads() []DownloadPrompt {
	w.dlMu.Lock()
	defer w.dlMu.Unlock()
	list := make([]DownloadPrompt, 0, len(w.downloadPrompts))
	for _, prompt := range w.downloadPrompts {
		list = append(list, prompt)
	}
	slices.SortFunc(list, func(a, b DownloadPrompt) int {
		return strings.Compare(a.ID, b.ID)
	})
	return list
}

// PickDownloadDirectory opens a folder picker for a pending download without
// changing the profile's default folder unless confirmed.
func (w *Workspace) PickDownloadDirectory(defaultDir string) (string, error) {
	if strings.TrimSpace(defaultDir) == "" {
		defaultDir = w.DownloadDirectory()
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:            w.window,
		Title:             "Pilih folder tujuan unduhan",
		ButtonLabel:       "Pilih folder",
		Directory:         true,
		CreateDirectories: true,
		DefaultPath:       defaultDir,
	})
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	return paths[0], nil
}

// ConfirmDownload accepts or cancels a pending download prompt.
func (w *Workspace) ConfirmDownload(id string, accept bool, dir, name string) error {
	w.dlMu.Lock()
	prompt, ok := w.downloadPrompts[id]
	if ok {
		delete(w.downloadPrompts, id)
	}
	w.dlMu.Unlock()
	if !ok {
		return errors.New("permintaan unduhan tidak ditemukan atau sudah kedaluwarsa")
	}

	w.mu.Lock()
	p := w.processes[prompt.ProfileID]
	w.mu.Unlock()
	if p == nil {
		if !accept {
			return nil
		}
		return errors.New("proses tab sudah tidak aktif")
	}

	return w.sendCommand(p, profile.Command{
		Action: "downloadreply",
		ID:     id,
		Accept: accept,
		Dir:    strings.TrimSpace(dir),
		Name:   strings.TrimSpace(name),
	})
}

// DownloadPreview returns a bounded data URL thumbnail for finished image downloads,
// or "" for non-image, active, or oversized files.
func (w *Workspace) DownloadPreview(id string) (string, error) {
	item, err := w.findDownload(id)
	if err != nil {
		return "", err
	}
	if item.State != DownloadDone || item.Path == "" {
		return "", nil
	}
	switch strings.ToLower(filepath.Ext(item.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
	default:
		return "", nil
	}
	const maxPreviewBytes = 5 << 20
	info, err := os.Stat(item.Path)
	if err != nil || info.IsDir() || info.Size() == 0 || info.Size() > maxPreviewBytes {
		return "", nil
	}
	data, err := os.ReadFile(item.Path)
	if err != nil {
		return "", nil
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp":
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
	default:
		return "", nil
	}
}
