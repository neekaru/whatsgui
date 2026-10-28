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
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const whatsappOrigin = "https://web.whatsapp.com"

//go:embed notification.js
var notificationScript string

type worker struct {
	conn *connection
	page *mygo.Page
	ctx  context.Context
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
	w := &worker{conn: conn, ctx: ctx}
	// MyGo v0.6.4 has one environment per process, keyed by application name.
	mygo.App.SetName(name)
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
	win := mygo.NewWindow(mygo.WindowOptions{
		Title: "WhatsApp", Width: 1100, Height: 700,
		Hidden: true, Frameless: true, SkipTaskbar: true,
		BackgroundColor: "#111b21",
		Page: mygo.PageOptions{
			PreloadScript:  notificationScript,
			TrustedOrigins: []string{whatsappOrigin},
		},
	})
	win.OnClosed(cancel)
	w.page = win.Page()
	w.page.SetPermissionHandler(requestPermission)
	w.page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		if isWhatsApp(e.URL) {
			return
		}
		e.PreventDefault()
		if e.UserInitiated {
			w.openExternal(e.URL)
		}
	})
	w.page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		w.openExternal(req.URL)
		return nil
	})
	w.page.OnDidFailLoad(func(err *mygo.LoadError) {
		w.report(fmt.Errorf("WhatsApp gagal dimuat: %v", err))
	})
	if err := w.conn.send(Message{Kind: "ready", HWND: win.NativeHandle()}); err != nil {
		log.Printf("kanal workspace terputus: %v", err)
		mygo.App.Quit()
		return
	}
	if err := w.page.LoadURL(whatsappOrigin + "/"); err != nil {
		w.report(err)
	}
	go w.readCommands()
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
		case "quit":
			return
		default:
			w.report(errors.New("profil sudah dimulai"))
			return
		}
	}
}

func (w *worker) report(err error) {
	if sendErr := w.conn.send(Message{Kind: "error", Body: err.Error()}); sendErr != nil {
		log.Printf("%v; kanal workspace terputus: %v", err, sendErr)
		mygo.App.Quit()
	}
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

func isWhatsApp(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "web.whatsapp.com" && u.User == nil
}

func requestPermission(req mygo.PermissionRequest) bool {
	if req.Origin != whatsappOrigin {
		return false
	}
	for _, permission := range req.Permissions {
		switch permission {
		case mygo.PermissionNotifications:
		case mygo.PermissionCamera, mygo.PermissionMicrophone:
			answer, err := mygo.Dialog.Message(mygo.MessageOptions{
				Type: "question", Title: "Izin WhatsApp",
				Message: "Izinkan WhatsApp mengakses " + string(permission) + "?",
				Buttons: []string{"Tolak", "Izinkan"}, DefaultButton: 0, CancelButton: 0,
			})
			if err != nil {
				log.Printf("permintaan izin WhatsApp: %v", err)
				return false
			}
			if answer.Button != 1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
