package workspace

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"whatsappgui-new/internal/profile"
	"whatsappgui-new/internal/win32"
)

type Tab struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Unread is WhatsApp's aggregate of the chats with unread messages, as
	// its page title reports them. It is runtime state: loading tabs.json
	// clears it, so a count from the last run never shows.
	Unread int `json:"unread"`
}

type State struct {
	Tabs     []Tab  `json:"tabs"`
	ActiveID string `json:"activeId"`
}

type Bounds struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Scale  float64 `json:"scale"`
}

var WorkspaceChanged = mygo.NewEvent[State]("workspaceChanged")
var WorkerError = mygo.NewEvent[string]("workerError")

type profileProcess struct {
	cmd      *exec.Cmd
	input    io.WriteCloser
	hwnd     uintptr
	done     chan struct{}
	stopping atomic.Bool
	job      *win32.ProcessJob
	// One writer at a time: the encoder is not safe for concurrent use, and
	// bound methods, the UI and media requests all write to the pipe.
	writeMu sync.Mutex
	// pending holds the replies awaited by request, by request ID.
	pending sync.Map
}

// expect registers the channel that receives the reply to id.
func (p *profileProcess) expect(id string) chan profile.Message {
	ch := make(chan profile.Message, 1)
	p.pending.Store(id, ch)
	return ch
}

func (p *profileProcess) forget(id string) { p.pending.Delete(id) }

// replyTo returns the channel waiting for the reply to id, if any.
func (p *profileProcess) replyTo(id string) (chan profile.Message, bool) {
	value, ok := p.pending.Load(id)
	if !ok {
		return nil, false
	}
	ch, ok := value.(chan profile.Message)
	return ch, ok
}

type Workspace struct {
	mu            sync.Mutex
	data          State
	file          string
	theme         string
	window        *mygo.Window
	processes     map[string]*profileProcess
	bounds        Bounds
	closing       atomic.Bool
	quitOnce      sync.Once
	tray          *trayState
	notifications sync.Map
	deletedIDs    []string
	// dlMu guards the downloads of every profile, which the profile readers
	// update without waiting for mu.
	dlMu          sync.Mutex
	downloadItems map[string]DownloadItem
	downloadDirs  map[string]string
}

func openWorkspace() (*Workspace, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, err
	}
	w := &Workspace{
		file:      filepath.Join(dir, "tabs.json"),
		processes: make(map[string]*profileProcess),
		tray:      newTrayState(filepath.Join(dir, "dnd.json")),
	}
	if err := w.loadDND(); err != nil {
		// A broken dnd.json must not keep the app from starting: the defaults
		// are used and the user is told.
		w.tray.loadErr = err
	}
	data, err := os.ReadFile(w.file)
	if errors.Is(err, os.ErrNotExist) {
		w.data = State{Tabs: []Tab{newTab("WhatsApp 1")}}
		w.data.ActiveID = w.data.Tabs[0].ID
		return w, w.save(w.data)
	}
	if err != nil {
		return nil, err
	}
	var stored storedState
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("tabs.json tidak valid: %w", err)
	}
	w.data, w.deletedIDs = stored.State, stored.DeletedIDs
	// Counts are runtime state; a stored one is stale the moment the app quit.
	for i := range w.data.Tabs {
		w.data.Tabs[i].Unread = 0
	}
	seen := map[string]bool{}
	for _, tab := range w.data.Tabs {
		if !profile.ValidID(tab.ID) || seen[tab.ID] {
			return nil, errors.New("ID profil dalam tabs.json tidak valid/duplikat")
		}
		if _, err := cleanName(tab.Name); err != nil {
			return nil, err
		}
		seen[tab.ID] = true
	}
	if len(w.data.Tabs) == 0 {
		w.data.Tabs = []Tab{}
		w.data.ActiveID = ""
	} else if !seen[w.data.ActiveID] {
		return nil, errors.New("tab aktif dalam tabs.json tidak ditemukan")
	}
	for _, id := range w.deletedIDs {
		if !profile.ValidID(id) || seen[id] {
			return nil, errors.New("antrean penghapusan profil tidak valid")
		}
		seen[id] = true
	}
	return w, nil
}

