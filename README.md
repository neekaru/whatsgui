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
node --test internal/profile/notification.test.cjs
CGO_ENABLED=0 go test ./internal/profile -run 'TestNotificationPhotoFitsProfilePipe|TestInvalidNotificationPhotoKeepsText'
```
