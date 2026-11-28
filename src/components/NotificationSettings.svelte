<script lang="ts">
  import { onMount } from "svelte";
  import {
    Workspace,
    events,
    type DNDSettings,
    type NotificationSoundChoice,
    type NotificationSoundSettings,
  } from "../lib/mygo";
  import {
    CALL_SOUND_PRESETS,
    MESSAGE_SOUND_PRESETS,
    createPresetWavUrl,
    type SoundKind,
  } from "../lib/notificationSounds";

  let settings = $state<DNDSettings | null>(null);
  let sounds = $state<NotificationSoundSettings | null>(null);
  let saving = $state(false);
  let soundSaving = $state(false);
  let error = $state("");
  let saved = $state(false);
  let soundSaved = $state(false);
  let playingKind = $state<SoundKind | null>(null);

  let previewAudio: HTMLAudioElement | null = null;
  let previewObjectUrl = "";
  let soundSavedTimer: ReturnType<typeof setTimeout> | undefined;

  const channels = [
    {
      kind: "message" as const,
      title: "Pesan Masuk",
      presets: MESSAGE_SOUND_PRESETS,
    },
    {
      kind: "call" as const,
      title: "Telepon Masuk",
      presets: CALL_SOUND_PRESETS,
    },
  ];

  function stopPreview() {
    if (previewAudio) {
      previewAudio.pause();
      previewAudio.currentTime = 0;
      previewAudio = null;
    }
    if (previewObjectUrl) {
      URL.revokeObjectURL(previewObjectUrl);
      previewObjectUrl = "";
    }
    playingKind = null;
  }

  onMount(() => {
    let live = true;
    const offDnd = events.dndChanged.on((value) => {
      settings = value;
    });
    const offSounds = events.notificationSoundsChanged.on((value) => {
      sounds = value;
    });
    void Promise.all([Workspace.getDND(), Workspace.getNotificationSounds()])
      .then(([dndValue, soundValue]) => {
        if (!live) return;
        settings = dndValue;
        sounds = soundValue;
      })
      .catch((e: unknown) => {
        if (live) error = e instanceof Error ? e.message : String(e);
      });
    return () => {
      live = false;
      clearTimeout(soundSavedTimer);
      stopPreview();
      offDnd();
      offSounds();
    };
  });

  function showSoundSaved() {
    soundSaved = true;
    clearTimeout(soundSavedTimer);
    soundSavedTimer = setTimeout(() => {
      soundSaved = false;
    }, 2500);
  }

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
      }, 3000);
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      saving = false;
    }
  }

  async function commitSounds(next: NotificationSoundSettings) {
    if (soundSaving) return;
    soundSaving = true;
    error = "";
    try {
      await Workspace.setNotificationSounds(next);
      sounds = next;
      showSoundSaved();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      soundSaving = false;
    }
  }

  async function onModeChange(kind: SoundKind, mode: string) {
    if (!sounds) return;
    stopPreview();
    if (mode === "custom" && !sounds[kind].fileName) {
      await chooseCustomSound(kind);
      return;
    }
    const next: NotificationSoundSettings = {
      ...sounds,
      [kind]: { ...sounds[kind], mode },
    };
    sounds = next;
    await commitSounds(next);
  }

  async function onPresetChange(kind: SoundKind, preset: string) {
    if (!sounds) return;
    stopPreview();
    const next: NotificationSoundSettings = {
      ...sounds,
      [kind]: { ...sounds[kind], mode: "preset", preset },
    };
    sounds = next;
    await commitSounds(next);
  }

  async function onVolumeCommit(kind: SoundKind, volume: number) {
    if (!sounds) return;
    const clamped = Math.min(100, Math.max(10, Math.round(volume)));
    const next: NotificationSoundSettings = {
      ...sounds,
      [kind]: { ...sounds[kind], volume: clamped },
    };
    sounds = next;
    if (previewAudio && playingKind === kind) {
      previewAudio.volume = clamped / 100;
    }
    await commitSounds(next);
  }

  async function chooseCustomSound(kind: SoundKind) {
    if (soundSaving) return;
    stopPreview();
    soundSaving = true;
    error = "";
    try {
      const updated = await Workspace.chooseNotificationSound(kind);
      sounds = updated;
      if (updated[kind].mode === "custom") {
        showSoundSaved();
      }
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      soundSaving = false;
    }
  }

  async function clearCustomSound(kind: SoundKind) {
    if (soundSaving) return;
    stopPreview();
    soundSaving = true;
    error = "";
    try {
      sounds = await Workspace.clearNotificationSound(kind);
      showSoundSaved();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      soundSaving = false;
    }
  }

  async function togglePreview(kind: SoundKind, choice: NotificationSoundChoice) {
    if (playingKind === kind) {
      stopPreview();
      return;
    }
    stopPreview();
    error = "";
    try {
      let src = "";
      if (choice.mode === "custom") {
        src = await Workspace.notificationSoundPreview(kind);
        if (!src) {
          error = "Berkas audio kustom belum tersedia.";
          return;
        }
      } else {
        const fallbackPreset = kind === "call" ? "classic" : "chime";
        previewObjectUrl = createPresetWavUrl(kind, choice.preset || fallbackPreset);
        src = previewObjectUrl;
      }
      const audio = new Audio(src);
      audio.volume = Math.min(1, Math.max(0.1, (choice.volume || 100) / 100));
      audio.loop = kind === "call";
      audio.onended = () => {
        if (previewAudio === audio) stopPreview();
      };
      previewAudio = audio;
      playingKind = kind;
      await audio.play();
    } catch (e) {
      stopPreview();
      error = e instanceof Error ? e.message : String(e);
    }
  }
</script>

