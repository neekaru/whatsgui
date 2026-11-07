package profile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
)

// Download states the workspace shows.
const (
	DownloadActive = "active"
	DownloadDone   = "done"
	DownloadFailed = "failed"
)

// maxDownloads is how many downloads a profile keeps in its list.
const maxDownloads = 100

// DownloadItem is a file this profile downloaded. Total is 0: the pinned MyGo
// reports no content length, so the UI shows the bytes received instead of a
// percentage.
type DownloadItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	State     string `json:"state"`
	Received  int64  `json:"received"`
	Total     int64  `json:"total"`
	Error     string `json:"error,omitempty"`
	StartedAt int64  `json:"startedAt"`
}

// downloadStore is what a profile saves: its folder and its downloads.
type downloadStore struct {
	Dir   string         `json:"dir"`
	Items []DownloadItem `json:"items"`
}

// downloadManager saves the downloads of one profile in its own folder and
// reports them to the workspace. Only the worker of that profile owns one.
type downloadManager struct {
	mu    sync.Mutex
	file  string
	dir   string
	items []DownloadItem
	// live holds the progress watcher of each running download.
	live map[string]chan struct{}
	send func(Message) error
}

func newDownloadManager(send func(Message) error, file string) *downloadManager {
	d := &downloadManager{file: file, live: map[string]chan struct{}{}, send: send}
	d.load()
	return d
}

// defaultDownloadDir is the user's Downloads folder, or the profile's own
// folder when Windows has none.
func defaultDownloadDir() string {
	if dir, err := mygo.App.Path(mygo.PathDownloads); err == nil && dir != "" {
		return dir
	}
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil && dir != "" {
		return filepath.Join(dir, "downloads")
	}
	return os.TempDir()
}

func (d *downloadManager) load() {
	d.dir = defaultDownloadDir()
	if d.file == "" {
		return
	}
	data, err := os.ReadFile(d.file)
	if err != nil {
		return
	}
	var stored downloadStore
	if json.Unmarshal(data, &stored) != nil {
		return
	}
	if stored.Dir != "" {
		d.dir = stored.Dir
	}
	for _, item := range stored.Items {
		if item.ID == "" || item.Path == "" {
			continue
		}
		// A download still active lost its page when the app closed; its file
		// stays as it was.
		if item.State == DownloadActive {
			item.State = DownloadFailed
			item.Error = "unduhan terputus saat aplikasi ditutup"
		}
		if info, err := os.Stat(item.Path); err == nil && !info.IsDir() {
			item.Received = info.Size()
		}
		d.items = append(d.items, item)
	}
	if len(d.items) > maxDownloads {
		d.items = d.items[len(d.items)-maxDownloads:]
	}
}

// save writes the store; the caller holds d.mu.
func (d *downloadManager) save() {
	if d.file == "" {
		return
	}
	data, err := json.MarshalIndent(downloadStore{Dir: d.dir, Items: d.items}, "", "  ")
	if err != nil {
		log.Printf("menyimpan daftar unduhan: %v", err)
		return
	}
	file, err := os.CreateTemp(filepath.Dir(d.file), "downloads-*.tmp")
	if err != nil {
		log.Printf("menyimpan daftar unduhan: %v", err)
		return
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		log.Printf("menyimpan daftar unduhan: %v", err)
		return
	}
	if err := file.Close(); err != nil {
		log.Printf("menyimpan daftar unduhan: %v", err)
		return
	}
	if err := os.Rename(file.Name(), d.file); err != nil {
		log.Printf("menyimpan daftar unduhan: %v", err)
	}
}

func (d *downloadManager) downloadDir() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dir
}

// setDir chooses where this profile saves downloads, for every later download.
func (d *downloadManager) setDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" || !filepath.IsAbs(dir) || strings.ContainsRune(dir, 0) || utf8.RuneCountInString(dir) > 1024 {
		return errors.New("folder unduhan tidak valid")
	}
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("folder unduhan tidak dapat dibuat: %w", err)
	}
	d.mu.Lock()
	d.dir = dir
	d.save()
	d.mu.Unlock()
	return nil
}

// track follows the downloads of every page of the profile: the main page and
// call popups alike, which share this policy.
func (d *downloadManager) track(page *mygo.Page) {
	page.OnWillDownload(d.begin)
	page.OnDownloadDone(d.finish)
}

// announce sends the saved downloads to the workspace when the page opens.
func (d *downloadManager) announce() {
	d.mu.Lock()
	msg := Message{Kind: "downloads", Dir: d.dir, Downloads: slices.Clone(d.items)}
	d.mu.Unlock()
	if d.send == nil {
		return
	}
	if err := d.send(msg); err != nil {
		log.Printf("melaporkan daftar unduhan: %v", err)
	}
}

