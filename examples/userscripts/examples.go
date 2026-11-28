package userscripts

import "embed"

// PrivacyBlurExample is the bundled example userscript for WhatsApp Web.
//
//go:embed privacy-blur-toggle.user.js
var PrivacyBlurExample string

// AudioStickersExample is the bundled WA-audio-stickers userscript wrapper.
//
//go:embed wa-audio-stickers.user.js
var AudioStickersExample string

// AudioStickersAssets holds the unmodified WA-audio-stickers extension scripts.
//
//go:embed wa-audio-stickers/*.js
var AudioStickersAssets embed.FS
