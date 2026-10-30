package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

type storedState struct {
	State
	DeletedIDs []string `json:"deletedIds,omitempty"`
}

// cleanupDeleted runs under mu. Deleted tabs never become live tabs again merely
// because an antivirus/indexer or a previous WebView2 process still owns a file.
func (w *Workspace) cleanupDeleted() error {
	var failures []error
	for _, id := range slices.Clone(w.deletedIDs) {
		if err := removeProfile(profileDirectory(profileName(id))); err != nil {
			failures = append(failures, fmt.Errorf("data profil %s: %w", id, err))
			continue
		}
		previous := w.deletedIDs
		w.deletedIDs = slices.DeleteFunc(slices.Clone(previous), func(deleted string) bool { return deleted == id })
		if err := w.save(w.data); err != nil {
			w.deletedIDs = previous
			return err
		}
	}
	return errors.Join(failures...)
}

func removeProfile(dir string) error {
	err := os.RemoveAll(dir)
	if err == nil {
		return nil
	}
	if !fileBusy(err) {
		return err
	}
	// Upgrade recovery: pre-job workers may have left an orphan browser alive.
	// Match the exact user-data directory; never kill all WebView2/Edge processes.
	if err := stopOrphanBrowser(filepath.Join(dir, "WebView2")); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = os.RemoveAll(dir)
		if err == nil || !fileBusy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func fileBusy(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}

func stopOrphanBrowser(dataDir string) error {
	const script = `$ErrorActionPreference = 'Stop'
$processes = @(Get-CimInstance Win32_Process)
$profile = Split-Path -Leaf (Split-Path -Parent $env:WHATSAPP_PROFILE_CLEANUP)
$hostPattern = '(?:^|\s)--profile\s+"?' + [regex]::Escape($profile) + '"?(?:\s|$)'
if ($processes | Where-Object { $_.CommandLine -match $hostPattern }) {
  throw 'Profil masih digunakan oleh proses lain'
}
foreach ($process in $processes) {
  if ($process.Name -ine 'msedgewebview2.exe' -or $process.CommandLine -match '(?:^|\s)--type=') { continue }
  $match = [regex]::Match($process.CommandLine, '(?:^|\s)(?:"--user-data-dir=([^"]+)"|--user-data-dir=(?:"([^"]+)"|(\S+)))')
  if (!$match.Success) { continue }
  $path = if ($match.Groups[1].Success) { $match.Groups[1].Value } elseif ($match.Groups[2].Success) { $match.Groups[2].Value } else { $match.Groups[3].Value }
  $root = $env:WHATSAPP_PROFILE_CLEANUP.TrimEnd('\')
  if (![string]::Equals($path.TrimEnd('\'), $root, [StringComparison]::OrdinalIgnoreCase) -and
      ![string]::Equals($path.TrimEnd('\'), ($root + '\EBWebView'), [StringComparison]::OrdinalIgnoreCase)) { continue }
  if ($processes | Where-Object { $_.ProcessId -eq $process.ParentProcessId }) { continue }
  Stop-Process -Id $process.ProcessId -Force -ErrorAction SilentlyContinue
}`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "WHATSAPP_PROFILE_CLEANUP="+dataDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("menutup browser profil lama: %w: %s", err, output)
	}
	return nil
}
