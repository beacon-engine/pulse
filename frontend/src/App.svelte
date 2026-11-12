<script lang="ts">
  import { onMount } from "svelte";
  import ContainerDrawer from "./components/ContainerDrawer.svelte";
  import SwarmDrawer from "./components/SwarmDrawer.svelte";
  import EnvironmentSelector from "./components/EnvironmentSelector.svelte";
  import AddEnvironmentModal from "./components/AddEnvironmentModal.svelte";
  import AuthScreen from "./components/AuthScreen.svelte";
  import TopLoader from "./components/TopLoader.svelte";
  import ToastContainer from "./components/ToastContainer.svelte";
  import RuntimeUnavailable from "./components/RuntimeUnavailable.svelte";

  import SettingsPage from "./pages/SettingsPage.svelte";
  import HostsPage from "./pages/HostsPage.svelte";
  import ContainersPage from "./pages/ContainersPage.svelte";
  import StacksPage from "./pages/StacksPage.svelte";
  import EventsPage from "./pages/EventsPage.svelte";
  import KubernetesPage from "./pages/KubernetesPage.svelte";
  import MetricsPage from "./pages/MetricsPage.svelte";
  import ResourcesPage from "./pages/ResourcesPage.svelte";
  import SwarmPage from "./pages/SwarmPage.svelte";

  import { api, type Account } from "./lib/api";
  import { connect } from "./lib/websocket";
  import { currentEnv, currentRoute, initRouter, navigate, type Route } from "./lib/router";
  import { hostsStore } from "./stores/hosts.svelte";
  import { containersStore } from "./stores/container.svelte";
  import { swarmStore } from "./stores/swarm.svelte";
  import { loadingStore } from "./stores/loading.svelte";
  import { toast } from "./lib/toast.svelte";
  import { debounce } from "./lib/timing";
  import type { Container } from "./types/containers";
  import type { Environment, Requirement } from "./types/environments";
  import { PULSE_VERSION_LABEL } from "./lib/version";

  // ── Theme ─────────────────────────────────────────────────────────────────
  let theme = $state<"light" | "dark">("light");
  $effect(() => {
    document.documentElement.setAttribute("data-theme", theme);
  });
  function toggleTheme() {
    theme = theme === "light" ? "dark" : "light";
  }

  // ── Auth ──────────────────────────────────────────────────────────────────
  let account = $state<Account | null>(null);
  let authReady = $state(false);
  let wsDisconnect: (() => void) | null = null;

  // ── Navigation ────────────────────────────────────────────────────────────
  let route = $state<Route>("containers");
  currentRoute.subscribe((r) => {
    route = r;
    loadingStore.reset(); // Reset in-flight loader counter on route transitions
  });
  currentEnv.subscribe((id) => void hostsStore.syncFromUrl(id));

  type NavItem = { id: Route; label: string; icon: string; requires?: Requirement };

  const NAV: NavItem[] = [
    {
      id: "containers",
      requires: "containers",
      label: "Containers",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="7" width="18" height="13" rx="2"/><path d="M3 11h18M8 7V4.5A1.5 1.5 0 0 1 9.5 3h5A1.5 1.5 0 0 1 16 4.5V7"/></svg>`,
    },
    {
      id: "stacks",
      requires: "containers",
      label: "Stacks",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="3" width="7" height="7" rx="1.4"/><rect x="14" y="3" width="7" height="7" rx="1.4"/><rect x="3" y="14" width="7" height="7" rx="1.4"/><rect x="14" y="14" width="7" height="7" rx="1.4"/></svg>`,
    },
    {
      id: "swarm",
      requires: "swarm",
      label: "Swarm",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="5" r="2"/><circle cx="19" cy="9" r="2"/><circle cx="19" cy="15" r="2"/><circle cx="12" cy="19" r="2"/><circle cx="5" cy="15" r="2"/><circle cx="5" cy="9" r="2"/><path d="M7 9l4-2.5M13 9l4 2.5M17 15l-4 2.5M11 15l-4-2.5"/></svg>`,
    },
    {
      id: "events",
      label: "Events",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>`,
    },
    {
      id: "metrics",
      label: "Metrics",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>`,
    },
    {
      id: "resources",
      requires: "containers",
      label: "Resources",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`,
    },
    {
      id: "kubernetes",
      requires: "kubernetes",
      label: "Kubernetes",
      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z"/><path d="M12 22V12M20 6.5L12 12M4 6.5L12 12"/></svg>`,
    },
  ];

  // Admin-only. Users moved inside Settings; #/users still opens that section.
  const SETTINGS_NAV: NavItem = {
    id: "settings",
    label: "Settings",
    icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>`,
  };

  // Only offer pages the active environment's runtimes can back: a
  // Kubernetes-only agent has no Containers page, a Podman host has no Swarm.
  let navItems = $derived(
    (account?.role === "admin" ? [...NAV, SETTINGS_NAV] : NAV).filter(
      (n) => !n.requires || hostsStore.supports(n.requires)
    )
  );

  let routeRequirement = $derived(NAV.find((n) => n.id === route)?.requires);

  // Switching to an environment that can't back the current page moves to the
  // first page it can (e.g. Containers → Kubernetes for a cluster agent). An
  // unreachable agent stays put so the explanation panel is shown instead.
  $effect(() => {
    const caps = hostsStore.activeCapabilities;
    if (!caps?.reachable || !routeRequirement) return;
    if (hostsStore.supports(routeRequirement)) return;
    const next = NAV.find((n) => n.requires && hostsStore.supports(n.requires));
    navigate(next?.id ?? "events");
  });

  // ── Modals ────────────────────────────────────────────────────────────────
  let showAddHost = $state(false);

  // ── Command palette ───────────────────────────────────────────────────────
  let cmdkOpen = $state(false);
  let cmdkQuery = $state("");
  let cmdkSel = $state(0);
  let cmdkResults = $derived(
    containersStore.containers
      .filter(
        (c) =>
          !cmdkQuery.trim() ||
          c.name.toLowerCase().includes(cmdkQuery.toLowerCase()) ||
          c.image.toLowerCase().includes(cmdkQuery.toLowerCase())
      )
      .slice(0, 8)
  );

  function openCmdk() {
    cmdkOpen = true;
    cmdkQuery = "";
    cmdkSel = 0;
  }
  function closeCmdk() {
    cmdkOpen = false;
  }
  function cmdkSelect(c: Container) {
    containersStore.selectContainer(c);
    navigate("containers");
    closeCmdk();
  }

  // ── Host Actions ──────────────────────────────────────────────────────────
  async function handleSwitchHost(host: Environment) {
    hostsStore.switchHost(host);
  }

  async function handleRenameHost(id: string, name: string) {
    await api.renameHost(id, name);
    await hostsStore.loadHosts();
    toast.success(`Host renamed to "${name}"`);
  }

  async function handleDeleteHost(id: string) {
    await api.deleteHost(id);
    await hostsStore.loadHosts();
    toast.info("Host removed");
  }

  // ── WebSocket Handler ──────────────────────────────────────────────────────
  const handleSocketEvent = debounce(() => {
    // Only trigger live container refreshes when viewing the containers screen
    if (route === "containers") {
      containersStore.refresh();
    }
  }, 1000);

  // ── Boot / teardown ────────────────────────────────────────────────────────
  async function boot() {
    await hostsStore.loadHosts();
    // wsDisconnect = connect(handleSocketEvent);
  }

  function teardown() {
    if (wsDisconnect) {
      wsDisconnect();
      wsDisconnect = null;
    }
  }

  async function onAuthed(u: Account) {
    account = u;
    authReady = true;
    await boot();
  }

  async function logout() {
    try {
      await api.logout();
    } catch {
      /* ignore */
    }
    teardown();
    account = null;
    if (route === "users" || route === "settings") navigate("containers");
  }

  onMount(() => {
    const stopRouter = initRouter();

    function onkey(e: KeyboardEvent) {
      if (!account) return;
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        openCmdk();
        return;
      }
      if (e.key === "Escape") {
        closeCmdk();
        return;
      }
      if (cmdkOpen) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          cmdkSel = Math.min(cmdkSel + 1, cmdkResults.length - 1);
        }
        if (e.key === "ArrowUp") {
          e.preventDefault();
          cmdkSel = Math.max(cmdkSel - 1, 0);
        }
        if (e.key === "Enter" && cmdkResults[cmdkSel]) {
          cmdkSelect(cmdkResults[cmdkSel]);
        }
      }
    }
    window.addEventListener("keydown", onkey);

    const onUnauth = () => {
      teardown();
      account = null;
    };
    window.addEventListener("pulse:unauthorized", onUnauth);

    (async () => {
      try {
        account = await api.me();
      } catch {
        account = null;
      }
      authReady = true;
      if (account) await boot();
    })();

    return () => {
      stopRouter();
      teardown();
      window.removeEventListener("keydown", onkey);
      window.removeEventListener("pulse:unauthorized", onUnauth);
    };
  });
</script>

<TopLoader />
<ToastContainer />

{#if !authReady}
  <div class="app-boot">Loading…</div>
{:else if !account}
  <AuthScreen {onAuthed} />
{:else}
  <!-- ── Container Drawer ────────────────────────────────────────────────── -->
  {#if containersStore.selectedContainer && hostsStore.activeHostId}
    <ContainerDrawer
      hostId={hostsStore.activeHostId}
      container={containersStore.selectedContainer}
      onclose={() => containersStore.selectContainer(null)}
      onrefresh={() => containersStore.refresh()}
    />
  {/if}

  <!-- ── Swarm Drawer ────────────────────────────────────────────────────── -->
  {#if swarmStore.selectedService && hostsStore.activeHostId}
    <SwarmDrawer
      hostId={hostsStore.activeHostId}
      swarm={swarmStore.selectedService}
      onclose={() => swarmStore.selectService(null)}
      onrefresh={() => swarmStore.refresh()}
    />
  {/if}

  <!-- ── Add host modal ─────────────────────────────────────────────────── -->
  {#if showAddHost}
    <AddEnvironmentModal
      onclose={() => (showAddHost = false)}
      oncreated={async () => {
        showAddHost = false;
        await hostsStore.loadHosts();
      }}
    />
  {/if}

  <!-- ── Command palette ────────────────────────────────────────────────── -->
  {#if cmdkOpen}
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div
      class="cmdk-overlay"
      onclick={(e) => {
        if (e.target === e.currentTarget) closeCmdk();
      }}
    >
      <div class="cmdk-box">
        <div class="cmdk-input-wrap">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="7" /><path d="M21 21l-4.3-4.3" />
          </svg>
          <!-- svelte-ignore a11y_autofocus -->
          <input
            class="cmdk-input"
            type="text"
            placeholder="Search containers…"
            bind:value={cmdkQuery}
            autofocus
            autocomplete="off"
          />
          <kbd>esc</kbd>
        </div>
        <div class="cmdk-list">
          {#if cmdkResults.length === 0}
            <div class="cmdk-empty">No containers match your search</div>
          {:else}
            {#each cmdkResults as c, i (c.id)}
              <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
              <div
                class="cmdk-row"
                class:sel={i === cmdkSel}
                onclick={() => cmdkSelect(c)}
              >
                <div class="cmdk-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                    <rect x="3" y="7" width="18" height="13" rx="2" />
                    <path d="M3 11h18M8 7V4.5A1.5 1.5 0 0 1 9.5 3h5A1.5 1.5 0 0 1 16 4.5V7" />
                  </svg>
                </div>
                <div class="cmdk-nm">{c.name}</div>
                <div class="cmdk-ty">{c.state}</div>
              </div>
            {/each}
          {/if}
        </div>
      </div>
    </div>
  {/if}

  <!-- ── App shell ──────────────────────────────────────────────────────── -->
  <div class="app">
    <!-- ── Sidebar ─────────────────────────────────────────────────────── -->
    <nav class="sidebar">
      <div class="brand">
        <div class="brand-mark">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none">
            <path
              d="M3 16L12 20L21 16M3 12L12 16L21 12M12 3L21 8L12 13L3 8L12 3Z"
              stroke="white"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
        </div>
        <div>
          <div class="brand-name">Pulse</div>
          <div class="brand-sub">Fleet Console</div>
        </div>
      </div>

      <div class="nav-section-label">Monitor</div>
      {#each navItems as item}
        <button
          class="nav-item"
          class:active={route === item.id || (item.id === "settings" && route === "users")}
          onclick={() => navigate(item.id)}
        >
          {@html item.icon}
          {item.label}
          {#if item.id === "containers" && containersStore.runningCount > 0}
            <span class="nav-badge">{containersStore.runningCount}</span>
          {/if}
        </button>
      {/each}

      <EnvironmentSelector
        hosts={hostsStore.hosts}
        total={hostsStore.totalHosts}
        activeId={hostsStore.activeHostId}
        onselect={handleSwitchHost}
        onadd={() => (showAddHost = true)}
        ondelete={handleDeleteHost}
        onrename={handleRenameHost}
        onseeall={() => navigate("hosts")}
      />

      <div class="sidebar-footer">
        {#if account}
          <div class="user-chip">
            <span class="user-avatar">
              {account.username.slice(0, 1).toUpperCase()}
            </span>
            <div class="user-meta">
              <div class="user-name">{account.username}</div>
              <div class="user-role">{account.role}</div>
            </div>
            <button
              class="logout-btn"
              title="Sign out"
              onclick={logout}
              aria-label="Sign out"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
                <path d="M16 17l5-5-5-5M21 12H9" />
              </svg>
            </button>
          </div>
        {/if}
        <div class="version">Pulse {PULSE_VERSION_LABEL}</div>
      </div>
    </nav>

    <!-- ── Main ───────────────────────────────────────────────────────── -->
    <div class="main">
      <!-- Topbar -->
      <div class="topbar">
        <div class="topbar-left">
          <h1>
            {[...NAV, SETTINGS_NAV].find((n) => n.id === (route === "users" ? "settings" : route))?.label ??
              (route === "hosts" ? "Hosts" : "")}
          </h1>
          <div class="topbar-sub">
            {hostsStore.activeHost?.name ?? ""}
          </div>
        </div>
        <button class="search-trigger" onclick={openCmdk}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="7" /><path d="M21 21l-4.3-4.3" />
          </svg>
          Search containers…
          <kbd>⌘K</kbd>
        </button>
        <button
          class="icon-btn"
          onclick={toggleTheme}
          title="Toggle theme"
        >
          {#if theme === "light"}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
            </svg>
          {:else}
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
              <circle cx="12" cy="12" r="4.5" />
              <path d="M12 2v2.5M12 19.5V22M4.2 4.2l1.8 1.8M18 18l1.8 1.8M2 12h2.5M19.5 12H22M4.2 19.8l1.8-1.8M18 6l1.8-1.8" />
            </svg>
          {/if}
        </button>
        <button class="icon-btn" onclick={() => containersStore.refresh()} title="Refresh">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" />
            <path d="M3 3v5h5" />
          </svg>
        </button>
      </div>

      <!-- Content Views -->
      <div class="content">
        {#if routeRequirement && !hostsStore.supports(routeRequirement)}
          <RuntimeUnavailable requirement={routeRequirement} />
        {:else if route === "containers"}
          <ContainersPage />
        {:else if route === "stacks"}
          <StacksPage />
        {:else if route === "events"}
          <EventsPage />
        {:else if route === "metrics"}
          <MetricsPage />
        {:else if route === "resources"}
          <ResourcesPage />
        {:else if route === "swarm"}
          <SwarmPage />
        {:else if route === "kubernetes"}
          <KubernetesPage />
        {:else if (route === "users" || route === "settings") && account?.role === "admin"}
          {#key route}
            <SettingsPage me={account} initial={route === "users" ? "users" : undefined} />
          {/key}
        {:else if route === "hosts"}
          <HostsPage />
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
    /* ── CSS Design Tokens ────────────────────────────────────────────────── */
    :global(:root) {
        --canvas: #f5f6fa;
        --surface: #ffffff;
        --surface-2: #fafbfd;
        --border: #e7e9f0;
        --border-strong: #d7dae3;
        --ink: #1a1d29;
        --mist: #6b7180;
        --mist-2: #9ca1af;
        --accent: #4a5fe8;
        --accent-ink: #3849c4;
        --accent-tint: #edf0fe;
        --green: #1c9a63;
        --green-tint: #e4f6ed;
        --amber: #b07a0b;
        --amber-tint: #fcf1d8;
        --red: #d1424b;
        --red-tint: #fce9ea;
        --violet: #6d56d6;
        --violet-tint: #eeeafb;
        --shadow-sm:
            0 1px 2px rgba(20, 22, 50, 0.04), 0 1px 1px rgba(20, 22, 50, 0.04);
        --shadow-md:
            0 6px 20px rgba(20, 22, 50, 0.08), 0 2px 6px rgba(20, 22, 50, 0.04);
        --shadow-lg:
            0 24px 56px rgba(20, 22, 50, 0.18),
            0 8px 20px rgba(20, 22, 50, 0.08);
        --radius-sm: 9px;
        --radius-md: 13px;
        --radius-lg: 20px;
        --sidebar-w: 248px;
    }
    :global([data-theme="dark"]) {
        --canvas: #111319;
        --surface: #1a1d26;
        --surface-2: #1f222c;
        --border: #2a2d38;
        --border-strong: #383b48;
        --ink: #ebecf2;
        --mist: #9ca1b3;
        --mist-2: #6e7385;
        --accent-tint: #1e2240;
        --green-tint: #0f2c1f;
        --amber-tint: #332a0e;
        --red-tint: #371d20;
        --violet-tint: #23203d;
        --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.4);
        --shadow-md: 0 6px 20px rgba(0, 0, 0, 0.45);
        --shadow-lg: 0 24px 56px rgba(0, 0, 0, 0.6);
    }
    :global(*, *::before, *::after) {
        box-sizing: border-box;
    }
    :global(html, body) {
        margin: 0;
        padding: 0;
    }
    :global(body) {
        font-family:
            "Inter",
            -apple-system,
            BlinkMacSystemFont,
            sans-serif;
        background: var(--canvas);
        color: var(--ink);
        font-size: 14px;
        line-height: 1.5;
        -webkit-font-smoothing: antialiased;
        transition:
            background 0.25s ease,
            color 0.25s ease;
    }
    :global(button) {
        font-family: inherit;
        cursor: pointer;
    }
    :global(::-webkit-scrollbar) {
        width: 8px;
        height: 8px;
    }
    :global(::-webkit-scrollbar-thumb) {
        background: var(--border-strong);
        border-radius: 8px;
    }
    :global(::-webkit-scrollbar-track) {
        background: transparent;
    }

    /* ── Pill ─────────────────────────────────────────────────────────────── */
    :global(.pill) {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 3px 9px 3px 7px;
        border-radius: 20px;
        font-size: 12px;
        font-weight: 650;
        white-space: nowrap;
    }
    :global(.pill .dot) {
        width: 6px;
        height: 6px;
        border-radius: 50%;
    }
    :global(.pill-green) {
        background: var(--green-tint);
        color: var(--green);
    }
    :global(.pill-green .dot) {
        background: var(--green);
        animation: pulse-green 2s infinite;
    }
    :global(.pill-red) {
        background: var(--red-tint);
        color: var(--red);
    }
    :global(.pill-amber) {
        background: var(--amber-tint);
        color: var(--amber);
    }
    :global(.pill-violet) {
        background: var(--violet-tint);
        color: var(--violet);
    }
    :global(.pill-mist) {
        background: var(--surface-2);
        color: var(--mist);
    }
    @keyframes pulse-green {
        0% {
            box-shadow: 0 0 0 0 rgba(28, 154, 99, 0.45);
        }
        70% {
            box-shadow: 0 0 0 5px rgba(28, 154, 99, 0);
        }
        100% {
            box-shadow: 0 0 0 0 rgba(28, 154, 99, 0);
        }
    }

    /* ── Layout ───────────────────────────────────────────────────────────── */
    .app {
        display: grid;
        grid-template-columns: var(--sidebar-w) 1fr;
        min-height: 100vh;
    }

    /* ── Sidebar ──────────────────────────────────────────────────────────── */
    .sidebar {
        background: var(--surface);
        border-right: 1px solid var(--border);
        padding: 20px 14px;
        display: flex;
        flex-direction: column;
        gap: 2px;
        position: sticky;
        top: 0;
        height: 100vh;
        overflow-y: auto;
    }
    .brand {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 6px 10px 22px;
    }
    .brand-mark {
        width: 30px;
        height: 30px;
        border-radius: 9px;
        background: linear-gradient(155deg, var(--accent), #7b8cf0);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        box-shadow: var(--shadow-sm);
    }
    .brand-name {
        font-family: "Manrope", sans-serif;
        font-weight: 800;
        font-size: 16.5px;
        letter-spacing: -0.02em;
        color: var(--ink);
    }
    .brand-sub {
        font-size: 10.5px;
        color: var(--mist-2);
        font-weight: 600;
        letter-spacing: 0.06em;
        text-transform: uppercase;
        margin-top: 1px;
    }

    .nav-section-label {
        font-size: 10.5px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: var(--mist-2);
        padding: 14px 12px 6px;
    }
    .nav-item {
        display: flex;
        align-items: center;
        gap: 11px;
        padding: 9px 12px;
        border-radius: var(--radius-sm);
        color: var(--mist);
        font-weight: 600;
        font-size: 13.5px;
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        transition:
            background 0.15s,
            color 0.15s;
    }
    .nav-item :global(svg) {
        width: 17px;
        height: 17px;
        flex-shrink: 0;
    }
    .nav-item:hover {
        background: var(--surface-2);
        color: var(--ink);
    }
    .nav-item.active {
        background: var(--accent-tint);
        color: var(--accent-ink);
    }
    .nav-badge {
        margin-left: auto;
        background: var(--red);
        color: #fff;
        font-size: 10.5px;
        font-weight: 700;
        border-radius: 20px;
        padding: 1px 6px;
    }
    .nav-item.active .nav-badge {
        background: var(--accent);
    }

    .sidebar-footer {
        margin-top: auto;
        border-top: 1px solid var(--border);
        padding-top: 12px;
    }
    .version {
        font-size: 11px;
        color: var(--mist-2);
        padding: 6px 12px;
    }

    .user-chip {
        display: flex;
        align-items: center;
        gap: 9px;
        padding: 6px 10px 10px;
    }
    .user-avatar {
        width: 30px;
        height: 30px;
        border-radius: 50%;
        flex-shrink: 0;
        background: var(--accent-tint);
        color: var(--accent-ink);
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: 700;
        font-size: 12.5px;
    }
    .user-meta {
        min-width: 0;
        flex: 1;
    }
    .user-name {
        font-weight: 650;
        font-size: 13px;
        color: var(--ink);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .user-role {
        font-size: 10.5px;
        color: var(--mist-2);
        text-transform: capitalize;
    }
    .logout-btn {
        width: 30px;
        height: 30px;
        border-radius: 8px;
        border: 1px solid var(--border);
        background: var(--surface);
        color: var(--mist);
        flex-shrink: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        transition:
            background 0.15s,
            color 0.15s;
    }
    .logout-btn:hover {
        background: var(--surface-2);
        color: var(--red);
    }
    .logout-btn :global(svg) {
        width: 15px;
        height: 15px;
    }

    .app-boot {
        position: fixed;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        background: var(--canvas);
        color: var(--mist);
        font-size: 14px;
    }

    /* ── Main ────────────────────────────────────────────────────────────── */
    .main {
        min-width: 0;
        display: flex;
        flex-direction: column;
    }

    .topbar {
        position: sticky;
        top: 0;
        z-index: 30;
        background: var(--canvas);
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 16px 28px;
        border-bottom: 1px solid var(--border);
        backdrop-filter: blur(6px);
    }
    .topbar-left {
        flex: 1;
        min-width: 0;
    }
    .topbar-left h1 {
        font-family: "Manrope", sans-serif;
        font-size: 19px;
        font-weight: 800;
        letter-spacing: -0.01em;
        margin: 0;
        color: var(--ink);
    }
    .topbar-sub {
        color: var(--mist);
        font-size: 12.5px;
        margin-top: 1px;
    }

    .search-trigger {
        display: flex;
        align-items: center;
        gap: 10px;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: 10px;
        padding: 8px 12px;
        color: var(--mist);
        font-size: 13px;
        min-width: 220px;
        transition:
            border-color 0.15s,
            color 0.15s;
    }
    .search-trigger:hover {
        border-color: var(--border-strong);
        color: var(--ink);
    }
    .search-trigger :global(svg) {
        width: 14px;
        height: 14px;
        flex-shrink: 0;
    }
    .search-trigger kbd {
        margin-left: auto;
        background: var(--surface-2);
        border: 1px solid var(--border);
        border-radius: 5px;
        padding: 1px 5px;
        font-size: 11px;
        font-family: "JetBrains Mono", monospace;
        color: var(--mist-2);
    }
    .icon-btn {
        width: 36px;
        height: 36px;
        border-radius: 10px;
        border: 1px solid var(--border);
        background: var(--surface);
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--mist);
        transition:
            background 0.15s,
            color 0.15s;
        flex-shrink: 0;
    }
    .icon-btn:hover {
        background: var(--surface-2);
        color: var(--ink);
    }
    .icon-btn :global(svg) {
        width: 17px;
        height: 17px;
    }

    .content {
        padding: 24px 28px 60px;
    }

    /* ── Command palette ─────────────────────────────────────────────────── */
    .cmdk-overlay {
        position: fixed;
        inset: 0;
        background: rgba(20, 22, 40, 0.4);
        z-index: 200;
        display: flex;
        align-items: flex-start;
        justify-content: center;
        padding-top: 13vh;
        backdrop-filter: blur(2px);
    }
    .cmdk-box {
        width: 560px;
        max-width: 90vw;
        background: var(--surface);
        border-radius: var(--radius-lg);
        box-shadow: var(--shadow-lg);
        border: 1px solid var(--border);
        overflow: hidden;
    }
    .cmdk-input-wrap {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 14px 18px;
        border-bottom: 1px solid var(--border);
    }
    .cmdk-input-wrap :global(svg) {
        width: 16px;
        height: 16px;
        color: var(--mist-2);
        flex-shrink: 0;
    }
    .cmdk-input-wrap kbd {
        font-family: "JetBrains Mono", monospace;
        font-size: 11px;
        color: var(--mist-2);
        background: var(--surface-2);
        border: 1px solid var(--border);
        border-radius: 5px;
        padding: 2px 6px;
    }
    .cmdk-input {
        flex: 1;
        border: none;
        background: none;
        font-size: 14.5px;
        color: var(--ink);
        outline: none;
        font-family: "Inter", sans-serif;
    }
    .cmdk-list {
        max-height: 340px;
        overflow-y: auto;
        padding: 8px;
    }
    .cmdk-row {
        display: flex;
        align-items: center;
        gap: 11px;
        padding: 9px 10px;
        border-radius: 9px;
        cursor: pointer;
        transition: background 0.1s;
    }
    .cmdk-row:hover,
    .cmdk-row.sel {
        background: var(--accent-tint);
    }
    .cmdk-icon {
        width: 30px;
        height: 30px;
        border-radius: 8px;
        background: var(--surface-2);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
    }
    .cmdk-icon :global(svg) {
        width: 14px;
        height: 14px;
        color: var(--mist);
    }
    .cmdk-nm {
        font-weight: 650;
        font-size: 13px;
        flex: 1;
    }
    .cmdk-ty {
        font-size: 11.5px;
        color: var(--mist-2);
        font-family: "JetBrains Mono", monospace;
    }
    .cmdk-empty {
        padding: 30px;
        text-align: center;
        color: var(--mist-2);
        font-size: 13px;
    }
</style>
