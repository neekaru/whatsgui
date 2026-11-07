package profile

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode/utf8"
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
	URL     string        `json:"url,omitempty"`
	Audio   []MediaDevice `json:"audio,omitempty"`
	Video   []MediaDevice `json:"video,omitempty"`
	AudioID string        `json:"audioId,omitempty"`
	VideoID string        `json:"videoId,omitempty"`
	Count   int           `json:"count,omitempty"`
	Err     string        `json:"err,omitempty"`
	// Dir is the folder this profile saves downloads to.
	Dir string `json:"dir,omitempty"`
	// Download is one changed download; Downloads is the saved list.
	Download  *DownloadItem  `json:"download,omitempty"`
	Downloads []DownloadItem `json:"downloads,omitempty"`
}

// MediaDevice is a microphone or camera the profile's WebView2 reports.
type MediaDevice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Command is a request from the workspace process over the private pipe.
type Command struct {
	Action  string `json:"action"`
	ID      string `json:"id"`
	Token   string `json:"token"`
	AudioID string `json:"audioId"`
	VideoID string `json:"videoId"`
	Theme   string `json:"theme"`
	Dir     string `json:"dir"`
}

// One reader owns stdin; the encoder is shared by page callbacks and bound methods.
type connection struct {
	input  *bufio.Scanner
	output *json.Encoder
	mu     sync.Mutex
}

func newConnection(input io.Reader, output io.Writer) *connection {
	return &connection{input: bufio.NewScanner(input), output: json.NewEncoder(output)}
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
	case "start", "reload", "quit", "devtools":
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
	case "setdownloaddir":
		if len(cmd.Dir) > 1024 || strings.ContainsRune(cmd.Dir, 0) {
			return cmd, errors.New("folder unduhan tidak valid")
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

// ProfileBridge exposes notifications only, not workspace or filesystem access.
type ProfileBridge struct{ conn *connection }

func (b *ProfileBridge) Notify(title, body, token string, silent bool) error {
	if utf8.RuneCountInString(title) > 256 || utf8.RuneCountInString(body) > 4096 || len(token) > 128 {
		return errors.New("notifikasi terlalu panjang")
	}
	return b.conn.send(Message{Kind: "notify", Title: title, Body: body, Token: token, Silent: silent})
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