// begin chooses the file a page download is written to. It runs on the page's
// UI thread, so it only touches the disk and the pipe.
func (d *downloadManager) begin(e *mygo.DownloadEvent) {
	name := safeFileName(e.SuggestedName)
	if !downloadAllowed(e.URL) {
		e.PreventDefault()
		d.fail(name, "unduhan dari sumber yang tidak dikenal ditolak")
		return
	}
	dir := d.downloadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.PreventDefault()
		d.fail(name, "folder unduhan tidak dapat dibuat: "+err.Error())
		return
	}
	// Reserve the name exclusively. The backend removes a file at Path before
	// it downloads, so reusing an existing name would delete that file.
	file, err := reserve(dir, name)
	if err != nil {
		e.PreventDefault()
		d.fail(name, "tidak dapat memilih nama berkas unduhan: "+err.Error())
		return
	}
	e.Path = file
	id := newDownloadID()
	item := DownloadItem{
		ID: id, Name: filepath.Base(file), Path: file,
		State: DownloadActive, StartedAt: time.Now().UnixMilli(),
	}
	stop := make(chan struct{})
	d.mu.Lock()
	d.items = append(d.items, item)
	if len(d.items) > maxDownloads {
		d.items = slices.Delete(d.items, 0, len(d.items)-maxDownloads)
	}
	d.live[id] = stop
	d.save()
	d.mu.Unlock()
	d.broadcast(item)
	go d.watch(id, file, stop)
}

// fail records a download the profile refused or could not start, so the
// workspace can show why nothing arrived.
func (d *downloadManager) fail(name, reason string) {
	item := DownloadItem{
		ID: newDownloadID(), Name: name, State: DownloadFailed,
		Error: reason, StartedAt: time.Now().UnixMilli(),
	}
	d.mu.Lock()
	d.items = append(d.items, item)
	if len(d.items) > maxDownloads {
		d.items = slices.Delete(d.items, 0, len(d.items)-maxDownloads)
	}
	d.save()
	d.mu.Unlock()
	d.broadcast(item)
}

// finish marks a download the page ended. A failed download keeps the bytes
// it wrote, so the user can still see what arrived.
func (d *downloadManager) finish(done *mygo.Download) {
	d.mu.Lock()
	i := slices.IndexFunc(d.items, func(item DownloadItem) bool {
		return item.Path == done.Path && item.State == DownloadActive
	})
	if i < 0 {
		d.mu.Unlock()
		return
	}
	item := &d.items[i]
	if stop, ok := d.live[item.ID]; ok {
		close(stop)
		delete(d.live, item.ID)
	}
	if done.Err != nil {
		item.State = DownloadFailed
		item.Error = done.Err.Error()
	} else {
		item.State = DownloadDone
	}
	if info, err := os.Stat(item.Path); err == nil && !info.IsDir() {
		item.Received = info.Size()
	}
	out := *item
	d.save()
	d.mu.Unlock()
	d.broadcast(out)
}

// watch reports the bytes on disk while a download runs.
func (d *downloadManager) watch(id, file string, stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			info, err := os.Stat(file)
			if err != nil || info.IsDir() {
				continue
			}
			d.progress(id, info.Size())
		}
	}
}

// progress reports what the backend has written so far. The pinned MyGo
// exposes no content length, so Total stays 0 rather than a made-up figure.
func (d *downloadManager) progress(id string, received int64) {
	d.mu.Lock()
	i := slices.IndexFunc(d.items, func(item DownloadItem) bool { return item.ID == id })
	if i < 0 || d.items[i].State != DownloadActive || d.items[i].Received == received {
		d.mu.Unlock()
		return
	}
	d.items[i].Received = received
	item := d.items[i]
	d.mu.Unlock()
	d.broadcast(item)
}

func (d *downloadManager) broadcast(item DownloadItem) {
	if d.send == nil {
		return
	}
	if err := d.send(Message{Kind: "download", Download: &item, Dir: d.downloadDir()}); err != nil {
		log.Printf("melaporkan unduhan ke workspace: %v", err)
	}
}

// downloadAllowed accepts what the WhatsApp pages serve. A page cannot make
// the app write files from an arbitrary origin.
func downloadAllowed(raw string) bool {
	if strings.HasPrefix(raw, "data:") {
		return true
	}
	if rest, ok := strings.CutPrefix(raw, "blob:"); ok {
		raw = rest
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "whatsapp.com" || strings.HasSuffix(host, ".whatsapp.com") ||
		host == "whatsapp.net" || strings.HasSuffix(host, ".whatsapp.net")
}

// safeFileName keeps a suggested name usable on Windows and inside one folder.
func safeFileName(name string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>:"|?*`, r) {
			return -1
		}
		return r
	}, name)
	// Windows drops trailing dots and spaces, which would change the name.
	name = strings.TrimRight(name, " .")
	if name == "" || name == "." || name == ".." {
		name = "unduhan"
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if r := []rune(base); len(r) > 80 {
		base = string(r[:80])
	}
	if r := []rune(ext); len(r) > 24 {
		ext = ""
	}
	name = base + ext
	switch strings.ToLower(base) {
	case "con", "prn", "aux", "nul",
		"com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9",
		"lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		name = "berkas-" + name
	}
	return name
}

// reserve creates the file of a download with its name, numbered like
// "name (2).ext" when it is taken. Creation is exclusive, so no file that
// already exists is ever chosen and later removed by the backend.
func reserve(dir, name string) (string, error) {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i <= 1000; i++ {
		file := filepath.Join(dir, name)
		if i > 1 {
			file = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		}
		f, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", errors.New("terlalu banyak berkas dengan nama yang sama")
}

func newDownloadID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
