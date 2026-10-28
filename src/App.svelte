<script lang="ts">
  import { onMount, tick } from "svelte";
  import { currentWindow } from "mygo-runtime";
  import { Workspace, events, type State } from "./mygo";

  type Pending = "startup" | "create" | "rename" | "remove" | "activate" | "reload" | "notify";
  type Form = "none" | "create" | "rename" | "delete";

  let model = $state<State>({ tabs: [], activeId: "" });
  let loading = $state(true);
  let pending = $state<Pending | null>(null);
  let error = $state("");
  let form = $state<Form>("none");
  let name = $state("");
  let target = $state<string | null>(null);
  let maximized = $state(false);

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

  async function testNotification() {
    await run("notify", () => Workspace.testNotification());
  }

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
          aria-selected={tab.id === model.activeId}
          aria-controls="profile-view"
          tabindex={tab.id === focusId ? 0 : -1}
          onclick={() => activate(tab.id)}
          onkeydown={(e) => onTabKey(e, index)}
        >
          {tab.name}
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
        <button
          class="btn"
          type="button"
          disabled={busy || !active}
          title="Kirim notifikasi uji dengan suara. Klik notifikasi untuk membuka profil."
          onclick={testNotification}
        >
          {pending === "notify" ? "Mengirim…" : "Tes notifikasi"}
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