func newTab(name string) Tab {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return Tab{ID: hex.EncodeToString(id[:]), Name: name}
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 60 || strings.ContainsAny(name, "\r\n\x00") {
		return "", errors.New("nama tab harus 1–60 karakter tanpa baris baru")
	}
	return name, nil
}

func (w *Workspace) save(state State) error {
	data, err := json.MarshalIndent(storedState{State: state, DeletedIDs: w.deletedIDs}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(w.file, data)
}

// writeFileAtomic replaces a settings file in one step: a crash mid-write
// leaves the old file, never a truncated one.
func writeFileAtomic(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*.tmp")
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
	return os.Rename(file.Name(), path)
}

func (w *Workspace) snapshot() State {
	return State{Tabs: slices.Clone(w.data.Tabs), ActiveID: w.data.ActiveID}
}
func (w *Workspace) index(id string) int {
	return slices.IndexFunc(w.data.Tabs, func(t Tab) bool { return t.ID == id })
}

func (w *Workspace) State() (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, tab := range w.data.Tabs {
		if err := w.start(tab.ID); err != nil {
			return w.snapshot(), err
		}
	}
	w.place()
	return w.snapshot(), nil
}

// SetTheme applies the workspace theme to all WhatsApp profiles.
func (w *Workspace) SetTheme(theme string) error {
	if theme != profile.ThemeSystem && theme != profile.ThemeLight && theme != profile.ThemeDark {
		return errors.New("tema tidak valid")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.theme = theme
	var errs []error
	for _, p := range w.processes {
		if err := w.sendCommand(p, profile.Command{Action: "theme", Theme: theme}); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (w *Workspace) Create(name string) (State, error) {
	name, err := cleanName(name)
	if err != nil {
		return State{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing.Load() {
		return State{}, errors.New("aplikasi sedang ditutup")
	}
	tab := newTab(name)
	state := w.snapshot()
	state.Tabs = append(state.Tabs, tab)
	state.ActiveID = tab.ID
	if err := w.start(tab.ID); err != nil {
		return State{}, err
	}
	if err := w.save(state); err != nil {
		w.stop(tab.ID)
		return State{}, err
	}
	w.data = state
	w.place()
	return w.snapshot(), nil
}

func (w *Workspace) Rename(id, name string) (State, error) {
	name, err := cleanName(name)
	if err != nil {
		return State{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	i := w.index(id)
	if i < 0 {
		return State{}, errors.New("tab tidak ditemukan")
	}
	state := w.snapshot()
	state.Tabs[i].Name = name
	if err := w.save(state); err != nil {
		return State{}, err
	}
	w.data = state
	return w.snapshot(), nil
}

func (w *Workspace) Activate(id string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.index(id) < 0 {
		return State{}, errors.New("tab tidak ditemukan")
	}
	if err := w.start(id); err != nil {
		return State{}, err
	}
	state := w.snapshot()
	state.ActiveID = id
	if err := w.save(state); err != nil {
		return State{}, err
	}
	w.data = state
	w.place()
	return w.snapshot(), nil
}

func (w *Workspace) Remove(id string) (State, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := w.index(id)
	if i < 0 {
		return State{}, errors.New("tab tidak ditemukan")
	}
	state := w.snapshot()
	removed := state.Tabs[i].Unread
	state.Tabs = append(state.Tabs[:i:i], state.Tabs[i+1:]...)
	if state.ActiveID == id {
		state.ActiveID = ""
		if len(state.Tabs) > 0 {
			state.ActiveID = state.Tabs[min(i, len(state.Tabs)-1)].ID
		}
	}
	// Persist deletion intent before touching login data. A locked cache may be
	// cleaned later, but must never resurrect the tab or lose its deletion request.
	previous := w.deletedIDs
	w.deletedIDs = append(slices.Clone(previous), id)
	if err := w.save(state); err != nil {
		w.deletedIDs = previous
		return State{}, err
	}
	w.data = state
	w.addUnread(-removed)
	w.stop(id)
	w.notifications.Range(func(key, value any) bool {
		if strings.HasPrefix(key.(string), id+"/") {
			value.(*mygo.Notification).Close()
			w.notifications.Delete(key)
		}
		return true
	})
	w.place()
	if err := w.cleanupDeleted(); err != nil && w.window != nil {
		WorkerError.Emit(w.window, "Tab dihapus; pembersihan data tertunda dan akan dicoba saat aplikasi dibuka: "+err.Error())
	}
	return w.snapshot(), nil
}

func (w *Workspace) Resize(bounds Bounds) error {
	for _, n := range []float64{bounds.X, bounds.Y, bounds.Width, bounds.Height, bounds.Scale} {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 32768 {
			return errors.New("ukuran viewport tidak valid")
		}
	}
	if bounds.Scale < 0.25 || bounds.Scale > 8 {
		return errors.New("skala viewport tidak valid")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bounds = bounds
	w.place()
	return nil
}

func (w *Workspace) Reload() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.data.ActiveID == "" {
		return errors.New("tidak ada tab aktif")
	}
	if err := w.start(w.data.ActiveID); err != nil {
		return err
	}
	w.place()
	return w.send(w.processes[w.data.ActiveID], "reload")
}

// ClearCache clears the downloaded web content of the active profile only.
// Cookies, local storage and IndexedDB are kept, so the tab reopens signed in.
func (w *Workspace) ClearCache() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing.Load() {
		return errors.New("aplikasi sedang ditutup")
	}
	id := w.data.ActiveID
	if id == "" {
		return errors.New("tidak ada tab aktif")
	}
	// WebView2 maps its cache files, so they are cleared with the worker down.
	w.stop(id)
	err := clearProfileCache(profileDirectory(profileName(id)))
	if startErr := w.start(id); startErr != nil {
		return errors.Join(err, startErr)
	}
	w.place()
	return err
}

// OpenDevTools opens the web inspector of the active profile, also in builds
// made for release.
func (w *Workspace) OpenDevTools() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.data.ActiveID == "" {
		return errors.New("tidak ada tab aktif")
	}
	if err := w.start(w.data.ActiveID); err != nil {
		return err
	}
	return w.send(w.processes[w.data.ActiveID], "devtools")
}

// MediaSettings is the microphone and camera choice of the active profile,
// with the devices its own WebView2 reports.
type MediaSettings struct {
	Audio   []MediaDevice `json:"audio"`
	Video   []MediaDevice `json:"video"`
	AudioID string        `json:"audioId"`
	VideoID string        `json:"videoId"`
}

// MediaDevice is a microphone or camera of the profile.
type MediaDevice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// MediaDevices lists the microphones and cameras of the active profile and the
// device it has chosen. The list comes from the profile's WebView2 page, which
// is the only place the real devices exist.
func (w *Workspace) MediaDevices() (MediaSettings, error) {
	p, err := w.activeProcess()
	if err != nil {
		return MediaSettings{}, err
	}
	msg, err := w.request(p, profile.Command{Action: "media"})
	if err != nil {
		return MediaSettings{}, err
	}
	devices := func(list []profile.MediaDevice) []MediaDevice {
		out := make([]MediaDevice, 0, len(list))
		for _, d := range list {
			out = append(out, MediaDevice{ID: d.ID, Label: d.Label})
		}
		return out
	}
	return MediaSettings{
		Audio: devices(msg.Audio), Video: devices(msg.Video),
		AudioID: msg.AudioID, VideoID: msg.VideoID,
	}, nil
}

// SetMediaDevices chooses the microphone and camera of the active profile.
// Empty IDs mean the system default. The choice is stored in the profile, so
// every call it makes, popups included, uses it.
func (w *Workspace) SetMediaDevices(audioID, videoID string) error {
	if len(audioID) > 256 || len(videoID) > 256 {
		return errors.New("ID perangkat tidak valid")
	}
	p, err := w.activeProcess()
	if err != nil {
		return err
	}
	_, err = w.request(p, profile.Command{Action: "setmedia", AudioID: audioID, VideoID: videoID})
	return err
}

// activeProcess returns the worker of the active tab, starting it if needed.
func (w *Workspace) activeProcess() (*profileProcess, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing.Load() {
		return nil, errors.New("aplikasi sedang ditutup")
	}
	id := w.data.ActiveID
	if id == "" {
		return nil, errors.New("tidak ada tab aktif")
	}
	if err := w.start(id); err != nil {
		return nil, err
	}
	p := w.processes[id]
	if p == nil {
		return nil, errors.New("profil aktif tidak berjalan")
	}
	return p, nil
}

// request sends a command and waits for the reply with its ID. The worker
// answers with Message.Err set when it fails.
func (w *Workspace) request(p *profileProcess, cmd profile.Command) (profile.Message, error) {
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		return profile.Message{}, err
	}
	cmd.ID = hex.EncodeToString(id[:])
	reply := p.expect(cmd.ID)
	defer p.forget(cmd.ID)
	if err := w.sendCommand(p, cmd); err != nil {
		return profile.Message{}, fmt.Errorf("mengirim permintaan ke profil: %w", err)
	}
	select {
	case msg := <-reply:
		if msg.Err != "" {
			return msg, errors.New(msg.Err)
		}
		return msg, nil
	case <-p.done:
		return profile.Message{}, errors.New("profil berhenti sebelum menjawab")
	case <-time.After(20 * time.Second):
		return profile.Message{}, errors.New("profil tidak menjawab permintaan")
	}
}

func profileName(id string) string { return mygo.App.Name() + "-profile-" + id }
func profileDirectory(name string) string {
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, name)
}

func (w *Workspace) start(id string) error {
	if w.closing.Load() {
		return errors.New("aplikasi sedang ditutup")
	}
	if p := w.processes[id]; p != nil {
		select {
		case <-p.done:
			w.stop(id)
		default:
			return nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--profile", profileName(id))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stderr = os.Stderr
	// Keep background WhatsApp audio enabled; do not inherit the shell's debugging port.
	cmd.Env = append(os.Environ(), "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--autoplay-policy=no-user-gesture-required")
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return err
	}
	if err := cmd.Start(); err != nil {
		input.Close()
		return err
	}
	p := &profileProcess{cmd: cmd, input: input, done: make(chan struct{})}
	job, err := win32.ContainProcess(cmd.Process.Pid)
	if err != nil {
		input.Close()
		cmd.Process.Kill()
		cmd.Wait()
		out.Close()
		return fmt.Errorf("mengisolasi proses profil: %w", err)
	}
	p.job = job
	if err := w.sendCommand(p, profile.Command{Action: "start", Theme: w.theme}); err != nil {
		job.Close()
		input.Close()
		cmd.Wait()
		out.Close()
		return err
	}
	ready := make(chan uintptr, 1)
	go func() {
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			var msg profile.Message
			if json.Unmarshal(scanner.Bytes(), &msg) != nil {
				continue
			}
			switch msg.Kind {
			case "ready":
				select {
				case ready <- msg.HWND:
				default:
				}
			case "notify":
				go func(m profile.Message) {
					if err := w.notify(id, m); err != nil {
						WorkerError.Emit(w.window, err.Error())
					}
				}(msg)
			case "unread":
				w.setUnread(id, msg.Count)
			case "error":
				WorkerError.Emit(w.window, msg.Body)
			case "external":
				if strings.HasPrefix(msg.URL, "https://") || strings.HasPrefix(msg.URL, "http://") {
					if err := mygo.Shell.OpenExternal(msg.URL); err != nil {
						WorkerError.Emit(w.window, err.Error())
					}
				}
			case "media":
				if ch, ok := p.replyTo(msg.ID); ok {
					select {
					case ch <- msg:
					default:
					}
				}
			case "download":
				if msg.Download != nil {
					w.mergeDownload(id, *msg.Download, msg.Dir)
				}
			case "downloads":
				if msg.ID != "" {
					// The reply to a folder change.
					if ch, ok := p.replyTo(msg.ID); ok {
						select {
						case ch <- msg:
						default:
						}
					}
					continue
				}
				w.mergeDownloads(id, msg)
			case "dismiss":
				if n, ok := w.notifications.LoadAndDelete(id + "/" + msg.Token); ok {
					n.(*mygo.Notification).Close()
				}
			}
		}
		cmd.Wait()
		close(p.done)
		if !w.closing.Load() && !p.stopping.Load() {
			// No page means no count: a stale number would outlive its tab.
			w.setUnread(id, 0)
			WorkerError.Emit(w.window, "Proses tab berhenti. Pilih tab atau Muat ulang untuk membukanya kembali.")
		}
	}()
	select {
	case hwnd := <-ready:
		p.hwnd = hwnd
		var attachErr error
		mygo.RunOnMain(func() { attachErr = win32.Attach(hwnd, w.window.NativeHandle()) })
		if attachErr != nil {
			input.Close()
			cmd.Process.Kill()
			job.Close()
			<-p.done
			return attachErr
		}
		w.processes[id] = p
		return nil
	case <-p.done:
		input.Close()
		job.Close()
		return errors.New("proses profil gagal dimulai")
	case <-time.After(30 * time.Second):
		input.Close()
		cmd.Process.Kill()
		job.Close()
		<-p.done
		return errors.New("waktu memulai profil habis")
	}
}

func (w *Workspace) send(p *profileProcess, action string) error {
	return w.sendCommand(p, profile.Command{Action: action})
}

func (w *Workspace) sendCommand(p *profileProcess, cmd profile.Command) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return json.NewEncoder(p.input).Encode(cmd)
}
func (w *Workspace) stop(id string) {
	p := w.processes[id]
	if p == nil {
		return
	}
	p.stopping.Store(true)
	delete(w.processes, id)
	w.send(p, "quit")
	p.input.Close()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		// Terminate the entire contained tree below, not only its worker.
	}
	if err := p.job.Close(); err != nil {
		p.cmd.Process.Kill()
		if w.window != nil {
			WorkerError.Emit(w.window, err.Error())
		}
	}
	<-p.done
}
func (w *Workspace) place() {
	if len(w.processes) == 0 {
		return
	}
	mygo.RunOnMain(func() {
		b := w.bounds
		pixel := func(n float64) int { return int(math.Round(n * b.Scale)) }
		for id, p := range w.processes {
			win32.Place(p.hwnd, pixel(b.X), pixel(b.Y), pixel(b.Width), pixel(b.Height), id == w.data.ActiveID)
		}
	})
}
func (w *Workspace) close() {
	if !w.closing.CompareAndSwap(false, true) {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for id := range w.processes {
		w.stop(id)
	}
	w.shutdownTray()
}

func (w *Workspace) notify(id string, msg profile.Message) error {
	w.mu.Lock()
	i := w.index(id)
	if i < 0 {
		w.mu.Unlock()
		return errors.New("tab notifikasi tidak ditemukan")
	}
	name := w.data.Tabs[i].Name
	w.mu.Unlock()
	// Do Not Disturb is the user's own choice: counts keep coming from the
	// page, only the popup and its sound are dropped.
	if w.DNDActive() {
		return nil
	}
	if !mygo.NotificationsSupported() {
		return errors.New("notifikasi sistem tidak tersedia")
	}
	n := mygo.NewNotification(mygo.NotificationOptions{Title: name + " — " + msg.Title, Body: msg.Body, Silent: msg.Silent})
	key := id + "/" + msg.Token
	if msg.Token != "" {
		w.notifications.Store(key, n)
	}
	n.OnClick(func() {
		n.Close()
		w.notifications.Delete(key)
		go func() {
			state, err := w.Activate(id)
			if err != nil {
				WorkerError.Emit(w.window, err.Error())
				return
			}
			w.mu.Lock()
			if p := w.processes[id]; p != nil && msg.Token != "" {
				if err := w.sendCommand(p, profile.Command{Action: "click", Token: msg.Token}); err != nil {
					WorkerError.Emit(w.window, "Gagal membuka percakapan dari notifikasi: "+err.Error())
				}
			}
			w.mu.Unlock()
			w.window.Show()
			w.window.Restore()
			w.window.Focus()
			WorkspaceChanged.Emit(w.window, state)
		}()
	})
	if err := n.Show(); err != nil {
		w.notifications.Delete(key)
		return err
	}
	return nil
}
