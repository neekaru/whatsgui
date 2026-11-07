package workspace

import (
	"testing"

	"whatsappgui-new/internal/profile"
)

func TestDownloadsAreMergedPerProfile(t *testing.T) {
	w := &Workspace{data: State{Tabs: []Tab{{ID: "a", Name: "Kerja"}, {ID: "b", Name: "Pribadi"}}, ActiveID: "a"}}
	w.mergeDownload("a", profile.DownloadItem{
		ID: "1", Name: "laporan.pdf", Path: `C:\dl\laporan.pdf`, State: profile.DownloadActive,
		Received: 10, StartedAt: 100,
	}, `C:\dl`)
	w.mergeDownload("b", profile.DownloadItem{
		ID: "2", Name: "foto.jpg", Path: `C:\dl\foto.jpg`, State: profile.DownloadDone,
		Received: 20, Total: 0, StartedAt: 200,
	}, `D:\unduhan`)

	list := w.Downloads()
	if len(list) != 2 {
		t.Fatalf("unduhan = %+v", list)
	}
	// Newest first.
	if list[0].ID != "2" || list[0].ProfileName != "Pribadi" || list[1].ProfileName != "Kerja" {
		t.Fatalf("urutan/nama profil salah: %+v", list)
	}

	// A later report of the same download replaces it, it is not added twice.
	w.mergeDownload("a", profile.DownloadItem{
		ID: "1", Name: "laporan.pdf", Path: `C:\dl\laporan.pdf`, State: profile.DownloadFailed,
		Received: 30, Error: "terputus", StartedAt: 100,
	}, "")
	if list := w.Downloads(); len(list) != 2 {
		t.Fatalf("unduhan ganda: %+v", list)
	}
	item, err := w.findDownload("1")
	if err != nil || item.State != profile.DownloadFailed || item.Received != 30 || item.Error != "terputus" {
		t.Fatalf("perubahan unduhan hilang: %+v %v", item, err)
	}

	// The folder of a profile is kept for the UI.
	if got := w.DownloadDirectory(); got != `C:\dl` {
		t.Fatalf("folder unduhan = %q", got)
	}

	// Downloads of a removed profile leave the list.
	w.data.Tabs = w.data.Tabs[:1]
	if list := w.Downloads(); len(list) != 1 || list[0].ID != "1" {
		t.Fatalf("unduhan profil terhapus masih tampil: %+v", list)
	}
	if _, err := w.findDownload("2"); err == nil {
		t.Fatal("unduhan profil terhapus masih dapat dibuka")
	}
}
