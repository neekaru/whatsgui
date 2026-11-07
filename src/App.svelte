<script lang="ts">
  import { onMount, tick } from "svelte";
  import { currentWindow } from "mygo-runtime";
  import { Workspace, events, type State, type MediaSettings } from "./mygo";
  import NotificationSettings from "./NotificationSettings.svelte";

  type Pending = "startup" | "create" | "rename" | "remove" | "activate" | "reload" | "cache" | "devices" | "devtools";
  type Form = "none" | "create" | "rename" | "delete";

  let model = $state<State>({ tabs: [], activeId: "" });
  let loading = $state(true);
  let pending = $state<Pending | null>(null);
  let error = $state("");
  let form = $state<Form>("none");
  let name = $state("");
  let target = $state<string | null>(null);
  let maximized = $state(false);
  let settingsOpen = $state(false);
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
    if (ok) settingsMessage = "Perangkat disimpan untuk panggilan berikutnya.";
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
      settingsMessage = "Cache profil aktif dibersihkan.";
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

  // Window controls are the real OS ones: the page draws the captions.
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

  // The inline rename/create forms live in the toolbar, above the native
  // view, so the parent's worker window never covers them.
  $effect(() => {
    if (form === "create" || form === "rename") {
      nameField?.focus();
      nameField?.select();
    }
  });

  // The native child window sits exactly on the content rectangle: report it
  // in CSS pixels with the device pixel ratio whenever it moves or resizes,
  // and again on activation so the newly shown worker is positioned.
  $effect(() => {
    const el = view;
    const activeId = model.activeId;
    if (!el) return;

    let key = "";
    const report = () => {
      const rect = el.getBoundingClientRect();
      const width = Math.round(rect.width);
      const height = Math.round(rect.height);
      if (!width || !height) return;
      const x = Math.round(rect.left);
      const y = Math.round(rect.top);
      const scale = window.devicePixelRatio;
      const next = `${activeId}|${x}|${y}|${width}|${height}|${scale}`;
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

    // Moving the window to another monitor changes devicePixelRatio without
    // firing a resize, so watch the ratio's own media query.
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
      if (e.key === "Escape" && form !== "none") {
        e.preventDefault();
        void closeForm();
      } else if (e.key === "Escape" && settingsOpen) {
        e.preventDefault();
        settingsOpen = false;
      }
    };
    window.addEventListener("keydown", onEscape);

    // Maximizing, restoring and dragging between monitors all resize the
    // window, so the restore icon follows the real state.
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
          {tab.name}
          {#if tab.unread > 0}<span class="tab-count" aria-hidden="true">{tab.unread > 99 ? "99+" : tab.unread}</span>{/if}
        </button>
      {/each}
    </div>
    <button
      type="button"
      class="tab-add"
      bind:this={createButton}
      onclick={() => openForm("create")}
      disabled={busy}
      aria-label="Profil baru"
      title="Profil baru"
    >
      <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" focusable="false">
        <path d="M5 0.5v9M0.5 5h9" fill="none" stroke="currentColor" stroke-width="1" />
      </svg>
    </button>
    <div class="drag-space" aria-hidden="true"></div>
    <div class="captions">
      <button
        type="button"
        class="caption"
        onclick={() => windowAction("minimize")}
        aria-label="Minimalkan jendela"
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
      >
        <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true" focusable="false">
          <path d="M0.5 0.5l9 9M9.5 0.5l-9 9" fill="none" stroke="currentColor" stroke-width="1" />
        </svg>
      </button>
    </div>
  </header>

  <div class="toolbar" aria-busy={busy}>
    {#if form === "create"}
      <form class="inline" onsubmit={submitForm}>
        <label for="profile-name">Nama</label>
        <input
          id="profile-name"
          bind:this={nameField}
          bind:value={name}
          maxlength="60"
          autocomplete="off"
          placeholder="Profil baru"
          required
        />
        <button class="btn primary" type="submit" disabled={busy}>
          {busy ? "Menyimpan…" : "Buat"}
        </button>
        <button class="btn" type="button" onclick={() => closeForm()} disabled={busy}>Batal</button>
      </form>
    {:else if form === "rename"}
      <form class="inline" onsubmit={submitForm}>
        <label for="profile-name">Nama</label>
        <input
          id="profile-name"
          bind:this={nameField}
          bind:value={name}
          maxlength="60"
          autocomplete="off"
          required
        />
        <button class="btn primary" type="submit" disabled={busy}>
          {busy ? "Menyimpan…" : "Simpan"}
        </button>
        <button class="btn" type="button" onclick={() => closeForm()} disabled={busy}>Batal</button>
      </form>
    {:else if form === "delete"}
      <div class="inline">
        <p class="confirm-q" role="alert">
          Hapus “{targetTab?.name ?? ""}” beserta folder datanya?
        </p>
        <button class="btn danger" type="button" onclick={confirmRemove} disabled={busy}>
          {busy ? "Menghapus…" : "Hapus"}
        </button>
        <button class="btn" type="button" onclick={() => closeForm()} disabled={busy}>Batal</button>
      </div>
    {:else}
      <p class="current" title={active?.name}>{active?.name ?? "Tidak ada profil aktif"}</p>
      <div class="actions">
        <button
          class="btn"
          type="button"
          disabled={busy || !active}
          onclick={() => active && openForm("rename", active.id, active.name)}
        >
          Ganti nama
        </button>
        <button
          class="btn"
          type="button"
          disabled={busy || !active}
          onclick={() => active && openForm("delete", active.id)}
        >
          Hapus
        </button>
        <button class="btn" type="button" disabled={busy || !active} onclick={reloadProfile}>
          {pending === "reload" ? "Memuat…" : "Muat ulang"}
        </button>
        <button class="btn" type="button" disabled={busy} aria-expanded={settingsOpen}
          onclick={() => { settingsOpen = !settingsOpen; settingsMessage = ""; }}>
          Pengaturan
        </button>
      </div>
    {/if}
  </div>

  {#if error}
    <div class="alert" role="alert">
      <p>{error}</p>
      <button class="btn" type="button" onclick={() => (error = "")}>Tutup</button>
    </div>
  {/if}
  {#if settingsOpen}
    <section class="settings" aria-label="Pengaturan">
      <div class="settings-heading">
        <h2>Pengaturan</h2>
        <button class="btn" type="button" onclick={() => (settingsOpen = false)}>Tutup</button>
      </div>
      <div class="settings-grid">
        <label class="setting">
          <span>Tema aplikasi</span>
          <select value={theme} onchange={(e) => setTheme(e.currentTarget.value as "system" | "light" | "dark")}>
            <option value="system">Ikuti sistem</option>
            <option value="light">Terang</option>
            <option value="dark">Gelap</option>
          </select>
        </label>
        <label class="setting">
          <span>Mikrofon</span>
          <select bind:value={audioId} disabled={busy || devicesLoading || !devices}>
            <option value="">Default sistem</option>
            {#if audioId && !devices?.audio.some((device) => device.id === audioId)}
              <option value={audioId}>Perangkat tersimpan tidak tersedia</option>
            {/if}
            {#each devices?.audio ?? [] as device (device.id)}
              <option value={device.id}>{device.label || "Mikrofon"}</option>
            {/each}
          </select>
        </label>
        <label class="setting">
          <span>Kamera</span>
          <select bind:value={videoId} disabled={busy || devicesLoading || !devices}>
            <option value="">Default sistem</option>
            {#if videoId && !devices?.video.some((device) => device.id === videoId)}
              <option value={videoId}>Perangkat tersimpan tidak tersedia</option>
            {/if}
            {#each devices?.video ?? [] as device (device.id)}
              <option value={device.id}>{device.label || "Kamera"}</option>
            {/each}
          </select>
        </label>
        <div class="settings-actions">
          <button class="btn" type="button" disabled={busy || devicesLoading || !devices} onclick={saveDevices}>Simpan perangkat</button>
          <button class="btn" type="button" disabled={busy || devicesLoading || !active}
            onclick={() => loadDevices(model.activeId)}>Muat ulang perangkat</button>
          <button class="btn" type="button" disabled={busy || !active} onclick={clearCache}>Bersihkan cache</button>
          <button class="btn" type="button" disabled={busy || !active}
            onclick={() => run("devtools", () => Workspace.openDevTools())}>Buka DevTools</button>
        </div>
      </div>
      <p class="settings-note">
        {#if devicesLoading}Memuat perangkat…
        {:else if settingsMessage}{settingsMessage}
        {:else}Berlaku untuk profil aktif. Kamera dapat menyala sesaat saat memuat daftar perangkat.
          Pembersihan cache memuat ulang profil dan mengakhiri panggilan; sesi tetap disimpan.
          Jika perangkat tersimpan tidak tersedia, pilih perangkat lain atau default sistem.{/if}
      </p>
      <NotificationSettings />
    </section>
  {/if}

  <div
    id="profile-view"
    class="view"
    role="tabpanel"
    aria-label={active ? active.name : "Tampilan profil"}
    bind:this={view}
  >
    {#if loading}
      <p class="placeholder">Memuat profil…</p>
    {:else if !model.tabs.length}
      <div class="empty">
        <h2>Belum ada profil</h2>
        <p>Setiap profil berjalan di proses dan folder data sendiri.</p>
        <button class="btn primary" type="button" onclick={() => openForm("create")} disabled={busy}>
          Buat profil
        </button>
      </div>
    {:else if !active}
      <p class="placeholder">Pilih profil.</p>
    {/if}
  </div>
</div>
