package win32

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestJobAllowsProfileFlush(t *testing.T) {
	if file := os.Getenv("MYGO_JOB_FLUSH_FILE"); file != "" {
		var start [1]byte
		if _, err := io.ReadFull(os.Stdin, start[:]); err != nil {
			t.Fatal(err)
		}
		// A browser descendant can still be writing after its host exits.
		time.Sleep(200 * time.Millisecond)
		f, err := os.Create(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("session persisted"); err != nil {
			t.Fatal(err)
		}
		if err := f.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		return
	}
	file := filepath.Join(t.TempDir(), "session")
	cmd := exec.Command(os.Args[0], "-test.run=^TestJobAllowsProfileFlush$")
	cmd.Env = append(os.Environ(), "MYGO_JOB_FLUSH_FILE="+file)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	job, err := ContainProcess(cmd.Process.Pid)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		t.Fatal(err)
	}
	if _, err := input.Write([]byte{1}); err != nil {
		job.Close()
		cmd.Wait()
		t.Fatal(err)
	}
	closeErr := job.Close()
	waitErr := cmd.Wait()
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if waitErr != nil {
		t.Fatal(waitErr)
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "session persisted" {
		t.Fatalf("job killed profile before flush: %q, %v", data, err)
	}
}