<div class="notifications-panel" aria-label="Pengaturan Notifikasi">
  {#if settings}
    <fieldset class="prop-group">
      <legend class="prop-group-legend">Jangan Ganggu (DND)</legend>

      <div class="prop-row">
        <label class="prop-check">
          <input type="checkbox" bind:checked={settings.enabled} disabled={saving} />
          <span>Senyapkan notifikasi pop-up dan suara masuk</span>
        </label>
      </div>

      <div class="prop-row">
        <label class="prop-check">
          <input type="checkbox" bind:checked={settings.scheduleEnabled} disabled={saving} />
          <span>Jadwal otomatis harian</span>
        </label>

        {#if settings.scheduleEnabled}
          <div class="prop-inline-times">
            <input
              id="dnd-start"
              type="time"
              class="time-input compact"
              aria-label="Waktu mulai DND"
              bind:value={settings.start}
              disabled={saving}
              required
            />
            <span>–</span>
            <input
              id="dnd-end"
              type="time"
              class="time-input compact"
              aria-label="Waktu selesai DND"
              bind:value={settings.end}
              disabled={saving}
              required
            />
          </div>
        {/if}
      </div>

      <div class="prop-group-footer">
        <span class="prop-hint">Angka pesan belum dibaca tetap diperbarui.</span>
        <div class="prop-actions-right">
          {#if saved}
            <span class="status-badge success" role="status">Tersimpan</span>
          {/if}
          <button class="btn btn-primary btn-classic" type="button" onclick={save} disabled={saving}>
            {saving ? "Menyimpan…" : "Simpan DND"}
          </button>
        </div>
      </div>
    </fieldset>

    {#if sounds}
      <fieldset class="prop-group">
        <legend class="prop-group-legend">Suara Notifikasi &amp; Telepon</legend>

        {#each channels as ch (ch.kind)}
          {@const choice = sounds[ch.kind]}
          <div class="sound-row">
            <div class="sound-row-head">
              <span class="sound-row-title">{ch.title}</span>
              <span class="sound-row-file" title={choice.fileName || ""}>
                {#if choice.mode === "custom" && choice.fileName}
                  {choice.fileName}
                {:else if choice.mode === "mute"}
                  Disenyapkan
                {:else if choice.mode === "preset"}
                  Nada aplikasi
                {:else}
                  Bawaan WhatsApp
                {/if}
              </span>
            </div>

            <div class="sound-row-controls">
              <select
                id="sound-mode-{ch.kind}"
                class="sound-select"
                aria-label="Sumber suara {ch.title}"
                value={choice.mode}
                disabled={soundSaving}
                onchange={(e) => void onModeChange(ch.kind, e.currentTarget.value)}
              >
                <option value="default">Bawaan WhatsApp</option>
                <option value="preset">Nada Aplikasi</option>
                <option value="custom">File MP3 / Audio…</option>
                <option value="mute">Senyap (Bisukan)</option>
              </select>

              {#if choice.mode === "preset"}
                <select
                  id="sound-preset-{ch.kind}"
                  class="sound-select"
                  aria-label="Preset {ch.title}"
                  value={choice.preset || ch.presets[0]?.id || ""}
                  disabled={soundSaving}
                  onchange={(e) => void onPresetChange(ch.kind, e.currentTarget.value)}
                >
                  {#each ch.presets as preset (preset.id)}
                    <option value={preset.id}>{preset.label}</option>
                  {/each}
                </select>
              {/if}

              <button
                class="btn btn-secondary btn-classic"
                type="button"
                disabled={soundSaving}
                onclick={() => void chooseCustomSound(ch.kind)}
              >
                {choice.fileName ? "Ganti MP3…" : "Pilih MP3…"}
              </button>

              {#if choice.fileName}
                <button
                  class="btn btn-secondary btn-classic"
                  type="button"
                  disabled={soundSaving}
                  onclick={() => void clearCustomSound(ch.kind)}
                >
                  Hapus
                </button>
              {/if}

              {#if choice.mode === "preset" || choice.mode === "custom"}
                <div class="sound-volume-inline">
                  <input
                    id="sound-vol-{ch.kind}"
                    type="range"
                    min="10"
                    max="100"
                    step="5"
                    aria-label="Volume {ch.title}"
                    value={choice.volume}
                    disabled={soundSaving}
                    oninput={(e) => {
                      const val = Number(e.currentTarget.value);
                      if (sounds) {
                        sounds = { ...sounds, [ch.kind]: { ...sounds[ch.kind], volume: val } };
                      }
                      if (previewAudio && playingKind === ch.kind) {
                        previewAudio.volume = val / 100;
                      }
                    }}
                    onchange={(e) => void onVolumeCommit(ch.kind, Number(e.currentTarget.value))}
                  />
                  <span>{choice.volume}%</span>
                  <button
                    class="btn btn-secondary btn-classic"
                    type="button"
                    disabled={soundSaving || (choice.mode === "custom" && !choice.fileName)}
                    onclick={() => void togglePreview(ch.kind, choice)}
                  >
                    {playingKind === ch.kind ? "Stop" : "Uji"}
                  </button>
                </div>
              {/if}
            </div>
          </div>
        {/each}

        <div class="prop-group-footer">
          <span class="prop-hint">Mendukung MP3, WAV, OGG, M4A, FLAC (maks. 7 MB).</span>
          <div class="prop-actions-right">
            {#if soundSaved}
              <span class="status-badge success" role="status">Diterapkan</span>
            {/if}
          </div>
        </div>
      </fieldset>
    {/if}
  {:else if !error}
    <div class="loading-state">
      <p>Memuat pengaturan notifikasi…</p>
    </div>
  {/if}

  {#if error}
    <div class="status-badge error" role="alert">
      <span>{error}</span>
    </div>
  {/if}
</div>
