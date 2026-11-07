package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeFileName(t *testing.T) {
	long := strings.Repeat("a", 200) + ".pdf"
	for _, c := range []struct{ in, want string }{
		{"laporan.pdf", "laporan.pdf"},
		{`..\..\evil.exe`, "evil.exe"},
		{"../../evil.exe", "evil.exe"},
		{"CON", "berkas-CON"},
		{"con.txt", "berkas-con.txt"},
		{"LPT1", "berkas-LPT1"},
		{`a<b>c:d"e|f?g*.txt`, "abcdefg.txt"},
		{"nama.", "nama"},
		{"  nama  .  ", "nama"},
		{"a\x00b.txt", "ab.txt"},
		{"", "unduhan"},
		{".", "unduhan"},
		{"..", "unduhan"},
		{"...", "unduhan"},
		{"\x00\x01", "unduhan"},
		{long, strings.Repeat("a", 80) + ".pdf"},
	} {
		if got := safeFileName(c.in); got != c.want {
			t.Fatalf("safeFileName(%q) = %q, ingin %q", c.in, got, c.want)
		}
	}
}

// A file that already has the suggested name must never be chosen: the pinned
// MyGo removes a file at the download path before it writes the download.
func TestReserveKeepsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "laporan.pdf")
	if err := os.WriteFile(existing, []byte("penting"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := reserve(dir, "laporan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first) != "laporan (2).pdf" {
		t.Fatalf("nama berkas = %q", filepath.Base(first))
	}
	if data, err := os.ReadFile(existing); err != nil || string(data) != "penting" {
		t.Fatalf("berkas lama berubah: %q %v", data, err)
	}
	if info, err := os.Stat(first); err != nil || info.Size() != 0 {
		t.Fatalf("berkas unduhan tidak disiapkan: %v %v", info, err)
	}
	second, err := reserve(dir, "laporan.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(second) != "laporan (3).pdf" {
		t.Fatalf("nama berkas kedua = %q", filepath.Base(second))
	}
}

func TestDownloadAllowed(t *testing.T) {
	for raw, allowed := range map[string]bool{
		"https://web.whatsapp.com/abc":            true,
		"https://mmg.whatsapp.net/v/t62/x":        true,
		"https://webtp.whatsapp.net/pdf-viewer/":  true,
		"blob:https://web.whatsapp.com/1234-abcd": true,
		"data:application/pdf;base64,AAAA":        true,
		"https://evil.test/x":                     false,
		"http://web.whatsapp.com/x":               false,
		"https://web.whatsapp.com.evil.test/x":    false,
		"https://user@web.whatsapp.com/x":         false,
		"https://web.whatsapp.com:444/x":          false,
		"blob:https://evil.test/x":                false,
		"javascript:alert(1)":                     false,
		"file:///C:/Windows/win.ini":              false,
		"":                                        false,
	} {
		if got := downloadAllowed(raw); got != allowed {
			t.Fatalf("downloadAllowed(%q) = %v, ingin %v", raw, got, allowed)
		}
	}
}
