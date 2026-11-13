package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestOrphanBrowserCleanup(t *testing.T) {
	if os.Getenv("TEST_ORPHAN_CHILD") == "1" {
		path, err := windows.UTF16PtrFromString(os.Getenv("TEST_ORPHAN_FILE"))
		if err != nil {
			t.Fatal(err)
		}
		h, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer windows.CloseHandle(h)
		if err := os.WriteFile(os.Getenv("TEST_ORPHAN_READY"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Second)
		return
	}
	if os.Getenv("TEST_ORPHAN_LAUNCHER") == "1" {
		cmd := exec.Command(os.Getenv("TEST_ORPHAN_EXE"), "-test.run=^TestOrphanBrowserCleanup$", "--", "--user-data-dir="+os.Getenv("TEST_ORPHAN_DATA"))
		cmd.Env = append(os.Environ(), "TEST_ORPHAN_CHILD=1")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			if _, err := os.Stat(os.Getenv("TEST_ORPHAN_READY")); err == nil {
				return
			}
			if time.Now().After(deadline) {
				cmd.Process.Kill()
				t.Fatal("browser helper did not start")
			}
			time.Sleep(25 * time.Millisecond)
		}
	}

	dir := t.TempDir()
	webview := filepath.Join(dir, "profile", "WebView2")
	base := filepath.Join(webview, "EBWebView", "Default")
	locked := filepath.Join(base, "Cache", "journal.baj")
	cookie := filepath.Join(base, "Network", "Cookies")
	for _, file := range []string{locked, cookie} {
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("session"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(dir, "msedgewebview2.exe")
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, data, 0700); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(dir, "ready")
	// A browser with a live parent must never be terminated as an orphan.
	live := exec.Command(exe, "-test.run=^TestOrphanBrowserCleanup$", "--", "--user-data-dir="+filepath.Join(webview, "EBWebView"))
	live.Env = append(os.Environ(), "TEST_ORPHAN_CHILD=1", "TEST_ORPHAN_FILE="+locked, "TEST_ORPHAN_READY="+ready)
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	defer live.Process.Kill()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("live browser helper did not start")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err := stopOrphanBrowser(webview, "orphan-test"); err != nil {
		t.Fatal(err)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(live.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	status, err := windows.WaitForSingleObject(handle, 0)
	windows.CloseHandle(handle)
	if err != nil || status != uint32(windows.WAIT_TIMEOUT) {
		t.Fatalf("live browser stopped: %v", err)
	}
	if err := live.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := live.Wait(); err == nil {
		t.Fatal("live browser unexpectedly exited normally")
	}
	if err := os.Remove(ready); err != nil {
		t.Fatal(err)
	}
	launcher := exec.Command(os.Args[0], "-test.run=^TestOrphanBrowserCleanup$")
	launcher.Env = append(os.Environ(), "TEST_ORPHAN_LAUNCHER=1", "TEST_ORPHAN_EXE="+exe, "TEST_ORPHAN_FILE="+locked, "TEST_ORPHAN_READY="+ready, "TEST_ORPHAN_DATA="+filepath.Join(webview, "EBWebView"))
	if output, err := launcher.CombinedOutput(); err != nil {
		t.Fatalf("launcher: %v: %s", err, output)
	}
	pidBytes, err := os.ReadFile(ready)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	defer process.Kill()
	if err := clearProfileCache(filepath.Join(dir, "profile"), "orphan-test"); err != nil {
		t.Fatalf("cache still locked: %v", err)
	}
	if data, err := os.ReadFile(cookie); err != nil || string(data) != "session" {
		t.Fatalf("cookie changed: %q %v", data, err)
	}
}
