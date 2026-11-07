<script lang="ts">
  import { onMount, tick } from "svelte";
  import { currentWindow } from "mygo-runtime";
  import { Workspace, events, type State, type MediaSettings } from "./mygo";
  import NotificationSettings from "./NotificationSettings.svelte";

  type Pending = "startup" | "create" | "rename" | "remove" | "activate" | "reload" | "cache" | "devices" | "devtools";
  type Form = "none" | "create" | "rename" | "delete";
  type SettingsTab = "appearance" | "devices" | "notifications" | "maintenance";

  let model = $state<State>({ tabs: [], activeId: "" });
  let loading = $state(true);
  let pending = $state<Pending | null>(null);
  let error = $state("");
  let form = $state<Form>("none");
  let name = $state("");
  let target = $state<string | null>(null);
  let maximized = $state(false);
  let settingsOpen = $state(false);
  let settingsTab = $state<SettingsTab>("appearance");
  let theme = $state<"system" | "light" | "dark">("system");
  let devices = $state<MediaSettings | null>(null);
  let audioId = $state("");
  let videoId = $state("");
  let devicesLoading = $state(false);
  let settingsMessage = $state("");
  let deviceRequest = 0;

  let view = $state<HTMLElement>();
  let tablist = $state<HTMLElement>();
  let nameField = $state<HTMLInputElement>();
  let createButton = $state<HTMLButtonElement>();

  const busy = $derived(pending !== null);
  const active = $derived(model.tabs.find((tab) => tab.id === model.activeId) ?? null);
  const targetTab = $derived(model.tabs.find((tab) => tab.id === target) ?? null);
  const focusId = $derived(active?.id ?? model.tabs[0]?.id ?? "");
  const isModalOpen = $derived(settingsOpen || form !== "none");

  const reason = (e: unknown) => (e instanceof Error ? e.message : String(e));

  async function run<T>(kind: Pending, fn: () => Promise<T>): Promise<T | undefined> {
    pending = kind;
    error = "";
    try {
      return await fn();
    } catch (e) {
      error = reason(e);
      return undefined;
    } finally {
      pending = null;
    }
  }

  function openForm(next: Form, id: string | null = null, value = "") {
    error = "";
    form = next;
    target = id;
    name = value;
  }

  async function closeForm() {
    form = "none";
    target = null;
    name = "";
    await tick();
    const buttons = tablist?.querySelectorAll<HTMLButtonElement>('[role="tab"]');
    const index = model.tabs.findIndex((tab) => tab.id === model.activeId);
    (index >= 0 ? buttons?.[index] : createButton)?.focus();
  }

  async function submitForm(e: SubmitEvent) {
    e.preventDefault();
    const value = name.trim();
    if (!value) {
      error = "Nama profil tidak boleh kosong.";
      nameField?.focus();
      return;
    }
    if (form === "create") {
      const next = await run("create", () => Workspace.create(value));
      if (next) {
        model = next;
        await closeForm();
      }
    } else if (form === "rename") {
      const id = target;
      if (!id) return;
      const next = await run("rename", () => Workspace.rename(id, value));
      if (next) {
        model = next;
        await closeForm();
      }
    }
  }

  async function confirmRemove() {
    const id = target;
    if (!id) return;
    const next = await run("remove", () => Workspace.remove(id));
    if (next) {
      model = next;
      await closeForm();
    }
  }

  async function activate(id: string) {
    if (busy || id === model.activeId) return;
    const next = await run("activate", () => Workspace.activate(id));
    if (next) model = next;
  }

  async function reloadProfile() {
    await run("reload", () => Workspace.reload());
  }

  function setTheme(value: "system" | "light" | "dark") {
    theme = value;
    document.documentElement.dataset.theme = value;
    localStorage.setItem("workspace-theme", value);
    void Workspace.setTheme(value).catch(() => {});
  }

  function openSettings() {
    settingsOpen = true;
    settingsMessage = "";
    error = "";
  }

  async function loadDevices(id: string) {
    const request = ++deviceRequest;
    devices = null;
    if (!id) return;
    error = "";
    devicesLoading = true;
    try {
      const result = await Workspace.mediaDevices();
      if (request !== deviceRequest) return;
      devices = result;
      audioId = result.audioId;
      videoId = result.videoId;
    } catch (e) {
      if (request === deviceRequest) error = reason(e);
    } finally {
      if (request === deviceRequest) devicesLoading = false;
    }
  }

  async function saveDevices() {
    settingsMessage = "";
    const ok = await run("devices", async () => {
      await Workspace.setMediaDevices(audioId, videoId);
      return true;
    });
    if (ok) settingsMessage = "Perangkat audio & video disimpan untuk panggilan berikutnya.";
  }

  async function clearCache() {
    settingsMessage = "";
    deviceRequest++;
    devicesLoading = false;
    devices = null;
    const ok = await run("cache", async () => {
      await Workspace.clearCache();
      return true;
    });
    if (ok) {
      settingsMessage = "Cache profil aktif berhasil dibersihkan.";
      if (settingsOpen && model.activeId) void loadDevices(model.activeId);
    }
  }

  $effect(() => {
    const id = model.activeId;
    if (settingsOpen) void loadDevices(id);
    else {
      deviceRequest++;
      devicesLoading = false;
    }
  });

  // Focus and select input field whenever create or rename modal opens
  $effect(() => {
    if (form === "create" || form === "rename") {
      void tick().then(() => {
        nameField?.focus();
        nameField?.select();
      });
    }
  });

  // Manage child HWND position and hide it when any modal is open
  $effect(() => {
    const el = view;
    const activeId = model.activeId;
    const modalActive = isModalOpen;
    if (!el) return;

    let key = "";
    const report = () => {
      const rect = el.getBoundingClientRect();
      const width = modalActive ? 0 : Math.round(rect.width);
      const height = modalActive ? 0 : Math.round(rect.height);
      if (!modalActive && (!width || !height)) return;
      const x = Math.round(rect.left);
      const y = Math.round(rect.top);
      const scale = window.devicePixelRatio;
      const next = `${activeId}|${x}|${y}|${width}|${height}|${scale}|${modalActive}`;
      if (next === key) return;
      key = next;
      void Workspace.resize({ x, y, width, height, scale }).catch((e: unknown) => {
        error = reason(e);
      });
    };

    const observer = new ResizeObserver(report);
    observer.observe(el);
    window.addEventListener("resize", report);
    window.visualViewport?.addEventListener("resize", report);

    let ratio: MediaQueryList | undefined;
    const onRatio = () => {
      ratio?.removeEventListener("change", onRatio);
      report();
      watchRatio();
    };
    const watchRatio = () => {
      ratio = window.matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`);
      ratio.addEventListener("change", onRatio);
    };
    watchRatio();
    report();

    return () => {
      observer.disconnect();
      window.removeEventListener("resize", report);
      window.visualViewport?.removeEventListener("resize", report);
      ratio?.removeEventListener("change", onRatio);
    };
  });

  async function syncMaximized() {
    try {
      maximized = await currentWindow.isMaximized();
    } catch {
      maximized = false;
    }
  }

  async function windowAction(action: "minimize" | "maximize" | "close") {
    try {
      if (action === "close") {
        await currentWindow.close();
        return;
      }
      if (action === "minimize") await currentWindow.minimize();
      else await currentWindow.toggleMaximize();
      await syncMaximized();
    } catch (e) {
      error = reason(e);
    }
  }

  function focusTab(index: number) {
    tablist?.querySelectorAll<HTMLButtonElement>('[role="tab"]')[index]?.focus();
  }

  function onTabsWheel(e: WheelEvent) {
    const el = e.currentTarget as HTMLElement;
    if (e.ctrlKey || e.deltaX || el.scrollWidth <= el.clientWidth) return;
    el.scrollLeft += e.deltaY;
    e.preventDefault();
  }

  function onTabKey(e: KeyboardEvent, index: number) {
    const count = model.tabs.length;
    let next = -1;
    if (e.key === "ArrowRight") next = (index + 1) % count;
    else if (e.key === "ArrowLeft") next = (index - 1 + count) % count;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = count - 1;
    if (next < 0) return;
    e.preventDefault();
    const tab = model.tabs[next];
    if (!tab) return;
    focusTab(next);
    void activate(tab.id);
  }

  onMount(() => {
    const storedTheme = localStorage.getItem("workspace-theme");
    setTheme(storedTheme === "dark" || storedTheme === "light" ? storedTheme : "system");
    const offState = events.workspaceChanged.on((next) => {
      model = next;
      if (target && !next.tabs.some((tab) => tab.id === target)) {
        form = "none";
        target = null;
        name = "";
      }
    });
    const offError = events.workerError.on((text) => {
      error = text;
    });
    const onEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        if (form !== "none") {
          e.preventDefault();
          void closeForm();
        } else if (settingsOpen) {
          e.preventDefault();
          settingsOpen = false;
        }
      }
    };
    window.addEventListener("keydown", onEscape);

    const onResize = () => void syncMaximized();
    window.addEventListener("resize", onResize);
    void syncMaximized();

    void (async () => {
      const next = await run("startup", () => Workspace.state());
      if (next) model = next;
      loading = false;
    })();

    return () => {
      offState();
      offError();
      window.removeEventListener("keydown", onEscape);
      window.removeEventListener("resize", onResize);
    };
  });
</script>

<div class="app">
  <!-- Titlebar Tabs and Window Captions -->
  <header class="titlebar">
    <div class="tabs" role="tablist" aria-label="Profil WhatsApp" bind:this={tablist} onwheel={onTabsWheel}>
      {#each model.tabs as tab, index (tab.id)}
        <button
          type="button"
          role="tab"
          id={`tab-${tab.id}`}
          class="tab"
          title={tab.name}
          aria-label={tab.unread > 0 ? `${tab.name}, ${tab.unread} belum dibaca` : tab.name}
          aria-selected={tab.id === model.activeId}
          aria-controls="profile-view"
          tabindex={tab.id === focusId ? 0 : -1}
          onclick={() => activate(tab.id)}
          onkeydown={(e) => onTabKey(e, index)}
        >
          <span class="tab-label">{tab.name}</span>
          {#if tab.unread > 0}
            <span class="tab-count" aria-hidden="true">{tab.unread > 99 ? "99+" : tab.unread}</span>
          {/if}
        </button>
      {/each}
    </div>
    <button
      type="button"
      class="tab-add"
      bind:this={createButton}
      onclick={() => openForm("create")}
      disabled={busy}
      aria-label="Tambah Profil Baru"
      title="Tambah Profil Baru"
    >
      <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true" focusable="false">
        <path d="M6 1v10M1 6h10" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" />
      </svg>
    </button>
    <div class="drag-space" aria-hidden="true"></div>
    <div class="captions">
      <button
        type="button"
        class="caption"
        onclick={() => windowAction("minimize")}
        aria-label="Minimalkan jendela"
        title="Minimalkan"
      >
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" focusable="false">
          <path d="M0 5.5h10" fill="none" stroke="currentColor" stroke-width="1" />
        </svg>
      </button>
      <button
        type="button"
        class="caption"
        onclick={() => windowAction("maximize")}
        aria-label={maximized ? "Pulihkan jendela" : "Maksimalkan jendela"}
        title={maximized ? "Pulihkan" : "Maksimalkan"}
      >
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" focusable="false">
          {#if maximized}
            <path d="M2.5 2.5v-2h7v7h-2" fill="none" stroke="currentColor" stroke-width="1" />
            <rect x="0.5" y="2.5" width="7" height="7" fill="none" stroke="currentColor" stroke-width="1" />
          {:else}
            <rect x="0.5" y="0.5" width="9" height="9" fill="none" stroke="currentColor" stroke-width="1" />
          {/if}
        </svg>
      </button>
      <button
        type="button"
        class="caption caption-close"
        onclick={() => windowAction("close")}
        aria-label="Tutup jendela"
        title="Tutup"
      >
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" focusable="false">
          <path d="M0.5 0.5l9 9M9.5 0.5l-9 9" fill="none" stroke="currentColor" stroke-width="1" />
        </svg>
      </button>
    </div>
  </header>

  <!-- Clean, Organized Toolbar (Sub-Header) -->
  <div class="toolbar" aria-busy={busy}>
    <div class="profile-header">
      <div class="profile-avatar" aria-hidden="true">
        <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
          <path d="M12.04 2c-5.46 0-9.91 4.45-9.91 9.91 0 1.75.46 3.45 1.32 4.95L2.05 22l5.25-1.38c1.45.79 3.08 1.21 4.74 1.21 5.46 0 9.91-4.45 9.91-9.91 0-2.65-1.03-5.14-2.9-7.01A9.82 9.82 0 0 0 12.04 2z"/>
        </svg>
      </div>
      <div class="profile-meta">
        <div class="profile-title-row">
          <h1 class="profile-title" title={active?.name}>{active?.name ?? "Tidak ada profil aktif"}</h1>
          {#if active}
            <span class="profile-badge">
              <span class="badge-dot"></span>
              <span>Aktif</span>
            </span>
            {#if active.unread > 0}
              <span class="profile-unread-pill">{active.unread} belum dibaca</span>
            {/if}
          {/if}
        </div>
      </div>
    </div>

    <div class="toolbar-actions">
      {#if active}
        <div class="action-cluster">
          <button
            class="toolbar-btn"
            type="button"
            disabled={busy}
            onclick={reloadProfile}
            title="Muat ulang halaman profil (F5)"
          >
            <svg class="t-icon {pending === 'reload' ? 'spin' : ''}" viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M2.5 8a5.5 5.5 0 0 1 9.3-3.9L14 6.5M14 2v4.5H9.5M13.5 8a5.5 5.5 0 0 1-9.3 3.9L2 9.5M2 14v-4.5h4.5"/>
            </svg>
            <span>{pending === "reload" ? "Memuat…" : "Muat ulang"}</span>
          </button>

          <button
            class="toolbar-btn"
            type="button"
            disabled={busy}
            onclick={() => active && openForm("rename", active.id, active.name)}
            title="Ubah nama profil"
          >
            <svg class="t-icon" viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11.5 2.5a1.77 1.77 0 0 1 2.5 2.5L5 14l-3.5 1 1-3.5 9-9z"/>
            </svg>
            <span>Ganti nama</span>
          </button>

          <button
            class="toolbar-btn toolbar-btn-danger"
            type="button"
            disabled={busy}
            onclick={() => active && openForm("delete", active.id)}
            title="Hapus profil WhatsApp ini"
          >
            <svg class="t-icon" viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3 4h10M6 4V2.5A1.5 1.5 0 0 1 7.5 1h1A1.5 1.5 0 0 1 10 2.5V4M12.5 4v9.5a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5V4"/>
            </svg>
            <span>Hapus</span>
          </button>
        </div>

        <div class="toolbar-separator" aria-hidden="true"></div>
      {/if}

      <button
        class="toolbar-btn toolbar-btn-settings"
        type="button"
        disabled={busy}
        aria-expanded={settingsOpen}
        onclick={openSettings}
        title="Buka Pengaturan"
      >
        <svg class="t-icon gear" viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="8" cy="8" r="2.5"/>
          <path d="M13.5 8a5.5 5.5 0 0 0-.1-1l1.4-1.1-1.2-2.1-1.7.6a5.6 5.6 0 0 0-1.7-1l-.3-1.8H7.6l-.3 1.8a5.6 5.6 0 0 0-1.7 1L3.9 3.8 2.7 5.9l1.4 1.1a5.5 5.5 0 0 0 0 2L2.7 10.1l1.2 2.1 1.7-.6a5.6 5.6 0 0 0 1.7 1l.3 1.8h2.8l.3-1.8a5.6 5.6 0 0 0 1.7-1l1.7.6 1.2-2.1-1.4-1.1c.1-.3.1-.7.1-1z"/>
        </svg>
        <span>Pengaturan</span>
      </button>
    </div>
  </div>

  <!-- Global Error Banner -->
  {#if error}
    <div class="app-alert" role="alert">
      <div class="alert-icon">
        <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor">
          <path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm0 3.5a.8.8 0 0 1 .8.8v3.4a.8.8 0 1 1-1.6 0V5.3a.8.8 0 0 1 .8-.8zm0 7a1 1 0 1 1 0-2 1 1 0 0 1 0 2z"/>
        </svg>
      </div>
      <p class="alert-text">{error}</p>
      <button class="alert-close" type="button" onclick={() => (error = "")} aria-label="Tutup notifikasi error">
        <svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M3 3l10 10M13 3l-10 10" />
        </svg>
      </button>
    </div>
  {/if}

  <!-- SETTINGS MODAL -->
  {#if settingsOpen}
    <div
      class="modal-backdrop"
      role="presentation"
      onclick={(e) => { if (e.target === e.currentTarget) settingsOpen = false; }}
    >
      <div
        class="modal modal-lg"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-title"
      >
        <div class="modal-header">
          <div class="modal-header-info">
            <div class="modal-icon-badge">
              <svg viewBox="0 0 16 16" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="8" cy="8" r="2.5"/>
                <path d="M13.5 8a5.5 5.5 0 0 0-.1-1l1.4-1.1-1.2-2.1-1.7.6a5.6 5.6 0 0 0-1.7-1l-.3-1.8H7.6l-.3 1.8a5.6 5.6 0 0 0-1.7 1L3.9 3.8 2.7 5.9l1.4 1.1a5.5 5.5 0 0 0 0 2L2.7 10.1l1.2 2.1 1.7-.6a5.6 5.6 0 0 0 1.7 1l.3 1.8h2.8l.3-1.8a5.6 5.6 0 0 0 1.7-1l1.7.6 1.2-2.1-1.4-1.1c.1-.3.1-.7.1-1z"/>
              </svg>
            </div>
            <div>
              <h2 id="settings-title" class="modal-title">Pengaturan</h2>
              <p class="modal-subtitle">
                {#if active}
                  Preferensi profil <strong>{active.name}</strong> dan konfigurasi aplikasi
                {:else}
                  Preferensi aplikasi WhatsApp GUI
                {/if}
              </p>
            </div>
          </div>
          <button
            class="modal-close-btn"
            type="button"
            onclick={() => (settingsOpen = false)}
            aria-label="Tutup pengaturan"
            title="Tutup (Esc)"
          >
            <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
              <path d="M3 3l10 10M13 3l-10 10" />
            </svg>
          </button>
        </div>

        <nav class="modal-tabs" aria-label="Kategori Pengaturan">
          <button
            type="button"
            class="modal-tab-btn"
            class:active={settingsTab === "appearance"}
            onclick={() => (settingsTab = "appearance")}
          >
            <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="8" cy="8" r="6"/>
              <path d="M8 2a6 6 0 0 1 0 12V2z"/>
            </svg>
            <span>Tampilan</span>
          </button>
          <button
            type="button"
            class="modal-tab-btn"
            class:active={settingsTab === "devices"}
            onclick={() => (settingsTab = "devices")}
          >
            <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M8 1a2.5 2.5 0 0 0-2.5 2.5v4a2.5 2.5 0 0 0 5 0v-4A2.5 2.5 0 0 0 8 1z"/>
              <path d="M3.5 6.5v1a4.5 4.5 0 0 0 9 0v-1M8 12v3M5.5 15h5"/>
            </svg>
            <span>Audio & Video</span>
          </button>
          <button
            type="button"
            class="modal-tab-btn"
            class:active={settingsTab === "notifications"}
            onclick={() => (settingsTab = "notifications")}
          >
            <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M4 6a4 4 0 0 1 8 0c0 4 2 5 2 5H2s2-1 2-5M7 14a1.5 1.5 0 0 0 2 0"/>
            </svg>
            <span>Notifikasi</span>
          </button>
          <button
            type="button"
            class="modal-tab-btn"
            class:active={settingsTab === "maintenance"}
            onclick={() => (settingsTab = "maintenance")}
          >
            <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
              <path d="M10 2l4 4-2 2-4-4 2-2zM8 4L2.5 9.5a1.5 1.5 0 0 0 0 2.1l1.9 1.9a1.5 1.5 0 0 0 2.1 0L12 8"/>
            </svg>
            <span>Pemeliharaan</span>
          </button>
        </nav>

        <div class="modal-body">
          {#if settingsTab === "appearance"}
            <div class="settings-panel">
              <div class="settings-card">
                <div class="card-header">
                  <h3 class="card-title">Tema Aplikasi</h3>
                  <p class="card-desc">Pilih tema warna antarmuka yang paling nyaman untuk Anda.</p>
                </div>
                <div class="theme-options">
                  <button
                    type="button"
                    class="theme-card"
                    class:selected={theme === "system"}
                    onclick={() => setTheme("system")}
                  >
                    <div class="theme-card-icon">
                      <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="2" y="3" width="20" height="14" rx="2" />
                        <line x1="8" y1="21" x2="16" y2="21" />
                        <line x1="12" y1="17" x2="12" y2="21" />
                      </svg>
                    </div>
                    <span class="theme-card-label">Ikuti Sistem</span>
                    <span class="theme-card-sub">Sinkron dengan OS Windows</span>
                  </button>

                  <button
                    type="button"
                    class="theme-card"
                    class:selected={theme === "light"}
                    onclick={() => setTheme("light")}
                  >
                    <div class="theme-card-icon">
                      <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                        <circle cx="12" cy="12" r="5" />
                        <line x1="12" y1="1" x2="12" y2="3" />
                        <line x1="12" y1="21" x2="12" y2="23" />
                        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                        <line x1="1" y1="12" x2="3" y2="12" />
                        <line x1="21" y1="12" x2="23" y2="12" />
                        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
                      </svg>
                    </div>
                    <span class="theme-card-label">Terang</span>
                    <span class="theme-card-sub">Cerah dan bersih</span>
                  </button>

                  <button
                    type="button"
                    class="theme-card"
                    class:selected={theme === "dark"}
                    onclick={() => setTheme("dark")}
                  >
                    <div class="theme-card-icon">
                      <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
                      </svg>
                    </div>
                    <span class="theme-card-label">Gelap</span>
                    <span class="theme-card-sub">Nyaman di malam hari</span>
                  </button>
                </div>
              </div>
            </div>

          {:else if settingsTab === "devices"}
            <div class="settings-panel">
              {#if !active}
                <div class="info-alert">
                  <p>Pilih profil terlebih dahulu untuk mengonfigurasi perangkat audio dan video.</p>
                </div>
              {:else}
                <div class="settings-card">
                  <div class="card-header">
                    <h3 class="card-title">Perangkat Panggilan WhatsApp</h3>
                    <p class="card-desc">Pilih mikrofon dan kamera yang digunakan saat melakukan panggilan.</p>
                  </div>

                  <div class="form-group">
                    <label for="select-mic" class="form-label">
                      <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M8 1a2.5 2.5 0 0 0-2.5 2.5v4a2.5 2.5 0 0 0 5 0v-4A2.5 2.5 0 0 0 8 1z"/>
                        <path d="M3.5 6.5v1a4.5 4.5 0 0 0 9 0v-1M8 12v3M5.5 15h5"/>
                      </svg>
                      <span>Mikrofon</span>
                    </label>
                    <div class="select-wrapper">
                      <select id="select-mic" bind:value={audioId} disabled={busy || devicesLoading || !devices}>
                        <option value="">Default sistem Windows</option>
                        {#if audioId && !devices?.audio.some((device) => device.id === audioId)}
                          <option value={audioId}>Perangkat tersimpan tidak tersedia</option>
                        {/if}
                        {#each devices?.audio ?? [] as device (device.id)}
                          <option value={device.id}>{device.label || "Mikrofon (" + device.id.slice(0, 8) + "…)"}</option>
                        {/each}
                      </select>
                    </div>
                  </div>

                  <div class="form-group">
                    <label for="select-cam" class="form-label">
                      <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M14.5 4.5L10 8l4.5 3.5v-7z"/>
                        <rect x="1.5" y="3.5" width="8.5" height="9" rx="1.5"/>
                      </svg>
                      <span>Kamera</span>
                    </label>
                    <div class="select-wrapper">
                      <select id="select-cam" bind:value={videoId} disabled={busy || devicesLoading || !devices}>
                        <option value="">Default sistem Windows</option>
                        {#if videoId && !devices?.video.some((device) => device.id === videoId)}
                          <option value={videoId}>Perangkat tersimpan tidak tersedia</option>
                        {/if}
                        {#each devices?.video ?? [] as device (device.id)}
                          <option value={device.id}>{device.label || "Kamera (" + device.id.slice(0, 8) + "…)"}</option>
                        {/each}
                      </select>
                    </div>
                  </div>

                  <div class="card-footer-actions">
                    <button
                      class="btn btn-primary"
                      type="button"
                      disabled={busy || devicesLoading || !devices}
                      onclick={saveDevices}
                    >
                      <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                        <polyline points="3.5 8.5 6.5 11.5 12.5 4.5" />
                      </svg>
                      <span>Simpan Perangkat</span>
                    </button>

                    <button
                      class="btn btn-secondary"
                      type="button"
                      disabled={busy || devicesLoading || !active}
                      onclick={() => loadDevices(model.activeId)}
                    >
                      <svg class="t-icon {devicesLoading ? 'spin' : ''}" viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                        <path d="M2.5 8a5.5 5.5 0 0 1 9.3-3.9L14 6.5M14 2v4.5H9.5M13.5 8a5.5 5.5 0 0 1-9.3 3.9L2 9.5M2 14v-4.5h4.5"/>
                      </svg>
                      <span>{devicesLoading ? "Mendeteksi…" : "Deteksi Ulang"}</span>
                    </button>
                  </div>
                </div>

                <div class="info-alert">
                  <svg viewBox="0 0 16 16" width="15" height="15" fill="currentColor">
                    <circle cx="8" cy="8" r="7" fill="none" stroke="currentColor" stroke-width="1.5"/>
                    <path d="M8 7v4M8 5h.01"/>
                  </svg>
                  <p>Kamera dapat menyala sesaat saat sistem mendeteksi perangkat keras yang tersedia. Jika perangkat yang tersimpan dicabut, sistem akan kembali memakai default.</p>
                </div>
              {/if}
            </div>

          {:else if settingsTab === "notifications"}
            <div class="settings-panel">
              <NotificationSettings />
            </div>

          {:else if settingsTab === "maintenance"}
            <div class="settings-panel">
              <div class="settings-card">
                <div class="card-header">
                  <h3 class="card-title">Bersihkan Cache Profil</h3>
                  <p class="card-desc">Menghapus file gambar, media unduhan sementara, dan cache skrip profil aktif. Sesi login WhatsApp Anda tetap aman dan tidak akan logout.</p>
                </div>
                <div class="card-action-row">
                  <button
                    class="btn btn-secondary"
                    type="button"
                    disabled={busy || !active}
                    onclick={clearCache}
                  >
                    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M3 4h10M6 4V2.5A1.5 1.5 0 0 1 7.5 1h1A1.5 1.5 0 0 1 10 2.5V4M12.5 4v9.5a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5V4"/>
                    </svg>
                    <span>{pending === "cache" ? "Membersihkan…" : "Bersihkan Cache"}</span>
                  </button>
                </div>
              </div>

              <div class="settings-card">
                <div class="card-header">
                  <h3 class="card-title">Alat Pengembang (Developer Tools)</h3>
                  <p class="card-desc">Buka DevTools untuk inspeksi elemen, konsol log, dan diagnosis masalah webview.</p>
                </div>
                <div class="card-action-row">
                  <button
                    class="btn btn-secondary"
                    type="button"
                    disabled={busy || !active}
                    onclick={() => run("devtools", () => Workspace.openDevTools())}
                  >
                    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                      <polyline points="5 4 1 8 5 12"/>
                      <polyline points="11 4 15 8 11 12"/>
                      <line x1="9.5" y1="3" x2="6.5" y2="13"/>
                    </svg>
                    <span>Buka DevTools</span>
                  </button>
                </div>
              </div>
            </div>
          {/if}
        </div>

        <div class="modal-footer">
          {#if settingsMessage}
            <div class="modal-status-msg success" role="status">
              <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor">
                <path d="M8 0a8 8 0 1 0 0 16A8 8 0 0 0 8 0zm3.4 5.9l-4.2 4.2a.8.8 0 0 1-1.1 0L4.6 8.6a.8.8 0 0 1 1.1-1.1l1 1 3.6-3.7a.8.8 0 0 1 1.1 1.1z"/>
              </svg>
              <span>{settingsMessage}</span>
            </div>
          {/if}
          <div class="modal-footer-btns">
            <button class="btn btn-primary" type="button" onclick={() => (settingsOpen = false)}>
              Selesai
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- FORM MODALS (Create, Rename, Delete) -->
  {#if form !== "none"}
    <div
      class="modal-backdrop"
      role="presentation"
      onclick={(e) => { if (e.target === e.currentTarget) void closeForm(); }}
    >
      <div class="modal modal-sm" role="dialog" aria-modal="true">
        {#if form === "create"}
          <div class="modal-header">
            <div class="modal-header-info">
              <div class="modal-icon-badge">
                <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M8 2v12M2 8h12"/>
                </svg>
              </div>
              <div>
                <h2 class="modal-title">Tambah Profil Baru</h2>
                <p class="modal-subtitle">Setiap profil memiliki sesi login dan folder data mandiri.</p>
              </div>
            </div>
            <button class="modal-close-btn" type="button" onclick={() => closeForm()} aria-label="Batal" title="Batal (Esc)">
              <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
                <path d="M3 3l10 10M13 3l-10 10" />
              </svg>
            </button>
          </div>
          <form onsubmit={submitForm}>
            <div class="modal-body">
              <div class="form-group">
                <label for="create-profile-name" class="form-label">Nama Profil</label>
                <input
                  id="create-profile-name"
                  class="modal-input"
                  bind:this={nameField}
                  bind:value={name}
                  maxlength="60"
                  autocomplete="off"
                  placeholder="Contoh: Kantor, Toko Online, Pribadi…"
                  required
                />
              </div>
            </div>
            <div class="modal-footer">
              <div class="modal-footer-btns">
                <button class="btn btn-secondary" type="button" onclick={() => closeForm()} disabled={busy}>Batal</button>
                <button class="btn btn-primary" type="submit" disabled={busy}>
                  {busy ? "Membuat…" : "Buat Profil"}
                </button>
              </div>
            </div>
          </form>

        {:else if form === "rename"}
          <div class="modal-header">
            <div class="modal-header-info">
              <div class="modal-icon-badge">
                <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M11.5 2.5a1.77 1.77 0 0 1 2.5 2.5L5 14l-3.5 1 1-3.5 9-9z"/>
                </svg>
              </div>
              <div>
                <h2 class="modal-title">Ganti Nama Profil</h2>
                <p class="modal-subtitle">Ubah nama profil yang ditampilkan di bilah tab.</p>
              </div>
            </div>
            <button class="modal-close-btn" type="button" onclick={() => closeForm()} aria-label="Batal" title="Batal (Esc)">
              <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
                <path d="M3 3l10 10M13 3l-10 10" />
              </svg>
            </button>
          </div>
          <form onsubmit={submitForm}>
            <div class="modal-body">
              <div class="form-group">
                <label for="rename-profile-name" class="form-label">Nama Baru</label>
                <input
                  id="rename-profile-name"
                  class="modal-input"
                  bind:this={nameField}
                  bind:value={name}
                  maxlength="60"
                  autocomplete="off"
                  required
                />
              </div>
            </div>
            <div class="modal-footer">
              <div class="modal-footer-btns">
                <button class="btn btn-secondary" type="button" onclick={() => closeForm()} disabled={busy}>Batal</button>
                <button class="btn btn-primary" type="submit" disabled={busy}>
                  {busy ? "Menyimpan…" : "Simpan Perubahan"}
                </button>
              </div>
            </div>
          </form>

        {:else if form === "delete"}
          <div class="modal-header">
            <div class="modal-header-info">
              <div class="modal-icon-badge danger">
                <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M3 4h10M6 4V2.5A1.5 1.5 0 0 1 7.5 1h1A1.5 1.5 0 0 1 10 2.5V4M12.5 4v9.5a1.5 1.5 0 0 1-1.5 1.5H5a1.5 1.5 0 0 1-1.5-1.5V4"/>
                </svg>
              </div>
              <div>
                <h2 class="modal-title">Hapus Profil?</h2>
                <p class="modal-subtitle">Tindakan ini permanen dan tidak dapat dibatalkan.</p>
              </div>
            </div>
            <button class="modal-close-btn" type="button" onclick={() => closeForm()} aria-label="Batal" title="Batal (Esc)">
              <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
                <path d="M3 3l10 10M13 3l-10 10" />
              </svg>
            </button>
          </div>
          <div class="modal-body">
            <p class="delete-warning-text">
              Apakah Anda yakin ingin menghapus profil <strong>“{targetTab?.name ?? ""}”</strong>? Seluruh sesi login WhatsApp Web dan folder data profil ini akan dihapus dari penyimpanan.
            </p>
          </div>
          <div class="modal-footer">
            <div class="modal-footer-btns">
              <button class="btn btn-secondary" type="button" onclick={() => closeForm()} disabled={busy}>Batal</button>
              <button class="btn btn-danger" type="button" onclick={confirmRemove} disabled={busy}>
                {busy ? "Menghapus…" : "Ya, Hapus Profil"}
              </button>
            </div>
          </div>
        {/if}
      </div>
    </div>
  {/if}

  <!-- Active Profile Viewport (Where native child window is placed) -->
  <div
    id="profile-view"
    class="view"
    role="tabpanel"
    aria-label={active ? active.name : "Tampilan profil"}
    bind:this={view}
  >
    {#if loading}
      <div class="viewport-placeholder">
        <span class="loading-spinner"></span>
        <p>Memuat profil WhatsApp…</p>
      </div>
    {:else if !model.tabs.length}
      <div class="empty">
        <div class="empty-icon-circle">
          <svg viewBox="0 0 24 24" width="32" height="32" fill="currentColor">
            <path d="M12.04 2c-5.46 0-9.91 4.45-9.91 9.91 0 1.75.46 3.45 1.32 4.95L2.05 22l5.25-1.38c1.45.79 3.08 1.21 4.74 1.21 5.46 0 9.91-4.45 9.91-9.91 0-2.65-1.03-5.14-2.9-7.01A9.82 9.82 0 0 0 12.04 2z"/>
          </svg>
        </div>
        <h2>Belum Ada Profil WhatsApp</h2>
        <p>Mulai dengan menambahkan profil WhatsApp pertama Anda. Setiap profil memiliki sesi akun terpisah.</p>
        <button class="btn btn-primary btn-lg" type="button" onclick={() => openForm("create")} disabled={busy}>
          <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <path d="M8 2v12M2 8h12"/>
          </svg>
          <span>Buat Profil Pertama</span>
        </button>
      </div>
    {:else if !active}
      <div class="viewport-placeholder">
        <p>Pilih profil di bilah tab atas untuk mulai menggunakan WhatsApp.</p>
      </div>
    {/if}
  </div>
</div>
