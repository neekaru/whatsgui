<script lang="ts">
  import { onMount } from "svelte";
  import { Workspace, events, type Userscript } from "../lib/mygo";

  interface Props {
    hasActiveProfile: boolean;
    onReloadActive: () => Promise<void> | void;
  }

  let { hasActiveProfile, onReloadActive }: Props = $props();

  const TEMPLATE = `// ==UserScript==
// @name         Skrip Baru
// @version      1.0.0
// @description  Skrip kustom untuk WhatsApp Web
// @match        https://web.whatsapp.com/*
// ==/UserScript==

(() => {
  // Tulis kode JavaScript Anda di sini
  console.log("Userscript WhatsApp GUI aktif!");
})();
`;

  let scripts = $state<Userscript[]>([]);
  let dir = $state("");
  let loading = $state(true);
  let busy = $state(false);
  let error = $state("");
  let status = $state("");
  let needsReload = $state(false);
  let editingId = $state<string | null>(null);
  let editorCode = $state("");

  const reason = (e: unknown) => (e instanceof Error ? e.message : String(e));

  function showStatus(msg: string) {
    status = msg;
    setTimeout(() => {
      if (status === msg) status = "";
    }, 4000);
  }

  async function refresh() {
    error = "";
    try {
      const [list, folder] = await Promise.all([
        Workspace.userscripts(),
        Workspace.userscriptsDirectory(),
      ]);
      scripts = list;
      if (folder) dir = folder;
    } catch (e) {
      error = reason(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    const off = events.userscriptsChanged.on((list) => {
      scripts = list;
    });
    void refresh();
    return off;
  });

  async function runAction(fn: () => Promise<void>) {
    if (busy) return;
    busy = true;
    error = "";
    try {
      await fn();
    } catch (e) {
      error = reason(e);
    } finally {
      busy = false;
    }
  }

  function startCreate() {
    error = "";
    status = "";
    editingId = "";
    editorCode = TEMPLATE;
  }

  function startEdit(script: Userscript) {
    error = "";
    status = "";
    editingId = script.id;
    editorCode = script.code;
  }

  function cancelEdit() {
    editingId = null;
    editorCode = "";
    error = "";
  }

  async function saveEditor(e: SubmitEvent) {
    e.preventDefault();
    if (editingId === null) return;
    const isUpdate = editingId !== "";
    await runAction(async () => {
      scripts = await Workspace.saveUserscript(editingId!, editorCode);
      editingId = null;
      editorCode = "";
      if (isUpdate) needsReload = true;
      showStatus("Userscript berhasil disimpan & diterapkan.");
    });
  }

  async function importFiles() {
    await runAction(async () => {
      const prevCount = scripts.length;
      scripts = await Workspace.importUserscripts();
      if (scripts.length > prevCount) {
        showStatus("Userscript berhasil diimpor & diaktifkan.");
      }
    });
  }

  async function toggleScript(script: Userscript, nextEnabled: boolean) {
    await runAction(async () => {
      scripts = await Workspace.setUserscriptEnabled(script.id, nextEnabled);
      if (!nextEnabled) needsReload = true;
      showStatus(
        nextEnabled
          ? `${script.name} diaktifkan.`
          : `${script.name} dinonaktifkan.`,
      );
    });
  }

  async function removeScript(script: Userscript) {
    await runAction(async () => {
      scripts = await Workspace.deleteUserscript(script.id);
      needsReload = true;
      showStatus(`${script.name} dihapus.`);
    });
  }

  async function installExample(id: string, label: string) {
    await runAction(async () => {
      const existed = scripts.some((s) => s.id === id);
      scripts = await Workspace.installExampleUserscript(id);
      if (existed) needsReload = true;
      showStatus(
        existed
          ? `Contoh userscript (${label}) diperbarui. Muat ulang tab untuk menerapkan.`
          : `Contoh userscript (${label}) berhasil dipasang & diaktifkan.`,
      );
    });
  }

  async function openFolder() {
    await runAction(async () => {
      await Workspace.openUserscriptsDirectory();
    });
  }

  async function reloadActiveTab() {
    await runAction(async () => {
      await onReloadActive();
      needsReload = false;
      showStatus("Tab aktif dimuat ulang.");
    });
  }
</script>

<div class="userscripts-panel" aria-label="Pengaturan Userscript">
  <div class="userscripts-toolbar">
    <div class="userscripts-actions-left">
      <button class="btn btn-primary" type="button" onclick={importFiles} disabled={busy}>
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M8 10V2M4.5 5.5L8 2l3.5 3.5M2.5 13h11"/>
        </svg>
        <span>Impor (.user.js)</span>
      </button>
      <button class="btn btn-secondary" type="button" onclick={startCreate} disabled={busy}>
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" aria-hidden="true">
          <path d="M8 2.5v11M2.5 8h11"/>
        </svg>
        <span>Buat Skrip</span>
      </button>
    </div>

    <div class="userscripts-actions-right">
      <button class="btn btn-secondary" type="button" onclick={openFolder} disabled={busy} title="Buka folder userscript di Explorer">
        <span>Buka Folder</span>
      </button>
      <button class="btn btn-secondary" type="button" onclick={() => void refresh()} disabled={busy} aria-label="Muat ulang daftar userscript" title="Muat ulang daftar">
        <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M2.5 8a5.5 5.5 0 0 1 9.3-3.9L14 6.5M14 2v4.5H9.5M13.5 8a5.5 5.5 0 0 1-9.3 3.9L2 9.5M2 14v-4.5h4.5"/>
        </svg>
      </button>
    </div>
  </div>

  {#if dir}
    <p class="userscripts-dir-hint" title={dir}>
      Folder: <code>{dir}</code>
    </p>
  {/if}

  {#if editingId !== null}
    <form class="prop-group" onsubmit={saveEditor}>
      <div class="prop-row-info">
        <span class="prop-row-label">{editingId === "" ? "Buat Userscript Baru" : `Edit ${editingId}`}</span>
        <p class="prop-row-desc">Mendukung header <code>// ==UserScript==</code> (<code>@name</code>, <code>@description</code>, <code>@version</code>, <code>@match</code>).</p>
      </div>
      <textarea
        class="userscript-code-input"
        bind:value={editorCode}
        rows="11"
        spellcheck="false"
        disabled={busy}
        aria-label="Kode userscript JavaScript"
        required
      ></textarea>
      <div class="prop-group-footer">
        <div class="prop-actions-right">
          <button class="btn btn-secondary" type="button" onclick={cancelEdit} disabled={busy}>
            Batal
          </button>
          <button class="btn btn-primary" type="submit" disabled={busy}>
            {busy ? "Menyimpan…" : "Simpan & Terapkan"}
          </button>
        </div>
      </div>
    </form>
  {:else if loading}
    <div class="loading-state">
      <p>Memuat daftar userscript…</p>
    </div>
  {:else if scripts.length === 0}
    <fieldset class="prop-group">
      <legend class="prop-group-legend">Belum Ada Userscript</legend>
      <p class="prop-hint">
        Tambahkan skrip JavaScript kustom untuk halaman WhatsApp Web (<code>https://web.whatsapp.com/*</code>) lewat impor berkas <code>.user.js</code>, tulis baru, atau pasang contoh bawaan.
      </p>
      <div class="prop-group-footer">
        <div class="prop-actions-right">
          <button class="btn btn-primary" type="button" onclick={() => void installExample("wa-audio-stickers.user.js", "Stiker Audio")} disabled={busy}>
            Pasang Contoh: Stiker Audio
          </button>
          <button class="btn btn-secondary" type="button" onclick={() => void installExample("privacy-blur-toggle.user.js", "Privacy Blur")} disabled={busy}>
            Pasang Contoh: Privacy Blur
          </button>
        </div>
      </div>
    </fieldset>
  {:else}
    <div class="userscripts-list">
      {#each scripts as script (script.id)}
        <div class="userscript-row">
          <div class="userscript-info">
            <div class="userscript-title-row">
              <label class="prop-check">
                <input
                  type="checkbox"
                  checked={script.enabled}
                  disabled={busy}
                  aria-label={`Aktifkan ${script.name}`}
                  onchange={(e) => void toggleScript(script, e.currentTarget.checked)}
                />
                <span class="userscript-name">{script.name}</span>
              </label>
              {#if script.version}
                <span class="userscript-badge">v{script.version}</span>
              {/if}
              {#if !script.matchesApp}
                <span class="userscript-badge userscript-badge-warn" title="Pola @match tidak mencakup web.whatsapp.com">Tidak cocok</span>
              {/if}
            </div>
            {#if script.description}
              <p class="userscript-desc">{script.description}</p>
            {/if}
            <div class="userscript-meta">
              <code>{script.id}</code>
              {#if script.author}
                <span>· oleh {script.author}</span>
              {/if}
            </div>
          </div>

          <div class="userscript-controls">
            <button class="btn btn-secondary btn-xs" type="button" onclick={() => startEdit(script)} disabled={busy}>
              Edit
            </button>
            <button class="btn btn-secondary btn-xs userscript-delete-btn" type="button" onclick={() => void removeScript(script)} disabled={busy}>
              Hapus
            </button>
          </div>
        </div>
      {/each}
    </div>

    <div class="userscripts-example-bar">
      <span>Contoh bawaan (pasang / perbarui):</span>
      <div class="userscripts-actions-right">
        <button class="btn btn-secondary btn-xs" type="button" onclick={() => void installExample("wa-audio-stickers.user.js", "Stiker Audio")} disabled={busy}>
          {scripts.some((s) => s.id === "wa-audio-stickers.user.js") ? "Perbarui Stiker Audio" : "Stiker Audio"}
        </button>
        <button class="btn btn-secondary btn-xs" type="button" onclick={() => void installExample("privacy-blur-toggle.user.js", "Privacy Blur")} disabled={busy}>
          {scripts.some((s) => s.id === "privacy-blur-toggle.user.js") ? "Perbarui Privacy Blur" : "Privacy Blur"}
        </button>
      </div>
    </div>
  {/if}

  {#if needsReload && hasActiveProfile}
    <div class="info-alert userscripts-reload-alert">
      <p>Skrip yang baru dimatikan, diubah, atau dihapus akan bersih sepenuhnya setelah tab aktif dimuat ulang.</p>
      <button class="btn btn-secondary btn-xs" type="button" onclick={reloadActiveTab} disabled={busy}>
        Muat Ulang Tab
      </button>
    </div>
  {/if}

  {#if status}
    <div class="status-badge success" role="status">
      <span>{status}</span>
    </div>
  {/if}

  {#if error}
    <div class="status-badge error" role="alert">
      <span>{error}</span>
    </div>
  {/if}
</div>
