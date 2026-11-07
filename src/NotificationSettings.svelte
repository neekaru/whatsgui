<script lang="ts">
  import { onMount } from "svelte";
  import { Workspace, events, type DNDSettings } from "./mygo";

  let settings = $state<DNDSettings | null>(null);
  let saving = $state(false);
  let error = $state("");
  let saved = $state(false);

  onMount(() => {
    let live = true;
    const off = events.dndChanged.on((value) => {
      settings = value;
    });
    void Workspace.getDND()
      .then((value) => {
        if (live) settings = value;
      })
      .catch((e: unknown) => {
        if (live) error = String(e);
      });
    return () => {
      live = false;
      off();
    };
  });

  async function save() {
    if (!settings || saving) return;
    saving = true;
    error = "";
    saved = false;
    try {
      await Workspace.setDND(settings);
      saved = true;
      setTimeout(() => {
        saved = false;
      }, 3500);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }
</script>

<div class="notifications-panel" aria-label="Pengaturan Notifikasi">
  {#if settings}
    <div class="setting-cards-stack">
      <!-- Toggle Card 1: DND -->
      <label class="setting-card-toggle">
        <div class="setting-card-info">
          <div class="setting-card-title">
            <svg class="card-icon" viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12.5 10.5A6 6 0 0 1 5.5 3.5a6 6 0 1 0 7 7z"/>
            </svg>
            <span>Jangan Ganggu (DND)</span>
          </div>
          <p class="setting-card-desc">Senyapkan notifikasi pop-up dan suara pesan masuk. Lencana angka pesan belum dibaca tetap diperbarui.</p>
        </div>
        <div class="switch">
          <input type="checkbox" bind:checked={settings.enabled} disabled={saving} />
          <span class="slider"></span>
        </div>
      </label>

      <!-- Toggle Card 2: Schedule -->
      <label class="setting-card-toggle">
        <div class="setting-card-info">
          <div class="setting-card-title">
            <svg class="card-icon" viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="8" cy="8" r="6"/>
              <path d="M8 5v3l2 1.5"/>
            </svg>
            <span>Jadwal Harian Otomatis</span>
          </div>
          <p class="setting-card-desc">Aktifkan mode Jangan Ganggu secara otomatis pada jam kerja atau jam istirahat tertentu setiap hari.</p>
        </div>
        <div class="switch">
          <input type="checkbox" bind:checked={settings.scheduleEnabled} disabled={saving} />
          <span class="slider"></span>
        </div>
      </label>

      <!-- Schedule Time Inputs -->
      {#if settings.scheduleEnabled}
        <div class="schedule-config">
          <div class="schedule-field">
            <label for="dnd-start" class="field-label">
              <span>Waktu Mulai</span>
            </label>
            <input id="dnd-start" type="time" class="time-input" bind:value={settings.start} disabled={saving} required />
          </div>
          <div class="schedule-field">
            <label for="dnd-end" class="field-label">
              <span>Waktu Selesai</span>
            </label>
            <input id="dnd-end" type="time" class="time-input" bind:value={settings.end} disabled={saving} required />
          </div>
        </div>
      {/if}
    </div>

    <div class="notifications-actions">
      <button class="btn btn-primary" type="button" onclick={save} disabled={saving}>
        {#if saving}
          <span>Menyimpan…</span>
        {:else}
          <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="3.5 8.5 6.5 11.5 12.5 4.5" />
          </svg>
          <span>Simpan Pengaturan Notifikasi</span>
        {/if}
      </button>

      {#if saved}
        <div class="status-badge success" role="status">
          <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor">
            <path d="M8 0a8 8 0 1 0 0 16A8 8 0 0 0 8 0zm3.4 5.9l-4.2 4.2a.8.8 0 0 1-1.1 0L4.6 8.6a.8.8 0 0 1 1.1-1.1l1 1 3.6-3.7a.8.8 0 0 1 1.1 1.1z"/>
          </svg>
          <span>Pengaturan notifikasi berhasil disimpan.</span>
        </div>
      {/if}
    </div>
  {:else if !error}
    <div class="loading-state">
      <p>Memuat pengaturan notifikasi…</p>
    </div>
  {/if}

  {#if error}
    <div class="status-badge error" role="alert">
      <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor">
        <path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm0 3.5a.8.8 0 0 1 .8.8v3.4a.8.8 0 1 1-1.6 0V5.3a.8.8 0 0 1 .8-.8zm0 7a1 1 0 1 1 0-2 1 1 0 0 1 0 2z"/>
      </svg>
      <span>{error}</span>
    </div>
  {/if}
</div>
