<script lang="ts">
  import { fileKind } from "../lib/downloads";
  import type { DownloadPrompt } from "../lib/mygo";

  interface Props {
    prompt: DownloadPrompt;
    name: string;
    previewPath: string;
    busy: boolean;
    nameField?: HTMLInputElement;
    onPickFolder: () => void;
    onReply: (accept: boolean) => void;
  }

  let {
    prompt,
    name = $bindable(),
    previewPath,
    busy,
    nameField = $bindable(),
    onPickFolder,
    onReply,
  }: Props = $props();

  const kind = $derived(fileKind(name.trim() || prompt.name));
</script>

<div class="modal-backdrop" role="presentation">
  <div class="modal modal-sm" role="dialog" aria-modal="true" aria-labelledby="download-confirm-title">
    <div class="modal-header">
      <div class="modal-header-info">
        <div class="modal-icon-badge">
          <svg viewBox="0 0 16 16" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
            <path d="M8 2v8M4.5 6.5L8 10l3.5-3.5M2.5 13h11"/>
          </svg>
        </div>
        <div>
          <h2 id="download-confirm-title" class="modal-title">Konfirmasi Unduhan</h2>
          <p class="modal-subtitle">
            Dari profil <strong>{prompt.profileName || "WhatsApp"}</strong> · {prompt.source}
          </p>
        </div>
      </div>
      <button
        class="modal-close-btn"
        type="button"
        onclick={() => onReply(false)}
        disabled={busy}
        aria-label="Batalkan unduhan"
        title="Batal (Esc)"
      >
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
          <path d="M3 3l10 10M13 3l-10 10" />
        </svg>
      </button>
    </div>

    <form onsubmit={(e) => { e.preventDefault(); onReply(true); }}>
      <div class="modal-body download-confirm-body">
        <div class="download-file-preview-card">
          <div class="file-type-tile tone-{kind.tone}">{kind.badge}</div>
          <div class="download-file-preview-meta">
            <div class="download-file-preview-name" title={name.trim() || prompt.name}>
              {name.trim() || prompt.name}
            </div>
            <div class="download-file-preview-sub">{kind.label} · Siap diunduh</div>
          </div>
        </div>

        <div class="form-group">
          <label for="download-file-name" class="form-label">Nama Berkas</label>
          <input
            id="download-file-name"
            class="modal-input"
            bind:this={nameField}
            bind:value={name}
            maxlength="200"
            autocomplete="off"
            disabled={busy}
            required
          />
        </div>

        <div class="form-group">
          <div class="download-path-label-row">
            <span class="form-label">Path Penyimpanan</span>
            <button
              class="btn btn-secondary btn-xs"
              type="button"
              onclick={onPickFolder}
              disabled={busy}
            >
              Ubah Folder…
            </button>
          </div>
          <div class="download-path-box" title={previewPath}>
            <code>{previewPath}</code>
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <div class="modal-footer-btns">
          <button class="btn btn-secondary" type="button" onclick={() => onReply(false)} disabled={busy}>
            Cancel
          </button>
          <button class="btn btn-primary" type="submit" disabled={busy}>
            {busy ? "Memproses…" : "Download"}
          </button>
        </div>
      </div>
    </form>
  </div>
</div>
