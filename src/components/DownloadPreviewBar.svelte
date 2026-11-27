<script lang="ts">
  import { fileKind, formatBytes } from "../lib/downloads";
  import type { DownloadItem } from "../lib/mygo";

  interface Props {
    item: DownloadItem;
    previewUrl?: string;
    copied: boolean;
    onCopyPath: () => void;
    onShowFolder: () => void;
    onOpenFile: () => void;
    onShowAll: () => void;
    onDismiss: () => void;
  }

  let {
    item,
    previewUrl = "",
    copied,
    onCopyPath,
    onShowFolder,
    onOpenFile,
    onShowAll,
    onDismiss,
  }: Props = $props();

  const kind = $derived(fileKind(item.name));
</script>

<section class="download-preview-bar" aria-label="Preview Unduhan">
  <div class="download-preview-left">
    <div class="download-thumb">
      {#if previewUrl}
        <img src={previewUrl} alt={item.name} class="download-thumb-img" />
      {:else}
        <div class="file-type-tile tone-{kind.tone}">{kind.badge}</div>
      {/if}
    </div>
    <div class="download-preview-details">
      <div class="download-preview-top">
        <strong class="download-preview-name" title={item.name}>{item.name}</strong>
        <span class="download-state-pill state-{item.state}">
          {#if item.state === "active"}
            Mengunduh… · {formatBytes(item.received)}
          {:else if item.state === "done"}
            Selesai · {formatBytes(item.received)}
          {:else}
            Gagal{item.error ? `: ${item.error}` : ""}
          {/if}
        </span>
        <span class="download-profile-tag">{item.profileName}</span>
      </div>
      {#if item.path}
        <div class="download-preview-path" title={item.path}>
          <span class="download-path-prefix">Path:</span>
          <code>{item.path}</code>
        </div>
      {/if}
    </div>
  </div>
  <div class="download-preview-actions">
    {#if item.path}
      <button class="btn btn-secondary btn-xs" type="button" onclick={onCopyPath}>
        {copied ? "Tersalin" : "Salin Path"}
      </button>
      <button class="btn btn-secondary btn-xs" type="button" onclick={onShowFolder}>
        Tampilkan di Folder
      </button>
    {/if}
    {#if item.state === "done"}
      <button class="btn btn-primary btn-xs" type="button" onclick={onOpenFile}>
        Buka
      </button>
    {/if}
    <button class="btn btn-secondary btn-xs" type="button" onclick={onShowAll}>
      Semua
    </button>
    <button
      class="modal-close-btn download-preview-close"
      type="button"
      onclick={onDismiss}
      aria-label="Tutup preview unduhan"
      title="Tutup preview"
    >
      <svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <path d="M3 3l10 10M13 3l-10 10" />
      </svg>
    </button>
  </div>
</section>
