package profile

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

// Notification sound target channels and modes.
const (
	SoundKindMessage = "message"
	SoundKindCall    = "call"

	SoundModeDefault = "default"
	SoundModePreset  = "preset"
	SoundModeCustom  = "custom"
	SoundModeMute    = "mute"

	// MaxNotificationSoundBytes caps custom audio files (7 MiB) so base64 IPC payloads
	// stay comfortably inside the 32 MiB profile pipe scanner limit.
	MaxNotificationSoundBytes = 7 << 20
)

var (
	MessageSoundPresets = []string{"chime", "crystal", "pop", "bell"}
	CallSoundPresets    = []string{"classic", "marimba", "pulse", "gentle"}
)

var allowedSoundMimeTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/ogg",
	".m4a":  "audio/mp4",
	".mp4":  "audio/mp4",
	".aac":  "audio/aac",
	".flac": "audio/flac",
	".webm": "audio/webm",
}

// NotificationSoundChoice configures the sound used for either incoming
// messages or incoming calls.
type NotificationSoundChoice struct {
	Mode     string `json:"mode"`
	Preset   string `json:"preset,omitempty"`
	FileName string `json:"fileName,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Volume   int    `json:"volume"`
}

// NotificationSoundSettings stores the workspace's notification audio choices.
type NotificationSoundSettings struct {
	Message NotificationSoundChoice `json:"message"`
	Call    NotificationSoundChoice `json:"call"`
}

// NotificationSoundChannel is the runtime audio payload sent to profile workers.
type NotificationSoundChannel struct {
	Mode    string `json:"mode"`
	Preset  string `json:"preset,omitempty"`
	DataURL string `json:"dataUrl,omitempty"`
	Volume  int    `json:"volume"`
}

// NotificationSoundsPayload carries resolved sound sources into WhatsApp pages.
type NotificationSoundsPayload struct {
	Message NotificationSoundChannel `json:"message"`
	Call    NotificationSoundChannel `json:"call"`
}

func DefaultNotificationSoundChoice(kind string) NotificationSoundChoice {
	preset := MessageSoundPresets[0]
	if kind == SoundKindCall {
		preset = CallSoundPresets[0]
	}
	return NotificationSoundChoice{
		Mode:   SoundModeDefault,
		Preset: preset,
		Volume: 100,
	}
}

func DefaultNotificationSoundSettings() NotificationSoundSettings {
	return NotificationSoundSettings{
		Message: DefaultNotificationSoundChoice(SoundKindMessage),
		Call:    DefaultNotificationSoundChoice(SoundKindCall),
	}
}

func SoundMimeTypeForExt(ext string) (string, bool) {
	mime, ok := allowedSoundMimeTypes[strings.ToLower(strings.TrimSpace(ext))]
	return mime, ok
}

func ValidSoundKind(kind string) bool {
	return kind == SoundKindMessage || kind == SoundKindCall
}

func normalizeSoundChoice(kind string, choice NotificationSoundChoice) (NotificationSoundChoice, error) {
	presets := MessageSoundPresets
	if kind == SoundKindCall {
		presets = CallSoundPresets
	}
	mode := strings.ToLower(strings.TrimSpace(choice.Mode))
	if mode == "" {
		mode = SoundModeDefault
	}
	switch mode {
	case SoundModeDefault, SoundModePreset, SoundModeCustom, SoundModeMute:
	default:
		return NotificationSoundChoice{}, errors.New("mode suara notifikasi tidak valid")
	}

	preset := strings.ToLower(strings.TrimSpace(choice.Preset))
	if preset == "" {
		preset = presets[0]
	} else if !slices.Contains(presets, preset) {
		return NotificationSoundChoice{}, errors.New("preset suara notifikasi tidak dikenal")
	}

	volume := choice.Volume
	if volume <= 0 {
		volume = 100
	} else if volume > 100 {
		volume = 100
	}

	fileName := strings.TrimSpace(filepath.Base(choice.FileName))
	if fileName == "." || fileName == ".." || strings.ContainsRune(fileName, 0) {
		fileName = ""
	}
	mimeType := strings.TrimSpace(choice.MimeType)
	if mode == SoundModeCustom && (fileName == "" || mimeType == "") {
		return NotificationSoundChoice{}, errors.New("berkas audio kustom belum dipilih")
	}

	return NotificationSoundChoice{
		Mode:     mode,
		Preset:   preset,
		FileName: fileName,
		MimeType: mimeType,
		Volume:   volume,
	}, nil
}

// NormalizeNotificationSoundSettings validates and fills defaults for sound settings.
func NormalizeNotificationSoundSettings(settings NotificationSoundSettings) (NotificationSoundSettings, error) {
	msg, err := normalizeSoundChoice(SoundKindMessage, settings.Message)
	if err != nil {
		return NotificationSoundSettings{}, err
	}
	call, err := normalizeSoundChoice(SoundKindCall, settings.Call)
	if err != nil {
		return NotificationSoundSettings{}, err
	}
	return NotificationSoundSettings{Message: msg, Call: call}, nil
}
