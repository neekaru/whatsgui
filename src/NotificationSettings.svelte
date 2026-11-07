<script lang="ts">
  import { onMount } from "svelte";
  import { Workspace, events, type DNDSettings } from "./mygo";

  let settings = $state<DNDSettings | null>(null);
  let saving = $state(false);
  let error = $state("");
  let saved = $state(false);

  onMount(() => {
    let live = true;
    const off = events.dndChanged.on((value) => { settings = value; });
    void Workspace.getDND().then((value) => { if (live) settings = value; })
      .catch((e: unknown) => { if (live) error = String(e); });
    return () => { live = false; off(); };
  });

  async function save() {
    if (!settings || saving) return;
    saving = true;
    error = "";
    saved = false;
    try {
      await Workspace.setDND(settings);
      saved = true;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }
</script>

<section class="settings-section" aria-label="Notifikasi">
  <h3>Notifikasi</h3>
  {#if settings}
    <div class="settings-grid">
      <label class="setting-toggle">
        <input type="checkbox" bind:checked={settings.enabled} disabled={saving} /> Jangan ganggu
      </label>
      <label class="setting-toggle">
        <input type="checkbox" bind:checked={settings.scheduleEnabled} disabled={saving} /> Jadwalkan setiap hari
      </label>
      {#if settings.scheduleEnabled}
        <label class="setting"><span>Mulai</span><input type="time" bind:value={settings.start} disabled={saving} required /></label>
        <label class="setting"><span>Selesai</span><input type="time" bind:value={settings.end} disabled={saving} required /></label>
      {/if}
      <button class="btn" type="button" onclick={save} disabled={saving}>{saving ? "Menyimpan…" : "Simpan notifikasi"}</button>
    </div>
    <p>Jangan ganggu menonaktifkan popup dan suara; jumlah belum dibaca tetap diperbarui.</p>
    {#if saved}<p role="status">Pengaturan notifikasi disimpan.</p>{/if}
  {:else if !error}
    <p>Memuat pengaturan notifikasi…</p>
  {/if}
  {#if error}<p class="settings-error" role="alert">{error}</p>{/if}
</section>
