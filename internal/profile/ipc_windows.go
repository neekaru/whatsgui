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
	Kind   string  `json:"kind"`
	HWND   uintptr `json:"hwnd,omitempty"`
	Title  string  `json:"title,omitempty"`
	Body   string  `json:"body,omitempty"`
	Silent bool    `json:"silent,omitempty"`
	Token  string  `json:"token,omitempty"`
	URL    string  `json:"url,omitempty"`
}

type command struct {
	Action string `json:"action"`
	Token  string `json:"token"`
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

func (c *connection) receive() (command, error) {
	var cmd command
	if !c.input.Scan() {
		if err := c.input.Err(); err != nil {
			return cmd, fmt.Errorf("membaca kanal profil: %w", err)
		}
		return cmd, io.EOF
	}
	if err := json.Unmarshal(c.input.Bytes(), &cmd); err != nil {
		return cmd, fmt.Errorf("perintah profil tidak valid: %w", err)
	}
	switch cmd.Action {
	case "start", "reload", "quit":
	case "click":
		if len(cmd.Token) > 128 {
			return cmd, errors.New("token notifikasi terlalu panjang")
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
