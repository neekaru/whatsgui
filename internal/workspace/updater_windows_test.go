package workspace

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.2", "0.1.1", 1},
		{"v0.2.0", "0.1.9", 1},
		{"0.1.1", "v0.1.1", 0},
		{"0.1.1-beta.1", "0.1.1", -1},
		{"0.1.1", "0.1.1-beta.1", 1},
		{"1.0.0", "0.99.99", 1},
		{"0.1.0", "0.1.1", -1},
	}
	for _, tc := range cases {
		if got := compareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestFetchLatestGitHubReleaseAndAssetSelection(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"tag_name": "v0.2.0",
					"body": "Perbaikan dan fitur baru",
					"published_at": "2026-11-28T10:00:00Z",
					"html_url": "https://github.com/neekaru/whatsgui/releases/tag/v0.2.0",
					"assets": [
						{"name": "whatsgui-windows-amd64.zip", "browser_download_url": "https://example.com/app.zip"},
						{"name": "whatsappgui-new-setup-0.2.0-amd64.exe", "browser_download_url": "https://example.com/setup-amd64.exe"},
						{"name": "whatsappgui-new-setup-0.2.0-arm64.exe", "browser_download_url": "https://example.com/setup-arm64.exe"}
					]
				}`)),
			}, nil
		}),
	}

	rel, err := fetchLatestGitHubRelease(context.Background(), client, "https://api.github.com/repos/neekaru/whatsgui/releases/latest")
	if err != nil {
		t.Fatalf("fetchLatestGitHubRelease: %v", err)
	}
	name, url := pickReleaseAsset(rel.Assets, "amd64")
	if name != "whatsappgui-new-setup-0.2.0-amd64.exe" || url != "https://example.com/setup-amd64.exe" {
		t.Fatalf("picked = (%q, %q)", name, url)
	}
}
