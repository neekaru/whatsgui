package profile

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

const whatsappOrigin = "https://web.whatsapp.com"

//go:embed appearance.js
var appearanceScript string

//go:embed notification.js
var notificationScript string

//go:embed media.js
var mediaScript string

//go:embed status_background.js
var statusBackgroundScript string

//go:embed unread.js
var unreadScript string

// preloadScript runs in every WhatsApp page: the main window and call popups.
// appearanceScript comes first: WhatsApp's bootloader reads its theme from
// localStorage before any later script runs.
var preloadScript = appearanceScript + "\n" + notificationScript + "\n" + mediaScript + "\n" + statusBackgroundScript + "\n" + unreadScript

type worker struct {
	conn      *connection
	page      *mygo.Page
	ctx       context.Context
	downloads *downloadManager
	themeMu   sync.Mutex
	theme     string
}

// mediaList is what the injected script enumerates: microphones and cameras.
type mediaList struct {
	Audio   []MediaDevice `json:"audio"`
	Video   []MediaDevice `json:"video"`
	AudioID string        `json:"audioId"`
	VideoID string        `json:"videoId"`
}

func Run(name string) error {
	_, id, ok := strings.Cut(name, "-profile-")
	if !ok || !ValidID(id) || strings.ContainsAny(name, `/\\`) {
		return errors.New("nama profil tidak valid")
	}
	conn := newConnection(os.Stdin, os.Stdout)
	// WebView2 must not spawn before the parent assigns this process to its job.
	start, err := conn.receive()
	if err != nil {
		return fmt.Errorf("memulai profil: %w", err)
	}
	if start.Action != "start" {
		return errors.New("perintah pertama profil harus start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer os.Stdin.Close()
	w := &worker{conn: conn, ctx: ctx, theme: start.Theme}
	// MyGo v0.6.4 has one environment per process, keyed by application name.
	mygo.App.SetName(name)
	// Downloads are the profile's own: its list is saved next to its data.
	dataDir, _ := mygo.App.Path(mygo.PathUserData)
	store := ""
	if dataDir != "" {
		store = filepath.Join(dataDir, "downloads.json")
	}
	w.downloads = newDownloadManager(conn.send, store)
	// Windows creates one WebView2 environment per process, from the app's
	// first language, when the first window is made: this has to run before,
	// and changing it later needs a new worker. The user's languages follow
	// as fallbacks, so Indonesian leads the requests.
	mygo.App.SetLanguages("id")
	mygo.Bind(&ProfileBridge{conn: conn})
	mygo.App.WhenReady(func() { w.open(cancel) })
	return mygo.App.Run()
}

func (w *worker) open(cancel context.CancelFunc) {
	// Popup pages need the same origin and permission policy before navigation.
	mygo.App.OnWindowCreated(func(win *mygo.Window) { w.configurePage(win.Page()) })
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "WhatsApp", Width: 1100, Height: 700,
		Hidden: true, Frameless: true, SkipTaskbar: true,
		BackgroundColor: "#111b21",
		Page: mygo.PageOptions{
			PreloadScript:  preloadScript,
			TrustedOrigins: []string{whatsappOrigin},
			// The workspace offers the inspector to users of release builds.
			DevTools: mygo.DevToolsEnabled,
		},
	})
	win.OnClosed(cancel)
	w.page = win.Page()
	w.page.OnDidFinishLoad(func() { go w.applyTheme() })
	w.page.OnDidFailLoad(func(err *mygo.LoadError) {
		w.report(fmt.Errorf("WhatsApp gagal dimuat: %v", err))
	})
	if err := w.conn.send(Message{Kind: "ready", HWND: win.NativeHandle()}); err != nil {
		log.Printf("kanal workspace terputus: %v", err)
		mygo.App.Quit()
		return
	}
	w.downloads.announce()
	if err := w.page.LoadURL(whatsappOrigin + "/"); err != nil {
		w.report(err)
	}
	go w.readCommands()
}

