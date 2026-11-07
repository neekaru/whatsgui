package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

// DNDSettings is the user's Do Not Disturb choice: a manual switch and an
// optional daily window in local time, which may cross midnight.
type DNDSettings struct {
	Enabled         bool   `json:"enabled"`
	ScheduleEnabled bool   `json:"scheduleEnabled"`
	Start           string `json:"start"`
	End             string `json:"end"`
}

// DNDChanged tells the UI that the settings, or the schedule's effect, changed.
var DNDChanged = mygo.NewEvent[DNDSettings]("dndChanged")

const (
	defaultDNDStart = "22:00"
	defaultDNDEnd   = "07:00"
)

func defaultDND() DNDSettings {
	return DNDSettings{Start: defaultDNDStart, End: defaultDNDEnd}
}

// validClock accepts exactly "HH:MM" in 24-hour local time.
func validClock(value string) bool {
	if len(value) != 5 || value[2] != ':' {
		return false
	}
	for i := range 5 {
		if i != 2 && (value[i] < '0' || value[i] > '9') {
			return false
		}
	}
	hour := int(value[0]-'0')*10 + int(value[1]-'0')
	minute := int(value[3]-'0')*10 + int(value[4]-'0')
	return hour < 24 && minute < 60
}

func validateDND(settings DNDSettings) error {
	if !validClock(settings.Start) || !validClock(settings.End) {
		return errors.New("jadwal jangan ganggu harus berformat HH:MM (00:00–23:59)")
	}
	return nil
}

// loadDND reads the saved settings; a missing file is the defaults, a broken
// one is an error so the user can fix or delete it rather than lose the choice.
func (w *Workspace) loadDND() error {
	settings := defaultDND()
	data, err := os.ReadFile(w.tray.file)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = defaultDND()
			err = fmt.Errorf("dnd.json tidak valid: %w", err)
		} else if err := validateDND(settings); err != nil {
			settings = defaultDND()
			err = fmt.Errorf("dnd.json tidak valid: %w", err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	w.tray.dnd = settings
	w.tray.active = dndWindow(settings, time.Now())
	return err
}

// GetDND returns the saved Do Not Disturb settings.
func (w *Workspace) GetDND() DNDSettings {
	w.tray.mu.Lock()
	defer w.tray.mu.Unlock()
	return w.tray.dnd
}

// SetDND saves the Do Not Disturb settings. While they are active, popups and
// their sound are dropped; the unread counts keep updating.
func (w *Workspace) SetDND(settings DNDSettings) error {
	settings.Start = strings.TrimSpace(settings.Start)
	settings.End = strings.TrimSpace(settings.End)
	if err := validateDND(settings); err != nil {
		return err
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(w.tray.file, data); err != nil {
		return err
	}
	w.tray.mu.Lock()
	w.tray.dnd = settings
	w.tray.active = dndWindow(settings, time.Now())
	w.tray.mu.Unlock()
	w.refreshTray()
	w.emitDND(settings)
	return nil
}

// DNDActive reports whether popups are suppressed now, by the manual switch or
// by the schedule.
func (w *Workspace) DNDActive() bool {
	return dndWindow(w.GetDND(), time.Now())
}

func dndWindow(settings DNDSettings, now time.Time) bool {
	if settings.Enabled {
		return true
	}
	if !settings.ScheduleEnabled {
		return false
	}
	clock := now.Format("15:04")
	switch {
	case settings.Start == settings.End:
		return true // a window that starts and ends together covers the day
	case settings.Start < settings.End:
		return clock >= settings.Start && clock < settings.End
	default:
		// Overnight: 22:00–07:00 runs from the start until midnight and
		// again after midnight until the end.
		return clock >= settings.Start || clock < settings.End
	}
}

// watchDND turns the schedule on and off while the app runs, and tells the UI.
func (w *Workspace) watchDND() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.tray.stop:
			return
		case <-ticker.C:
			w.tray.mu.Lock()
			settings := w.tray.dnd
			active := dndWindow(settings, time.Now())
			changed := active != w.tray.active
			w.tray.active = active
			w.tray.mu.Unlock()
			if changed {
				w.emitDND(settings)
			}
		}
	}
}

func (w *Workspace) emitDND(settings DNDSettings) {
	if w.window != nil && !w.window.IsDestroyed() {
		DNDChanged.Emit(w.window, settings)
	}
}
