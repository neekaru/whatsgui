# whatsgui

## Notification photos

Contact/group pictures supplied through WhatsApp's notification `icon` now
travel from the page to `NotificationOptions.Image`. Windows displays them as
the notification's large icon, not a full-size message-photo preview.

- The browser converts supported raster images to PNG, at most 128×128 pixels
  and 24 KiB. High-entropy pictures shrink to 64×64 if needed.
- Page-owned blobs, PNG/JPEG/WebP data URLs and HTTPS images are loaded in the
  page under its normal CSP/CORS restrictions. Go never fetches image URLs.
- Invalid, blocked, oversized or slow pictures fall back to text; image loading
  has a 1.5-second deadline. Closing while a picture loads cancels its delivery.
- The wrapper does not write an avatar cache to disk. Windows may retain
  notification content according to system settings.
- The 24 KiB bound keeps photo notifications, including maximally escaped
  allowed text, within the existing 64 KiB profile IPC line limit.

Checks:

```sh
node --test internal/profile/scripts/notification.test.cjs
CGO_ENABLED=0 go test ./internal/profile -run 'TestNotificationPhotoFitsProfilePipe|TestInvalidNotificationPhotoKeepsText'
```

## Userscripts

Profiles support custom `.user.js` / `.js` scripts managed in **Settings → Userscript**, with:

- `@match` / `@include`, `@run-at` (`document-start`, `document-end`, `document-idle`), and local `@require` modules relative to the `userscripts/` folder.
- `GM_getValue`, `GM_setValue`, `GM_deleteValue`, `GM_listValues`, `GM_addStyle`, `GM_xmlhttpRequest`, `GM_notification`, `GM_setClipboard`, `GM_openInTab`, `GM_info`, `GM.*`, and `unsafeWindow`.

Bundled examples live in [`examples/userscripts/`](examples/userscripts/):

- `privacy-blur-toggle.user.js` — toggles chat list and header blur with `Alt+B`.
- `wa-audio-stickers.user.js` — adds a **Stiker Biasa** / **Stiker Audio** chooser when clicking the WhatsApp Web sticker **Buat** (`ic-add`) button.

## Credits & Third-Party Licenses

The `wa-audio-stickers` example ([`examples/userscripts/wa-audio-stickers/`](examples/userscripts/wa-audio-stickers/)) is adapted from [Shirtep/WA-audio-stickers](https://github.com/Shirtep/WA-audio-stickers) and is licensed under the **PolyForm Noncommercial License 1.0.0** (<https://polyformproject.org/licenses/noncommercial/1.0.0>) for noncommercial use only. See [`examples/userscripts/wa-audio-stickers/LICENSE.md`](examples/userscripts/wa-audio-stickers/LICENSE.md) for the full terms.

> Required Notice: Copyright Shirtep (https://github.com/Shirtep/WA-audio-stickers)