func (w *worker) configurePage(page *mygo.Page) {
	page.SetPermissionHandler(requestPermission)
	// The main page and call popups share this download policy.
	w.downloads.track(page)
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		if isWhatsApp(e.URL) || (w.page != nil && page != w.page && isAppNavigation(e.URL)) {
			return
		}
		e.PreventDefault()
		if e.UserInitiated {
			w.openExternal(e.URL)
		}
	})
	page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		if isAppNavigation(req.URL) {
			// MyGo adopts the popup into the worker's WebView2 environment,
			// preserving its session and window.opener. Calls run in such a
			// popup, so it needs the same preload and origin policy.
			return &mygo.WindowOptions{
				Title: "WhatsApp", Width: req.Width, Height: req.Height,
				Page: mygo.PageOptions{
					PreloadScript:  preloadScript,
					TrustedOrigins: []string{whatsappOrigin},
				},
			}
		}
		w.openExternal(req.URL)
		return nil
	})
}

func (w *worker) readCommands() {
	defer mygo.App.Quit()
	for {
		cmd, err := w.conn.receive()
		if err != nil {
			if !errors.Is(err, io.EOF) && w.ctx.Err() == nil {
				w.report(err)
			}
			return
		}
		switch cmd.Action {
		case "reload":
			w.page.Reload()
		case "theme":
			w.themeMu.Lock()
			w.theme = cmd.Theme
			w.themeMu.Unlock()
			w.applyTheme()
		case "click":
			token, err := json.Marshal(cmd.Token)
			if err != nil {
				w.report(err)
				continue
			}
			ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
			_, err = w.page.EvalContext(ctx, "window.__workspaceNotificationClick?.("+string(token)+")")
			cancel()
			if err != nil && w.ctx.Err() == nil {
				w.report(err)
			}
		case "media":
			// Enumeration may wait for a permission prompt, so it must not
			// hold up reload or quit.
			go w.answerMedia(cmd)
		case "setmedia":
			go w.answerSetMedia(cmd)
		case "setdownloaddir":
			// The folder is this profile's own; the workspace only chose it.
			msg := Message{Kind: "downloads", ID: cmd.ID}
			if err := w.downloads.setDir(cmd.Dir); err != nil {
				msg.Err = err.Error()
			} else {
				msg.Dir = w.downloads.downloadDir()
			}
			w.reply(msg)
		case "quit":
			return
		case "devtools":
			// WebView2 opens the inspector in a window of its own and reports
			// success by setting the flag; a closed flag means the runtime
			// refused, which the user has to see.
			w.page.OpenDevTools()
			if !w.page.IsDevToolsOpened() {
				w.report(errors.New("DevTools tidak dapat dibuka"))
			}
		default:
			w.report(errors.New("profil sudah dimulai"))
			return
		}
	}
}
func (w *worker) applyTheme() {
	w.themeMu.Lock()
	defer w.themeMu.Unlock()
	if w.theme == "" {
		return
	}
	theme, _ := json.Marshal(w.theme)
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()
	changed, err := w.page.EvalContext(ctx, "window.__workspaceApplyAppearance?.({theme: "+string(theme)+"})")
	if err != nil {
		if w.ctx.Err() == nil {
			w.report(fmt.Errorf("menerapkan tema WhatsApp: %w", err))
		}
		return
	}
	if changed == true {
		w.page.Reload()
	}
}

func (w *worker) report(err error) {
	if sendErr := w.conn.send(Message{Kind: "error", Body: err.Error()}); sendErr != nil {
		log.Printf("%v; kanal workspace terputus: %v", err, sendErr)
		mygo.App.Quit()
	}
}

// reply answers a media request. A broken channel ends the worker, as report does.
func (w *worker) reply(msg Message) {
	if err := w.conn.send(msg); err != nil {
		log.Printf("kanal workspace terputus: %v", err)
		mygo.App.Quit()
	}
}

