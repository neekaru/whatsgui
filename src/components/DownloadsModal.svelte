<script lang="ts">
  import { fileKind, formatBytes } from "../lib/downloads";
  import type { DownloadItem } from "../lib/mygo";

  interface Props {
    downloads: DownloadItem[];
    downloadDir: string;
    previews: Record<string, string>;
    copiedId: string;
    hasActiveProfile: boolean;
    onClose: () => void;
    onCopyPath: (id: string, path: string) => void;
    onShowFolder: (id: string) => void;
    onOpenFile: (id: string) => void;
    onChangeDefaultDir: () => void;
  }

  let {
    downloads,
    downloadDir,
    previews,
    copiedId,
    hasActiveProfile,
    onClose,
    onCopyPath,
    onShowFolder,
    onOpenFile,
    onChangeDefaultDir,
  }: Props = $props();
</script>

<div
  class="modal-backdrop"
  role="presentation"
  onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
>
  <div class="modal modal-lg" role="dialog" aria-modal="true" aria-labelledby="downloads-title">
    <div class="modal-header">
      <div class="modal-header-info">
        <div class="modal-icon-badge">
          <svg viewBox="0 0 16 16" width="17" height="17" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
            <path d="M8 2v8M4.5 6.5L8 10l3.5-3.5M2.5 13h11"/>
          </svg>
        </div>
        <div class="downloads-header-meta">
          <h2 id="downloads-title" class="modal-title">Daftar Unduhan</h2>
          <p class="modal-subtitle" title={downloadDir}>
            Folder default: <code>{downloadDir || "Belum ditentukan"}</code>
          </p>
        </div>
      </div>
      <button
        class="modal-close-btn"
        type="button"
        onclick={onClose}
        aria-label="Tutup daftar unduhan"
        title="Tutup (Esc)"
      >
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
          <path d="M3 3l10 10M13 3l-10 10" />
        </svg>
      </button>
    </div>

    <div class="modal-body">
      {#if downloads.length === 0}
        <div class="info-alert">
          <p>Belum ada berkas yang diunduh pada sesi ini.</p>
        </div>
      {:else}
        <div class="downloads-list">
          {#each downloads as item (item.id)}
            {@const kind = fileKind(item.name)}
            {@const thumb = previews[item.id]}
            <div class="download-row">
              <div class="download-thumb">
                {#if thumb}
                  <img src={thumb} alt={item.name} class="download-thumb-img" />
                {:else}
                  <div class="file-type-tile tone-{kind.tone}">{kind.badge}</div>
                {/if}
              </div>
              <div class="download-row-info">
                <div class="download-row-top">
                  <span class="download-row-name" title={item.name}>{item.name}</span>
                  <span class="download-state-pill state-{item.state}">
                    {#if item.state === "active"}
                      Mengunduh… · {formatBytes(item.received)}
                    {:else if item.state === "done"}
                      Selesai · {formatBytes(item.received)}
                    {:else}
                      Gagal
                    {/if}
                  </span>
                  <span class="download-profile-tag">{item.profileName}</span>
                </div>
                {#if item.path}
                  <div class="download-row-path" title={item.path}>
                    <code>{item.path}</code>
                  </div>
                {/if}
                {#if item.error}
                  <div class="download-row-error">{item.error}</div>
                {/if}
              </div>
              <div class="download-row-actions">
                {#if item.path}
                  <button class="btn btn-secondary btn-xs" type="button" onclick={() => onCopyPath(item.id, item.path)}>
                    {copiedId === item.id ? "Tersalin" : "Salin Path"}
                  </button>
                  <button class="btn btn-secondary btn-xs" type="button" onclick={() => onShowFolder(item.id)}>
                    Folder
                  </button>
                {/if}
                {#if item.state === "done"}
                  <button class="btn btn-primary btn-xs" type="button" onclick={() => onOpenFile(item.id)}>
                    Buka
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="modal-footer">
      {#if hasActiveProfile}
        <button class="btn btn-secondary" type="button" onclick={onChangeDefaultDir}>
          Ubah Folder Default…
        </button>
      {/if}
      <div class="modal-footer-btns">
        <button class="btn btn-primary" type="button" onclick={onClose}>
          Tutup
        </button>
      </div>
    </div>
  </div>
</div>
