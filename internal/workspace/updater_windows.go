package workspace

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

const githubLatestReleaseURL = "https://api.github.com/repos/neekaru/whatsgui/releases/latest"

// UpdateInfo describes the latest available release compared against the running app.
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Notes          string `json:"notes"`
	PublishedAt    string `json:"publishedAt"`
	ReleaseURL     string `json:"releaseUrl"`
	DownloadURL    string `json:"downloadUrl"`
	AssetName      string `json:"assetName"`
	CanAutoInstall bool   `json:"canAutoInstall"`
	AutoEnabled    bool   `json:"autoEnabled"`
}

// UpdateProgress reports progress while downloading or installing a signed update.
type UpdateProgress struct {
	Stage      string `json:"stage"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Error      string `json:"error,omitempty"`
}

// UpdateAvailable notifies the UI when a startup check finds a newer version.
var UpdateAvailable = mygo.NewEvent[UpdateInfo]("updateAvailable")

// UpdateProgressChanged notifies the UI during in-app update download and install.
var UpdateProgressChanged = mygo.NewEvent[UpdateProgress]("updateProgressChanged")

type updaterState struct {
	mu          sync.Mutex
	pending     *mygo.Update
	latest      UpdateInfo
	installing  bool
	releaseHTTP *http.Client
}

func newUpdaterState() *updaterState {
	return &updaterState{
		releaseHTTP: &http.Client{Timeout: 10 * time.Second},
	}
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// AppVersion returns the running application version string.
func (w *Workspace) AppVersion() string {
	v := strings.TrimSpace(mygo.App.Version())
	if v == "" {
		return "0.1.0"
	}
	return strings.TrimPrefix(v, "v")
}

// CheckForUpdates checks for a newer release via MyGo's signed updater when
// configured, falling back to the public GitHub Releases API.
func (w *Workspace) CheckForUpdates() (UpdateInfo, error) {
	current := w.AppVersion()
	autoEnabled := mygo.Updater.Enabled()
	info := UpdateInfo{
		CurrentVersion: current,
		LatestVersion:  current,
		AutoEnabled:    autoEnabled,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if autoEnabled {
		up, err := mygo.Updater.Check(ctx)
		if err == nil {
			if up == nil {
				w.setUpdaterResult(nil, info)
				return info, nil
			}
			info.Available = true
			info.LatestVersion = strings.TrimPrefix(up.Version, "v")
			info.Notes = up.Notes
			if !up.Date.IsZero() {
				info.PublishedAt = up.Date.Format(time.RFC3339)
			}
			info.ReleaseURL = "https://github.com/neekaru/whatsgui/releases/latest"
			info.CanAutoInstall = true
			w.setUpdaterResult(up, info)
			return info, nil
		}
	}

	rel, err := fetchLatestGitHubRelease(ctx, w.updater.releaseHTTP, githubLatestReleaseURL)
	if err != nil {
		return UpdateInfo{}, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if latest == "" {
		latest = current
	}
	assetName, downloadURL := pickReleaseAsset(rel.Assets, runtime.GOARCH)
	info.LatestVersion = latest
	info.Notes = rel.Body
	info.PublishedAt = rel.PublishedAt
	info.ReleaseURL = rel.HTMLURL
	info.AssetName = assetName
	info.DownloadURL = downloadURL
	info.Available = !rel.Draft && !rel.Prerelease && compareSemver(latest, current) > 0
	w.setUpdaterResult(nil, info)
	return info, nil
}

// InstallUpdate installs a signed update and relaunches when supported, or
// opens the GitHub release installer download in the user's browser.
func (w *Workspace) InstallUpdate() error {
	w.updater.mu.Lock()
	if w.updater.installing {
		w.updater.mu.Unlock()
		return errors.New("pembaruan sedang berlangsung")
	}
	up := w.updater.pending
	latest := w.updater.latest
	if up == nil {
		w.updater.mu.Unlock()
		target := latest.DownloadURL
		if target == "" {
			target = latest.ReleaseURL
		}
		if target == "" {
			return errors.New("tautan pembaruan tidak tersedia")
		}
		return mygo.Shell.OpenExternal(target)
	}
	w.updater.installing = true
	w.updater.mu.Unlock()

	emitProgress := func(p UpdateProgress) {
		if w.window != nil && !w.window.IsDestroyed() {
			_ = UpdateProgressChanged.Emit(w.window, p)
		}
	}

	go func() {
		emitProgress(UpdateProgress{Stage: "downloading"})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := up.Install(ctx, func(downloaded, total int64) {
			emitProgress(UpdateProgress{Stage: "downloading", Downloaded: downloaded, Total: total})
		})
		cancel()
		if err != nil {
			w.updater.mu.Lock()
			w.updater.installing = false
			w.updater.mu.Unlock()
			emitProgress(UpdateProgress{Stage: "error", Error: err.Error()})
			return
		}
		emitProgress(UpdateProgress{Stage: "ready"})
		w.close()
		mygo.App.Relaunch()
	}()
	return nil
}

func (w *Workspace) setUpdaterResult(up *mygo.Update, info UpdateInfo) {
	w.updater.mu.Lock()
	w.updater.pending = up
	w.updater.latest = info
	w.updater.mu.Unlock()
}

func (w *Workspace) checkUpdatesOnStartup() {
	time.Sleep(4 * time.Second)
	if w.closing.Load() {
		return
	}
	info, err := w.CheckForUpdates()
	if err != nil || !info.Available || w.closing.Load() {
		return
	}
	if w.window != nil && !w.window.IsDestroyed() {
		_ = UpdateAvailable.Emit(w.window, info)
	}
}

func fetchLatestGitHubRelease(ctx context.Context, client *http.Client, endpoint string) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return githubRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "whatsgui-updater")

	resp, err := client.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("memeriksa rilis terbaru: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub Releases mengembalikan status %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return githubRelease{}, fmt.Errorf("membaca metadata rilis: %w", err)
	}
	return rel, nil
}

func pickReleaseAsset(assets []githubAsset, arch string) (name, url string) {
	lowerArch := strings.ToLower(arch)
	// Prefer Windows NSIS installer matching current architecture, then portable zip.
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.Contains(lower, "setup") && strings.HasSuffix(lower, ".exe") && strings.Contains(lower, lowerArch) {
			return a.Name, a.BrowserDownloadURL
		}
	}
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.HasSuffix(lower, ".zip") && strings.Contains(lower, lowerArch) {
			return a.Name, a.BrowserDownloadURL
		}
	}
	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".zip") {
			return a.Name, a.BrowserDownloadURL
		}
	}
	return "", ""
}

// compareSemver compares two semantic version strings (ignoring leading "v").
func compareSemver(a, b string) int {
	pa, prea := splitSemver(a)
	pb, preb := splitSemver(b)
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var va, vb int
		if i < len(pa) {
			va = pa[i]
		}
		if i < len(pb) {
			vb = pb[i]
		}
		if c := cmp.Compare(va, vb); c != 0 {
			return c
		}
	}
	if prea == "" && preb == "" {
		return 0
	}
	if prea == "" {
		return 1
	}
	if preb == "" {
		return -1
	}
	return strings.Compare(prea, preb)
}

func splitSemver(v string) ([]int, string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v, _, _ = strings.Cut(v, "+")
	core, pre, _ := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	nums := make([]int, len(parts))
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err == nil && n >= 0 {
			nums[i] = n
		}
	}
	return nums, pre
}