// enumerate asks the page for the real WebView2 devices. Only the injected
// script can list them: the workspace process has no WebView2 environment.
func (w *worker) enumerate() (mediaList, error) {
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()
	for {
		ready, err := w.page.EvalContext(ctx, "location.origin === 'https://web.whatsapp.com' && typeof window.__workspaceMediaDevices === 'function'")
		if err == nil && ready == true {
			break
		}
		select {
		case <-ctx.Done():
			return mediaList{}, errors.New("halaman WhatsApp belum siap untuk membaca perangkat")
		case <-time.After(200 * time.Millisecond):
		}
	}
	value, err := w.page.EvalContext(ctx, "window.__workspaceMediaDevices()")
	if err != nil {
		return mediaList{}, fmt.Errorf("membaca perangkat media: %w", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return mediaList{}, err
	}
	var list mediaList
	if err := json.Unmarshal(data, &list); err != nil {
		return mediaList{}, fmt.Errorf("daftar perangkat media tidak valid: %w", err)
	}
	return list, nil
}

func hasDevice(devices []MediaDevice, id string) bool {
	return slices.ContainsFunc(devices, func(d MediaDevice) bool { return d.ID == id })
}

func (w *worker) answerMedia(cmd Command) {
	reply := Message{Kind: "media", ID: cmd.ID}
	list, err := w.enumerate()
	if err != nil {
		reply.Err = err.Error()
		w.reply(reply)
		return
	}
	reply.Audio, reply.Video = list.Audio, list.Video
	reply.AudioID, reply.VideoID = list.AudioID, list.VideoID
	w.reply(reply)
}

func (w *worker) answerSetMedia(cmd Command) {
	reply := Message{Kind: "media", ID: cmd.ID, AudioID: cmd.AudioID, VideoID: cmd.VideoID}
	list, err := w.enumerate()
	if err != nil {
		reply.Err = err.Error()
		w.reply(reply)
		return
	}
	if cmd.AudioID != "" && !hasDevice(list.Audio, cmd.AudioID) {
		reply.Err = "mikrofon yang dipilih tidak tersedia"
		w.reply(reply)
		return
	}
	if cmd.VideoID != "" && !hasDevice(list.Video, cmd.VideoID) {
		reply.Err = "kamera yang dipilih tidak tersedia"
		w.reply(reply)
		return
	}
	audio, _ := json.Marshal(cmd.AudioID)
	video, _ := json.Marshal(cmd.VideoID)
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()
	value, err := w.page.EvalContext(ctx, "window.__workspaceSetMediaDevices("+string(audio)+","+string(video)+")")
	if err != nil || value != true {
		reply.Err = fmt.Sprintf("menyimpan pilihan perangkat: %v", err)
		w.reply(reply)
		return
	}
	w.reply(reply)
}

func (w *worker) openExternal(raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return
	}
	// Launch outside the worker job so deleting a tab cannot close the user's browser.
	if err := w.conn.send(Message{Kind: "external", URL: raw}); err != nil {
		w.report(err)
	}
}

// Keep the tab on WhatsApp; auxiliary origins are allowed in popups and frames.
func isWhatsApp(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil &&
		(u.Port() == "" || u.Port() == "443") && strings.EqualFold(u.Hostname(), "web.whatsapp.com")
}

func isAppNavigation(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "web.whatsapp.com", "webtp.whatsapp.net", "flows.whatsapp.net":
		return true
	default:
		return false
	}
}

func requestPermission(req mygo.PermissionRequest) bool {
	if req.Origin != whatsappOrigin {
		return false
	}
	for _, permission := range req.Permissions {
		switch permission {
		case mygo.PermissionNotifications, mygo.PermissionMicrophone, mygo.PermissionCamera:
		default:
			return false
		}
	}
	return true
}
