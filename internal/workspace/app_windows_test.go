package workspace

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"whatsappgui-new/internal/win32"

	"github.com/egoist/mygo"
)

// App.Run needs the main goroutine; helper processes enter here, not a test
// goroutine. Each helper uses an isolated app identity and temporary data.
func TestMain(m *testing.M) {
	if root := os.Getenv("WHATSAPP_INSTANCE_TEST_ROOT"); root != "" {
		if err := runInstanceSmoke(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runInstanceSmoke(root string) error {
	if os.Getenv("WHATSAPP_INSTANCE_TEST_SECOND") != "1" {
		// Let the parent contain WebView2 descendants before the app starts.
		var start [1]byte
		if _, err := io.ReadFull(os.Stdin, start[:]); err != nil {
			return err
		}
	}
	mygo.App.SetName(fmt.Sprintf("workspace-instance-%x", sha256.Sum256([]byte(root))))
	mygo.App.SetPath(mygo.PathUserData, root)
	// Native MyGo creates this empty default skeleton even with a WebView2
	// directory override. Remove only empty test-owned directories on exit.
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = os.TempDir()
	}
	legacy := filepath.Join(local, mygo.App.Name())
	defer func() {
		_ = os.Remove(filepath.Join(legacy, "WebView2"))
		_ = os.Remove(legacy)
	}()
	var failure error
	if os.Getenv("WHATSAPP_INSTANCE_TEST_SECOND") == "1" {
		// A second launch must exit before even reading its workspace settings.
		mygo.App.SetPath(mygo.PathUserData, filepath.Join(root, "blocked"))
	} else {
		mygo.App.WhenReady(func() {
			// Queue behind Run's window creation, then simulate close-to-tray.
			mygo.App.WhenReady(func() {
				windows := mygo.Windows()
				if len(windows) != 1 {
					failure = fmt.Errorf("initial workspace windows = %d, want 1", len(windows))
					for _, window := range windows {
						window.Destroy()
					}
					mygo.App.Quit()
					return
				}
				windows[0].Hide()
				fmt.Fprintln(os.Stdout, "workspace hidden")
			})
		})
		mygo.App.OnSecondInstance(func([]string, string) {
			// Observe after Run's activation handler has restored the window.
			mygo.App.WhenReady(func() {
				windows := mygo.Windows()
				if len(windows) != 1 || !windows[0].IsVisible() {
					failure = fmt.Errorf("second launch did not restore exactly one workspace")
				} else {
					fmt.Fprintln(os.Stdout, "workspace restored")
				}
				// Bypass close-to-tray; the last window's destruction quits MyGo.
				for _, window := range windows {
					window.Destroy()
				}
			})
		})
	}
	if err := Run(); err != nil {
		return err
	}
	return failure
}

func TestWorkspaceSingleInstance(t *testing.T) {
	root := t.TempDir()
	// No accounts: the smoke window never starts workers or accesses WhatsApp.
	if err := os.WriteFile(filepath.Join(root, "tabs.json"), []byte(`{"tabs":[],"activeId":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "blocked"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "blocked", "tabs.json"), []byte(`invalid`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := func(second bool) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWorkspaceSingleInstance$")
		cmd.Env = append(os.Environ(), "WHATSAPP_INSTANCE_TEST_ROOT="+root,
			"WEBVIEW2_USER_DATA_FOLDER="+filepath.Join(root, "WebView2"))
		if second {
			cmd.Env = append(cmd.Env, "WHATSAPP_INSTANCE_TEST_SECOND=1")
		}
		cmd.Stderr = os.Stderr
		return cmd
	}
	first := command(false)
	input, err := first.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	stdout, err := first.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	job, err := win32.ContainProcess(first.Process.Pid)
	if err != nil {
		first.Process.Kill()
		first.Wait()
		t.Fatal(err)
	}
	defer func() {
		first.Process.Kill()
		if first.ProcessState == nil {
			first.Wait()
		}
		if err := job.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := input.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	input.Close()
	lines := bufio.NewScanner(stdout)
	if !lines.Scan() || lines.Text() != "workspace hidden" {
		t.Fatalf("first launch did not reach tray state: %q, %v", lines.Text(), lines.Err())
	}
	if output, err := command(true).Output(); err != nil {
		t.Fatalf("second launch failed instead of handing over: %s, %v", output, err)
	}
	if !lines.Scan() || lines.Text() != "workspace restored" {
		t.Fatalf("running workspace was not restored: %q, %v", lines.Text(), lines.Err())
	}
	// Drain the test helper's remaining output before waiting for its exit.
	for lines.Scan() {
		t.Log(lines.Text())
	}
	if err := lines.Err(); err != nil {
		t.Fatal(err)
	}
	if err := first.Wait(); err != nil {
		t.Fatal(err)
	}
}
