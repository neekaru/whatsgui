package workspace

import "github.com/egoist/mygo"

func Run() error {
	// Only the workspace takes the lock; --profile workers enter profile.Run.
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	workspace, err := openWorkspace()
	if err != nil {
		return err
	}
	mygo.Bind(workspace)
	mygo.App.OnSecondInstance(func([]string, string) {
		workspace.showWindow()
	})
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "WhatsApp Workspace", Width: 1280, Height: 860,
			MinWidth: 720, MinHeight: 540, StateKey: "workspace",
			BackgroundColor: "#10191d", URL: "/", Hidden: true,
			TitleBarStyle: mygo.TitleBarHidden, HideWindowButtons: true,
		})
		workspace.window = win
		// MyGo restores partially visible windows too. Fit before Show so an
		// oversized/restored window cannot leave its caption controls off-screen.
		if !win.IsMaximized() && !win.IsFullScreen() {
			bounds := win.Bounds()
			area := mygo.Screen.DisplayMatching(bounds).WorkArea
			if area.Width > 0 && area.Height > 0 {
				win.SetMinimumSize(min(720, area.Width), min(540, area.Height))
				win.SetBounds(fitWorkArea(bounds, area))
			}
		}
		workspace.window.OnClose(func(e *mygo.CloseEvent) {
			if workspace.closing.Load() {
				return
			}
			// Closing the window keeps the app running in the tray, so the
			// profiles keep counting messages; Quit in the tray menu leaves.
			e.PreventDefault()
			workspace.hideWindow()
		})
		go func() {
			workspace.mu.Lock()
			defer workspace.mu.Unlock()
			if err := workspace.cleanupDeleted(); err != nil {
				WorkerError.Emit(workspace.window, "Pembersihan data profil masih tertunda: "+err.Error())
			}
		}()
		win.Show()
		workspace.setupTray()
	})
	return mygo.App.Run()
}

func fitWorkArea(bounds, area mygo.Rectangle) mygo.Rectangle {
	bounds.Width = min(bounds.Width, area.Width)
	bounds.Height = min(bounds.Height, area.Height)
	bounds.X = max(area.X, min(bounds.X, area.X+area.Width-bounds.Width))
	bounds.Y = max(area.Y, min(bounds.Y, area.Y+area.Height-bounds.Height))
	return bounds
}
