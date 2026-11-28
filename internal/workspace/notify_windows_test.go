package workspace

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"whatsappgui-new/internal/profile"
)

func TestNotificationContentFallback(t *testing.T) {
	cases := []struct {
		name      string
		tab       string
		rawTitle  string
		rawBody   string
		wantTitle string
		wantBody  string
	}{
		{"normal", "Bisnis", "Budi", "Halo", "Bisnis — Budi", "Halo"},
		{"empty body", "Bisnis", "Panggilan masuk", "", "Bisnis — Panggilan masuk", "Pesan baru"},
		{"whitespace body", "Bisnis", "Budi", "   \n ", "Bisnis — Budi", "Pesan baru"},
		{"empty title", "Bisnis", "", "Halo", "Bisnis", "Halo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTitle, gotBody := notificationContent(tc.tab, tc.rawTitle, tc.rawBody)
			if gotTitle != tc.wantTitle || gotBody != tc.wantBody {
				t.Fatalf("notificationContent = (%q, %q), want (%q, %q)", gotTitle, gotBody, tc.wantTitle, tc.wantBody)
			}
		})
	}
}

func newTestNotifyWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	tab := newTab("Bisnis")
	w := &Workspace{
		data:             State{Tabs: []Tab{tab}, ActiveID: tab.ID},
		tray:             newTrayState(filepath.Join(t.TempDir(), "dnd.json")),
		notifyRetryDelay: time.Millisecond,
	}
	return w, tab.ID
}

func TestNotifyFallsBackToTextAndRetriesTransientFailure(t *testing.T) {
	w, id := newTestNotifyWorkspace(t)
	var calls []mygo.NotificationOptions
	w.showNotify = func(opts mygo.NotificationOptions, _ func()) (*mygo.Notification, error) {
		calls = append(calls, opts)
		if len(calls) == 1 {
			return nil, errors.New("mygo: cannot show the notification")
		}
		return nil, nil
	}

	err := w.notify(id, profile.Message{
		Title: "Budi",
		Body:  "",
		Token: "tok-1",
		Image: []byte{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("notify failed after image fallback: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if len(calls[0].Image) == 0 || len(calls[1].Image) != 0 {
		t.Fatalf("expected image fallback on second attempt: %+v", calls)
	}
	if calls[1].ID != id+"/tok-1" || calls[1].Body != "Pesan baru" {
		t.Fatalf("unexpected fallback options: %+v", calls[1])
	}

	// Verify transient failure without image recovers on the delayed retry.
	calls = nil
	w.showNotify = func(opts mygo.NotificationOptions, _ func()) (*mygo.Notification, error) {
		calls = append(calls, opts)
		if len(calls) == 1 {
			return nil, errors.New("mygo: cannot show the notification")
		}
		return nil, nil
	}
	if err := w.notify(id, profile.Message{Title: "Budi", Body: "Hai", Token: "tok-2"}); err != nil {
		t.Fatalf("notify failed after transient retry: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("transient retry calls = %d, want 2", len(calls))
	}
}

func TestNotifySuppressesRepeatedErrorBannerUntilRecovery(t *testing.T) {
	w, id := newTestNotifyWorkspace(t)
	fail := true
	w.showNotify = func(mygo.NotificationOptions, func()) (*mygo.Notification, error) {
		if fail {
			return nil, errors.New("mygo: cannot show the notification")
		}
		return nil, nil
	}

	if err := w.notify(id, profile.Message{Title: "A", Body: "1"}); err == nil {
		t.Fatal("expected first persistent failure to return an error")
	}
	if err := w.notify(id, profile.Message{Title: "A", Body: "2"}); err != nil {
		t.Fatalf("expected second consecutive failure to be suppressed, got %v", err)
	}

	fail = false
	if err := w.notify(id, profile.Message{Title: "A", Body: "3"}); err != nil {
		t.Fatalf("expected recovery to succeed: %v", err)
	}

	fail = true
	if err := w.notify(id, profile.Message{Title: "A", Body: "4"}); err == nil {
		t.Fatal("expected failure after recovery to report once again")
	}
}

func TestNotifyRespectsDND(t *testing.T) {
	w, id := newTestNotifyWorkspace(t)
	w.tray.dnd.Enabled = true
	called := false
	w.showNotify = func(mygo.NotificationOptions, func()) (*mygo.Notification, error) {
		called = true
		return nil, nil
	}
	if err := w.notify(id, profile.Message{Title: "A", Body: "1"}); err != nil {
		t.Fatalf("notify under DND returned error: %v", err)
	}
	if called {
		t.Fatal("notification shown while DND is active")
	}
}
