package profile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// Message is sent only over the private pipe to the workspace process.
type Message struct {
	Kind    string        `json:"kind"`
	ID      string        `json:"id,omitempty"`
	HWND    uintptr       `json:"hwnd,omitempty"`
	Title   string        `json:"title,omitempty"`
	Body    string        `json:"body,omitempty"`
	Silent  bool          `json:"silent,omitempty"`
	Token   string        `json:"token,omitempty"`
	Image   []byte        `json:"image,omitempty"`
	URL     string        `json:"url,omitempty"`
	Audio   []MediaDevice `json:"audio,omitempty"`
	Video   []MediaDevice `json:"video,omitempty"`
	AudioID string        `json:"audioId,omitempty"`
	VideoID string        `json:"videoId,omitempty"`
	Count   int           `json:"count,omitempty"`
	Err     string        `json:"err,omitempty"`
	// Dir is the folder this profile saves downloads to.
	Dir string `json:"dir,omitempty"`
	// Download is one changed download; Downloads is the saved list; Prompt asks
	// the workspace whether a download should proceed.
	Download  *DownloadItem   `json:"download,omitempty"`
	Downloads []DownloadItem  `json:"downloads,omitempty"`
	Prompt    *DownloadPrompt `json:"prompt,omitempty"`
}

// DownloadPrompt describes a pending download awaiting user confirmation.
type DownloadPrompt struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Dir    string `json:"dir"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

// MediaDevice is a microphone or camera the profile's WebView2 reports.
type MediaDevice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Command is a request from the workspace process over the private pipe.
type Command struct {
	Action      string              `json:"action"`
	ID          string              `json:"id"`
	Token       string              `json:"token"`
	AudioID     string              `json:"audioId"`
	VideoID     string              `json:"videoId"`
	Theme       string              `json:"theme"`
	Dir         string              `json:"dir"`
	Name        string              `json:"name,omitempty"`
	Accept      bool                       `json:"accept,omitempty"`
	Userscripts []UserscriptPayload        `json:"userscripts,omitempty"`
	Sounds      *NotificationSoundsPayload `json:"sounds,omitempty"`
}

// One reader owns stdin; the encoder is shared by page callbacks and bound methods.
type connection struct {
	input  *bufio.Scanner
	output *json.Encoder
	mu     sync.Mutex
}

func newConnection(input io.Reader, output io.Writer) *connection {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	return &connection{input: scanner, output: json.NewEncoder(output)}
}

func validUserscripts(list []UserscriptPayload) bool {
	if len(list) > 128 {
		return false
	}
	for _, item := range list {
		if item.ID == "" || len(item.ID) > 256 || strings.ContainsRune(item.ID, 0) {
			return false
		}
		if len(item.Name) > 256 || strings.ContainsRune(item.Name, 0) {
			return false
		}
		if len(item.Code) > MaxResolvedUserscriptBytes || !utf8.ValidString(item.Code) {
			return false
		}
	}
	return true
}

func (c *connection) receive() (Command, error) {
	var cmd Command
	if !c.input.Scan() {
		if err := c.input.Err(); err != nil {
			return cmd, fmt.Errorf("membaca kanal profil: %w", err)
		}
		return cmd, io.EOF
	}
	if err := json.Unmarshal(c.input.Bytes(), &cmd); err != nil {
		return cmd, fmt.Errorf("perintah profil tidak valid: %w", err)
	}
	if len(cmd.ID) > 64 {
		return cmd, errors.New("ID permintaan terlalu panjang")
	}
	switch cmd.Action {
	case "start":
		if !validUserscripts(cmd.Userscripts) {
			return cmd, errors.New("daftar userscript tidak valid")
		}
	case "reload", "quit", "devtools":
	case "click":
		if len(cmd.Token) > 128 {
			return cmd, errors.New("token notifikasi terlalu panjang")
		}
	case "media":
	case "setmedia":
		if len(cmd.AudioID) > 256 || len(cmd.VideoID) > 256 {
			return cmd, errors.New("ID perangkat terlalu panjang")
		}
	case "theme":
		if cmd.Theme != ThemeSystem && cmd.Theme != ThemeLight && cmd.Theme != ThemeDark {
			return cmd, errors.New("tema tidak valid")
		}
	case "userscripts":
		if !validUserscripts(cmd.Userscripts) {
			return cmd, errors.New("daftar userscript tidak valid")
		}
	case "sounds":
		if cmd.Sounds == nil {
			return cmd, errors.New("pengaturan suara notifikasi tidak valid")
		}
	case "setdownloaddir":
		if len(cmd.Dir) > 1024 || strings.ContainsRune(cmd.Dir, 0) {
			return cmd, errors.New("folder unduhan tidak valid")
		}
	case "downloadreply":
		if cmd.ID == "" || len(cmd.Dir) > 1024 || strings.ContainsRune(cmd.Dir, 0) || utf8.RuneCountInString(cmd.Name) > 256 || strings.ContainsRune(cmd.Name, 0) {
			return cmd, errors.New("balasan konfirmasi unduhan tidak valid")
		}
	default:
		return cmd, fmt.Errorf("perintah profil tidak dikenal: %q", cmd.Action)
	}
	return cmd, nil
}

func (c *connection) send(msg Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.output.Encode(msg)
}

func ValidID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16 && id == strings.ToLower(id)
}

// ProfileBridge exposes notifications and bounded userscript GM APIs to the page.
type ProfileBridge struct {
	conn *connection
	gm   *gmStorage
	ctx  context.Context
}

func (b *ProfileBridge) Notify(title, body, token string, silent bool, encodedImage string) error {
	if utf8.RuneCountInString(title) > 256 || utf8.RuneCountInString(body) > 4096 || len(token) > 128 {
		return errors.New("notifikasi terlalu panjang")
	}
	return b.conn.send(Message{
		Kind: "notify", Title: title, Body: body, Token: token, Silent: silent,
		Image: notificationImage(encodedImage),
	})
}

// This boundary accepts only the small PNG/JPEG thumbnails produced by notification.js.
// Bad or oversized pictures are omitted, never allowed to kill text delivery.
func notificationImage(encoded string) []byte {
	const maxBytes = 24 << 10
	if len(encoded) == 0 || len(encoded) > base64.StdEncoding.EncodedLen(maxBytes) {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) > maxBytes {
		return nil
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width < 1 || config.Height < 1 || config.Width > 128 || config.Height > 128 {
		return nil
	}
	return data
}

func (b *ProfileBridge) Dismiss(token string) error {
	if len(token) > 128 {
		return errors.New("token notifikasi tidak valid")
	}
	return b.conn.send(Message{Kind: "dismiss", Token: token})
}

// SetUnread reports the unread chats WhatsApp itself counts in the page title.
func (b *ProfileBridge) SetUnread(count int) error {
	if count < 0 || count > 9999 {
		return errors.New("jumlah pesan belum dibaca tidak valid")
	}
	return b.conn.send(Message{Kind: "unread", Count: count})
}

// GMSetValue stores a JSON-encoded value for a userscript in the profile.
func (b *ProfileBridge) GMSetValue(scriptID, key, jsonValue string) error {
	if b.gm == nil {
		return errors.New("penyimpanan userscript belum siap")
	}
	return b.gm.set(scriptID, key, jsonValue)
}

// GMDeleteValue removes a key from a userscript's storage in the profile.
func (b *ProfileBridge) GMDeleteValue(scriptID, key string) error {
	if b.gm == nil {
		return errors.New("penyimpanan userscript belum siap")
	}
	return b.gm.delete(scriptID, key)
}

// GMFetch performs an HTTP request on behalf of a userscript (GM_xmlhttpRequest).
func (b *ProfileBridge) GMFetch(reqJSON string) (string, error) {
	ctx := b.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return performGMFetch(ctx, reqJSON)
}

// GMSetClipboard copies plain text to the system clipboard for a userscript.
func (b *ProfileBridge) GMSetClipboard(text string) error {
	if len(text) > 1<<20 {
		return errors.New("teks clipboard terlalu panjang")
	}
	return mygo.Clipboard.Write(transfer.TextData(text))
}

// GMOpenInTab opens an external http/https URL in the user's browser.
func (b *ProfileBridge) GMOpenInTab(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("URL GM_openInTab tidak valid")
	}
	return b.conn.send(Message{Kind: "external", URL: u.String()})
}
