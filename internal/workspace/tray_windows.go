package workspace

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
)

// trayState is the notification area icon and the Do Not Disturb choice. It is
// independent of the window: hiding the window keeps the icon and the counts.
type trayState struct {
	file string
	mu   sync.Mutex
	dnd  DNDSettings
	// active is the last reported effect of the settings, so the UI hears
	// about a schedule boundary exactly once.
	active bool

	icon   *mygo.Tray
	menu   *mygo.Menu
	unread int
	stop   chan struct{}
	// loadErr is a dnd.json that could not be read; the defaults are used.
	loadErr error
}

func newTrayState(file string) *trayState {
	return &trayState{file: file, dnd: defaultDND(), stop: make(chan struct{})}
}

// setupTray adds the icon once the application is ready, and starts the clock
// that turns the schedule on and off.
func (w *Workspace) setupTray() {
	if w.tray.loadErr != nil {
		WorkerError.Emit(w.window, "Jangan Ganggu memakai pengaturan bawaan: "+w.tray.loadErr.Error())
	}
	icon, err := trayIcon()
	if err != nil {
		// The window still works without an icon; say so instead of hiding it.
		WorkerError.Emit(w.window, "Ikon tray tidak tersedia: "+err.Error())
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{ID: "open", Label: "Buka WhatsApp", Click: func(*mygo.MenuItem, *mygo.Window) { w.showWindow() }},
		{ID: "unread", Label: unreadLabel(0), Disabled: true},
		mygo.Separator(),
		{ID: "dnd", Label: "Jangan Ganggu", Type: mygo.MenuItemCheckbox, Checked: w.GetDND().Enabled,
			Click: func(item *mygo.MenuItem, _ *mygo.Window) { w.toggleDND(item.IsChecked()) }},
		mygo.Separator(),
		{ID: "quit", Label: "Keluar", Click: func(*mygo.MenuItem, *mygo.Window) { w.Quit() }},
	})
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID: "workspace", Icon: icon, ToolTip: "WhatsApp", Menu: menu,
	})
	if err != nil {
		WorkerError.Emit(w.window, "Tray tidak tersedia: "+err.Error())
		return
	}
	w.tray.mu.Lock()
	w.tray.icon, w.tray.menu = tray, menu
	w.tray.mu.Unlock()
	w.refreshTray()
	go w.watchDND()
}

func (w *Workspace) shutdownTray() {
	w.tray.mu.Lock()
	tray := w.tray.icon
	w.tray.icon, w.tray.menu = nil, nil
	w.tray.mu.Unlock()
	if tray != nil {
		tray.Destroy()
	}
	select {
	case <-w.tray.stop:
	default:
		close(w.tray.stop)
	}
}

// showWindow brings the window back from the tray.
func (w *Workspace) showWindow() {
	if w.window == nil || w.window.IsDestroyed() {
		return
	}
	w.window.Show()
	w.window.Restore()
	w.window.Focus()
	w.place()
}

// hideWindow keeps the app and its counts running without a window.
func (w *Workspace) hideWindow() {
	if w.window == nil || w.window.IsDestroyed() {
		return
	}
	w.window.Hide()
}

// toggleDND flips the manual switch from the tray menu.
func (w *Workspace) toggleDND(enabled bool) {
	settings := w.GetDND()
	settings.Enabled = enabled
	if err := w.SetDND(settings); err != nil {
		WorkerError.Emit(w.window, "Menyimpan Jangan Ganggu: "+err.Error())
	}
}

// addUnread moves the tray total by delta, without touching the tab state.
func (w *Workspace) addUnread(delta int) {
	w.tray.mu.Lock()
	w.tray.unread = max(w.tray.unread+delta, 0)
	w.tray.mu.Unlock()
	w.refreshTray()
}

// setUnread stores what a profile's page reports and tells the UI.
func (w *Workspace) setUnread(id string, count int) {
	if w.closing.Load() {
		return
	}
	w.mu.Lock()
	i := w.index(id)
	if i < 0 {
		w.mu.Unlock()
		return
	}
	delta := count - w.data.Tabs[i].Unread
	if delta == 0 {
		w.mu.Unlock()
		return
	}
	w.data.Tabs[i].Unread = count
	state := w.snapshot()
	w.mu.Unlock()
	w.addUnread(delta)
	if w.window != nil && !w.closing.Load() {
		WorkspaceChanged.Emit(w.window, state)
	}
}

// refreshTray shows the total in the tooltip and the menu.
func (w *Workspace) refreshTray() {
	w.tray.mu.Lock()
	tray, menu, total := w.tray.icon, w.tray.menu, w.tray.unread
	w.tray.mu.Unlock()
	if tray == nil {
		return
	}
	tray.SetToolTip(trayTip(total))
	if item := menu.ItemByID("unread"); item != nil {
		item.SetLabel(unreadLabel(total))
	}
	if item := menu.ItemByID("dnd"); item != nil {
		item.SetChecked(w.GetDND().Enabled)
	}
}

func trayTip(total int) string {
	if total == 0 {
		return "WhatsApp"
	}
	return fmt.Sprintf("WhatsApp — %d pesan belum dibaca", total)
}

func unreadLabel(total int) string {
	if total == 0 {
		return "Tidak ada pesan belum dibaca"
	}
	return fmt.Sprintf("%d pesan belum dibaca", total)
}

// trayIcon reads the app's icon and shrinks it to the notification area's size:
// the shipped 1024 px PNG would make an unusable icon otherwise.
func trayIcon() ([]byte, error) {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(dir, "icon.png"))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	src, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("membaca icon.png: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaleTo(src, 32)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// scaleTo averages the source pixels of each destination pixel, so a large
// logo shrinks without the noise nearest-neighbour sampling would leave.
func scaleTo(src image.Image, size int) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return dst
	}
	for y := range size {
		y0 := b.Min.Y + y*b.Dy()/size
		y1 := max(b.Min.Y+(y+1)*b.Dy()/size, y0+1)
		for x := range size {
			x0 := b.Min.X + x*b.Dx()/size
			x1 := max(b.Min.X+(x+1)*b.Dx()/size, x0+1)
			var r, g, bl, a, n uint64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					c := color.NRGBAModel.Convert(src.At(xx, yy)).(color.NRGBA)
					r, g, bl, a, n = r+uint64(c.R), g+uint64(c.G), bl+uint64(c.B), a+uint64(c.A), n+1
				}
			}
			dst.SetNRGBA(x, y, color.NRGBA{uint8(r / n), uint8(g / n), uint8(bl / n), uint8(a / n)})
		}
	}
	return dst
}

// Quit shuts the profiles down and leaves the app, whatever the window shows.
func (w *Workspace) Quit() {
	w.quitOnce.Do(func() {
		go func() {
			w.close()
			mygo.App.Quit()
		}()
	})
}
