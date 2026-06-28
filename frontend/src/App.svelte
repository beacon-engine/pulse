<script lang="ts">
  import { onMount } from 'svelte';
  import ContainerDrawer from './components/ContainerDrawer.svelte';
  import EnvironmentSelector from './components/EnvironmentSelector.svelte';
  import AddEnvironmentModal from './components/AddEnvironmentModal.svelte';
  import { api } from './lib/api';
  import { setAppState } from './lib/appContext';
  import { connect } from './lib/websocket';
  import { currentRoute, initRouter, navigate, type Route } from './lib/router';
  import ContainersPage from './pages/ContainersPage.svelte';
  import StacksPage from './pages/StacksPage.svelte';
  import EventsPage from './pages/EventsPage.svelte';
  import KubernetesPage from './pages/KubernetesPage.svelte';
  import SwarmPage from './pages/SwarmPage.svelte';
  import type { Container, ContainerEvent, K8sNamespace, K8sPod, SavedStack, Stack, SwarmNode, SwarmService } from './types/containers';
  import type { Environment } from './types/environments';
  import type { AppState } from './types/appState';

  // ── Theme ─────────────────────────────────────────────────────────────────
  let theme = $state<'light' | 'dark'>('light');
  $effect(() => { document.documentElement.setAttribute('data-theme', theme); });
  function toggleTheme() { theme = theme === 'light' ? 'dark' : 'light'; }

  // ── Navigation ────────────────────────────────────────────────────────────
  let route = $state<Route>('containers');
  currentRoute.subscribe(r => route = r);

  const NAV: { id: Route; label: string; icon: string }[] = [
    { id: 'containers', label: 'Containers', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="7" width="18" height="13" rx="2"/><path d="M3 11h18M8 7V4.5A1.5 1.5 0 0 1 9.5 3h5A1.5 1.5 0 0 1 16 4.5V7"/></svg>` },
    { id: 'stacks',     label: 'Stacks',     icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="3" width="7" height="7" rx="1.4"/><rect x="14" y="3" width="7" height="7" rx="1.4"/><rect x="3" y="14" width="7" height="7" rx="1.4"/><rect x="14" y="14" width="7" height="7" rx="1.4"/></svg>` },
    { id: 'swarm',      label: 'Swarm',      icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="5" r="2"/><circle cx="19" cy="9" r="2"/><circle cx="19" cy="15" r="2"/><circle cx="12" cy="19" r="2"/><circle cx="5" cy="15" r="2"/><circle cx="5" cy="9" r="2"/><path d="M7 9l4-2.5M13 9l4 2.5M17 15l-4 2.5M11 15l-4-2.5"/></svg>` },
    { id: 'events',     label: 'Events',     icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/></svg>` },
    { id: 'kubernetes', label: 'Kubernetes', icon: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z"/><path d="M12 22V12M20 6.5L12 12M4 6.5L12 12"/></svg>` },
  ];

  // ── Hosts ─────────────────────────────────────────────────────────────────
  let hosts      = $state<Environment[]>([]);
  let activeHostId = $state<string | null>(null);
  let showAddHost  = $state(false);
  let activeHost   = $derived(hosts.find(h => h.id === activeHostId) ?? null);

  // ── Data ──────────────────────────────────────────────────────────────────
  let containers = $state<Container[]>([]);
  let stacks     = $state<Stack[]>([]);
  let savedStacks = $state<SavedStack[]>([]);
  let events     = $state<ContainerEvent[]>([]);
  let pods       = $state<K8sPod[]>([]);
  let namespaces = $state<K8sNamespace[]>([]);
  let swarmServices = $state<SwarmService[]>([]);
  let swarmNodes = $state<SwarmNode[]>([]);
  let loading       = $state(true);
  let k8sLoading    = $state(false);
  let swarmLoading  = $state(false);
  let error         = $state('');
  let k8sError      = $state('');
  let swarmError    = $state('');
  let lastUpdated   = $state('');
  let selectedContainer = $state<Container | null>(null);

  let running = $derived(containers.filter(c => c.state === 'running').length);

  // ── Command palette ───────────────────────────────────────────────────────
  let cmdkOpen    = $state(false);
  let cmdkQuery   = $state('');
  let cmdkSel     = $state(0);
  let cmdkResults = $derived(
    containers.filter(c =>
      !cmdkQuery.trim() ||
      c.name.toLowerCase().includes(cmdkQuery.toLowerCase()) ||
      c.image.toLowerCase().includes(cmdkQuery.toLowerCase())
    ).slice(0, 8)
  );

  function openCmdk()   { cmdkOpen = true; cmdkQuery = ''; cmdkSel = 0; }
  function closeCmdk()  { cmdkOpen = false; }
  function cmdkSelect(c: Container) { selectedContainer = c; navigate('containers'); closeCmdk(); }

  // ── Toasts ────────────────────────────────────────────────────────────────
  type Toast = { id: number; msg: string; kind: 'ok' | 'warn' | 'err' };
  let toasts = $state<Toast[]>([]);
  let _tid   = 0;
  function toast(msg: string, kind: Toast['kind'] = 'ok') {
    const id = ++_tid;
    toasts = [...toasts, { id, msg, kind }];
    setTimeout(() => { toasts = toasts.filter(t => t.id !== id); }, 3500);
  }

  // ── Fetching ──────────────────────────────────────────────────────────────
  async function refresh() {
    const id = activeHostId;
    if (id === null) { loading = false; return; }
    try {
      const [c, d, s, e] = await Promise.all([
        api.getContainers(id),
        api.getDetectedStacks(id),
        api.getSavedStacks(id),
        api.getEvents(),
      ]);
      containers = c; stacks = d; savedStacks = s; events = e; error = '';
      lastUpdated = new Date().toLocaleTimeString();
    } catch (err) {
      error = String(err);
    } finally {
      loading = false;
    }
  }

  async function refreshK8s() {
    k8sLoading = true;
    k8sError = '';
    try {
      const [p, n] = await Promise.all([api.getK8sPods(), api.getK8sNamespaces()]);
      pods = p; namespaces = n;
    } catch (err) {
      k8sError = String(err);
    } finally {
      k8sLoading = false;
    }
  }

  async function refreshSwarm() {
    const id = activeHostId;
    if (id === null) { swarmLoading = false; return; }
    swarmLoading = true;
    swarmError = '';
    try {
      const [s, n] = await Promise.all([api.getSwarmServices(id), api.getSwarmNodes(id)]);
      swarmServices = s; swarmNodes = n;
    } catch (err) {
      swarmError = String(err);
    } finally {
      swarmLoading = false;
    }
  }

  async function switchHost(id: string) {
    if (id === activeHostId) return;
    activeHostId = id; loading = true;
    containers = []; stacks = []; selectedContainer = null;
    await refresh();
  }

  async function loadHosts() {
    try {
      hosts = await api.listHosts();
      if (activeHostId === null && hosts.length > 0)
        activeHostId = (hosts.find(h => h.is_local) ?? hosts[0]).id;
    } catch (e) { error = String(e); }
  }

  async function renameHost(id: string, name: string) {
    await api.renameHost(id, name);
    await loadHosts();
    toast(`Host renamed to "${name}"`);
  }

  async function deleteHost(id: string) {
    await api.deleteHost(id);
    if (activeHostId === id) { activeHostId = null; containers = []; stacks = []; }
    await loadHosts();
    if (activeHostId !== null) await refresh();
    toast('Host removed', 'warn');
  }

  // Container actions
  async function startContainer(id: string)   { await api.startContainer(activeHostId!, id);   toast('Container started');            await refresh(); }
  async function stopContainer(id: string)    { await api.stopContainer(activeHostId!, id);    toast('Container stopped', 'warn');    await refresh(); }
  async function restartContainer(id: string) { await api.restartContainer(activeHostId!, id); toast('Container restarted');          await refresh(); }

  // ── Shared state for pages ────────────────────────────────────────────────
  let state = $state<AppState>({
    hosts, activeHostId, activeHost, selectedContainer,
    containers, stacks, savedStacks, events,
    pods, namespaces, swarmServices, swarmNodes,
    loading, k8sLoading, swarmLoading,
    error, k8sError, swarmError, lastUpdated,
    refresh, refreshK8s, refreshSwarm,
    switchHost, startContainer, stopContainer, restartContainer,
    toast,
  });
  setAppState(state);

  $effect(() => {
    state.hosts = hosts;
    state.activeHostId = activeHostId;
    state.activeHost = activeHost;
    state.selectedContainer = selectedContainer;
    state.containers = containers;
    state.stacks = stacks;
    state.savedStacks = savedStacks;
    state.events = events;
    state.pods = pods;
    state.namespaces = namespaces;
    state.swarmServices = swarmServices;
    state.swarmNodes = swarmNodes;
    state.loading = loading;
    state.k8sLoading = k8sLoading;
    state.swarmLoading = swarmLoading;
    state.error = error;
    state.k8sError = k8sError;
    state.swarmError = swarmError;
    state.lastUpdated = lastUpdated;
  });

  $effect(() => {
    if (route === 'kubernetes') refreshK8s();
    if (route === 'swarm') refreshSwarm();
  });

  onMount(async () => {
    const stopRouter = initRouter();
    await loadHosts();
    await refresh();
    if (route === 'kubernetes') await refreshK8s();
    if (route === 'swarm') await refreshSwarm();
    const disconnect = connect(() => refresh());
    const interval   = setInterval(() => {
      refresh();
      if (route === 'kubernetes') refreshK8s();
      if (route === 'swarm') refreshSwarm();
    }, 10_000);

    function onkey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); openCmdk(); return; }
      if (e.key === 'Escape') { closeCmdk(); return; }
      if (cmdkOpen) {
        if (e.key === 'ArrowDown') { e.preventDefault(); cmdkSel = Math.min(cmdkSel + 1, cmdkResults.length - 1); }
        if (e.key === 'ArrowUp')   { e.preventDefault(); cmdkSel = Math.max(cmdkSel - 1, 0); }
        if (e.key === 'Enter' && cmdkResults[cmdkSel]) cmdkSelect(cmdkResults[cmdkSel]);
      }
    }
    window.addEventListener('keydown', onkey);
    return () => { stopRouter(); disconnect(); clearInterval(interval); window.removeEventListener('keydown', onkey); };
  });
</script>

<!-- ── Drawer ───────────────────────────────────────────────────────────── -->
{#if selectedContainer && activeHostId !== null}
  <ContainerDrawer
    hostId={activeHostId}
    container={selectedContainer}
    onclose={() => (selectedContainer = null)}
    onrefresh={refresh}
  />
{/if}

<!-- ── Add host modal ────────────────────────────────────────────────────── -->
{#if showAddHost}
  <AddEnvironmentModal
    onclose={() => (showAddHost = false)}
    oncreated={async () => { showAddHost = false; await loadHosts(); await refresh(); }}
  />
{/if}

<!-- ── Command palette ───────────────────────────────────────────────────── -->
{#if cmdkOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="cmdk-overlay" onclick={(e) => { if (e.target === e.currentTarget) closeCmdk(); }}>
    <div class="cmdk-box">
      <div class="cmdk-input-wrap">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
        <!-- svelte-ignore a11y_autofocus -->
        <input class="cmdk-input" type="text" placeholder="Search containers…" bind:value={cmdkQuery} autofocus autocomplete="off" />
        <kbd>esc</kbd>
      </div>
      <div class="cmdk-list">
        {#if cmdkResults.length === 0}
          <div class="cmdk-empty">No containers match your search</div>
        {:else}
          {#each cmdkResults as c, i (c.id)}
            <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
            <div class="cmdk-row" class:sel={i === cmdkSel} onclick={() => cmdkSelect(c)}>
              <div class="cmdk-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="7" width="18" height="13" rx="2"/><path d="M3 11h18M8 7V4.5A1.5 1.5 0 0 1 9.5 3h5A1.5 1.5 0 0 1 16 4.5V7"/></svg>
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

<!-- ── Toast stack ───────────────────────────────────────────────────────── -->
<div class="toast-stack">
  {#each toasts as t (t.id)}
    <div class="toast toast-{t.kind}">
      <span class="toast-ic">
        {#if t.kind === 'ok'}
          <svg viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2.4"><path d="M5 13l4 4 10-10"/></svg>
        {:else if t.kind === 'warn'}
          <svg viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2.4"><path d="M12 8v5M12 16h.01"/><path d="M12 3L22 20H2L12 3z"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="2.4"><path d="M6 6l12 12M18 6 6 18"/></svg>
        {/if}
      </span>
      <span>{t.msg}</span>
    </div>
  {/each}
</div>

<!-- ── App shell ─────────────────────────────────────────────────────────── -->
<div class="app">

  <!-- ── Sidebar ────────────────────────────────────────────────────────── -->
  <nav class="sidebar">
    <div class="brand">
      <div class="brand-mark">
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none"><path d="M3 16L12 20L21 16M3 12L12 16L21 12M12 3L21 8L12 13L3 8L12 3Z" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </div>
      <div>
        <div class="brand-name">Beacon</div>
        <div class="brand-sub">Fleet Console</div>
      </div>
    </div>

    <div class="nav-section-label">Monitor</div>
    {#each NAV as item}
      <button class="nav-item" class:active={route === item.id} onclick={() => navigate(item.id)}>
        {@html item.icon}
        {item.label}
        {#if item.id === 'containers' && running > 0}
          <span class="nav-badge">{running}</span>
        {/if}
      </button>
    {/each}

    <EnvironmentSelector
      {hosts}
      activeId={activeHostId}
      onselect={switchHost}
      onadd={() => (showAddHost = true)}
      ondelete={deleteHost}
      onrename={renameHost}
    />

    <div class="sidebar-footer">
      <div class="version">Beacon v0.1.0</div>
    </div>
  </nav>

  <!-- ── Main ──────────────────────────────────────────────────────────── -->
  <div class="main">

    <!-- Topbar -->
    <div class="topbar">
      <div class="topbar-left">
        <h1>{NAV.find(n => n.id === route)?.label ?? ''}</h1>
        <div class="topbar-sub">
          {activeHost?.name ?? ''}
          {lastUpdated ? ` · updated ${lastUpdated}` : ''}
          {#if error}<span class="topbar-err">{error}</span>{/if}
        </div>
      </div>
      <button class="search-trigger" onclick={openCmdk}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
        Search containers…
        <kbd>⌘K</kbd>
      </button>
      <button class="icon-btn" onclick={toggleTheme} title="Toggle theme">
        {#if theme === 'light'}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>
        {:else}
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="4.5"/><path d="M12 2v2.5M12 19.5V22M4.2 4.2l1.8 1.8M18 18l1.8 1.8M2 12h2.5M19.5 12H22M4.2 19.8l1.8-1.8M18 6l1.8-1.8"/></svg>
        {/if}
      </button>
      <button class="icon-btn" onclick={refresh} title="Refresh">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
      </button>
    </div>

    <!-- Content -->
    <div class="content">
      {#if route === 'containers'}
        <ContainersPage />
      {:else if route === 'stacks'}
        <StacksPage />
      {:else if route === 'events'}
        <EventsPage />
      {:else if route === 'swarm'}
        <SwarmPage />
      {:else if route === 'kubernetes'}
        <KubernetesPage />
      {/if}
    </div>
  </div>
</div>

<style>
  /* ── CSS Design Tokens ────────────────────────────────────────────────── */
  :global(:root) {
    --canvas: #F5F6FA;
    --surface: #FFFFFF;
    --surface-2: #FAFBFD;
    --border: #E7E9F0;
    --border-strong: #D7DAE3;
    --ink: #1A1D29;
    --mist: #6B7180;
    --mist-2: #9CA1AF;
    --accent: #4A5FE8;
    --accent-ink: #3849C4;
    --accent-tint: #EDF0FE;
    --green: #1C9A63; --green-tint: #E4F6ED;
    --amber: #B07A0B; --amber-tint: #FCF1D8;
    --red: #D1424B;   --red-tint: #FCE9EA;
    --violet: #6D56D6; --violet-tint: #EEEAFB;
    --shadow-sm: 0 1px 2px rgba(20,22,50,.04), 0 1px 1px rgba(20,22,50,.04);
    --shadow-md: 0 6px 20px rgba(20,22,50,.08), 0 2px 6px rgba(20,22,50,.04);
    --shadow-lg: 0 24px 56px rgba(20,22,50,.18), 0 8px 20px rgba(20,22,50,.08);
    --radius-sm: 9px; --radius-md: 13px; --radius-lg: 20px;
    --sidebar-w: 248px;
  }
  :global([data-theme="dark"]) {
    --canvas: #111319;
    --surface: #1A1D26;
    --surface-2: #1F222C;
    --border: #2A2D38;
    --border-strong: #383B48;
    --ink: #EBECF2;
    --mist: #9CA1B3;
    --mist-2: #6E7385;
    --accent-tint: #1E2240;
    --green-tint: #0F2C1F; --amber-tint: #332A0E; --red-tint: #371D20; --violet-tint: #23203D;
    --shadow-sm: 0 1px 2px rgba(0,0,0,.4);
    --shadow-md: 0 6px 20px rgba(0,0,0,.45);
    --shadow-lg: 0 24px 56px rgba(0,0,0,.6);
  }
  :global(*, *::before, *::after) { box-sizing: border-box; }
  :global(html, body) { margin: 0; padding: 0; }
  :global(body) {
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
    background: var(--canvas); color: var(--ink); font-size: 14px; line-height: 1.5;
    -webkit-font-smoothing: antialiased;
    transition: background .25s ease, color .25s ease;
  }
  :global(button) { font-family: inherit; cursor: pointer; }
  :global(::-webkit-scrollbar) { width: 8px; height: 8px; }
  :global(::-webkit-scrollbar-thumb) { background: var(--border-strong); border-radius: 8px; }
  :global(::-webkit-scrollbar-track) { background: transparent; }

  /* ── Pill ─────────────────────────────────────────────────────────────── */
  :global(.pill) {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 3px 9px 3px 7px; border-radius: 20px;
    font-size: 12px; font-weight: 650; white-space: nowrap;
  }
  :global(.pill .dot) { width: 6px; height: 6px; border-radius: 50%; }
  :global(.pill-green) { background: var(--green-tint); color: var(--green); }
  :global(.pill-green .dot) { background: var(--green); animation: pulse-green 2s infinite; }
  :global(.pill-red)    { background: var(--red-tint);    color: var(--red); }
  :global(.pill-amber)  { background: var(--amber-tint);  color: var(--amber); }
  :global(.pill-violet) { background: var(--violet-tint); color: var(--violet); }
  :global(.pill-mist)   { background: var(--surface-2);   color: var(--mist); }
  @keyframes pulse-green {
    0%   { box-shadow: 0 0 0 0 rgba(28,154,99,.45); }
    70%  { box-shadow: 0 0 0 5px rgba(28,154,99,0); }
    100% { box-shadow: 0 0 0 0 rgba(28,154,99,0); }
  }

  /* ── Layout ───────────────────────────────────────────────────────────── */
  .app { display: grid; grid-template-columns: var(--sidebar-w) 1fr; min-height: 100vh; }

  /* ── Sidebar ──────────────────────────────────────────────────────────── */
  .sidebar {
    background: var(--surface); border-right: 1px solid var(--border);
    padding: 20px 14px; display: flex; flex-direction: column; gap: 2px;
    position: sticky; top: 0; height: 100vh; overflow-y: auto;
  }
  .brand { display: flex; align-items: center; gap: 10px; padding: 6px 10px 22px; }
  .brand-mark {
    width: 30px; height: 30px; border-radius: 9px;
    background: linear-gradient(155deg, var(--accent), #7B8CF0);
    display: flex; align-items: center; justify-content: center;
    flex-shrink: 0; box-shadow: var(--shadow-sm);
  }
  .brand-name { font-family: 'Manrope', sans-serif; font-weight: 800; font-size: 16.5px; letter-spacing: -.02em; color: var(--ink); }
  .brand-sub  { font-size: 10.5px; color: var(--mist-2); font-weight: 600; letter-spacing: .06em; text-transform: uppercase; margin-top: 1px; }

  .nav-section-label {
    font-size: 10.5px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase;
    color: var(--mist-2); padding: 14px 12px 6px;
  }
  .nav-item {
    display: flex; align-items: center; gap: 11px; padding: 9px 12px;
    border-radius: var(--radius-sm); color: var(--mist); font-weight: 600;
    font-size: 13.5px; border: none; background: none; width: 100%;
    text-align: left; transition: background .15s, color .15s;
  }
  .nav-item :global(svg) { width: 17px; height: 17px; flex-shrink: 0; }
  .nav-item:hover  { background: var(--surface-2); color: var(--ink); }
  .nav-item.active { background: var(--accent-tint); color: var(--accent-ink); }
  .nav-badge {
    margin-left: auto; background: var(--red); color: #fff;
    font-size: 10.5px; font-weight: 700; border-radius: 20px; padding: 1px 6px;
  }
  .nav-item.active .nav-badge { background: var(--accent); }

  .sidebar-footer { margin-top: auto; border-top: 1px solid var(--border); padding-top: 12px; }
  .version { font-size: 11px; color: var(--mist-2); padding: 6px 12px; }

  /* ── Main ────────────────────────────────────────────────────────────── */
  .main { min-width: 0; display: flex; flex-direction: column; }

  .topbar {
    position: sticky; top: 0; z-index: 30; background: var(--canvas);
    display: flex; align-items: center; gap: 12px; padding: 16px 28px;
    border-bottom: 1px solid var(--border); backdrop-filter: blur(6px);
  }
  .topbar-left { flex: 1; min-width: 0; }
  .topbar-left h1 {
    font-family: 'Manrope', sans-serif; font-size: 19px; font-weight: 800;
    letter-spacing: -.01em; margin: 0; color: var(--ink);
  }
  .topbar-sub { color: var(--mist); font-size: 12.5px; margin-top: 1px; }
  .topbar-err { color: var(--red); font-size: 11.5px; margin-left: 8px; }

  .search-trigger {
    display: flex; align-items: center; gap: 10px; background: var(--surface);
    border: 1px solid var(--border); border-radius: 10px; padding: 8px 12px;
    color: var(--mist); font-size: 13px; min-width: 220px;
    transition: border-color .15s, color .15s;
  }
  .search-trigger:hover { border-color: var(--border-strong); color: var(--ink); }
  .search-trigger :global(svg) { width: 14px; height: 14px; flex-shrink: 0; }
  .search-trigger kbd {
    margin-left: auto; background: var(--surface-2); border: 1px solid var(--border);
    border-radius: 5px; padding: 1px 5px; font-size: 11px;
    font-family: 'JetBrains Mono', monospace; color: var(--mist-2);
  }
  .icon-btn {
    width: 36px; height: 36px; border-radius: 10px; border: 1px solid var(--border);
    background: var(--surface); display: flex; align-items: center; justify-content: center;
    color: var(--mist); transition: background .15s, color .15s; flex-shrink: 0;
  }
  .icon-btn:hover { background: var(--surface-2); color: var(--ink); }
  .icon-btn :global(svg) { width: 17px; height: 17px; }

  .content { padding: 24px 28px 60px; }

  /* ── Command palette ─────────────────────────────────────────────────── */
  .cmdk-overlay {
    position: fixed; inset: 0; background: rgba(20,22,40,.4); z-index: 200;
    display: flex; align-items: flex-start; justify-content: center;
    padding-top: 13vh; backdrop-filter: blur(2px);
  }
  .cmdk-box {
    width: 560px; max-width: 90vw; background: var(--surface);
    border-radius: var(--radius-lg); box-shadow: var(--shadow-lg);
    border: 1px solid var(--border); overflow: hidden;
  }
  .cmdk-input-wrap {
    display: flex; align-items: center; gap: 10px; padding: 14px 18px;
    border-bottom: 1px solid var(--border);
  }
  .cmdk-input-wrap :global(svg) { width: 16px; height: 16px; color: var(--mist-2); flex-shrink: 0; }
  .cmdk-input-wrap kbd {
    font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--mist-2);
    background: var(--surface-2); border: 1px solid var(--border); border-radius: 5px; padding: 2px 6px;
  }
  .cmdk-input {
    flex: 1; border: none; background: none; font-size: 14.5px; color: var(--ink);
    outline: none; font-family: 'Inter', sans-serif;
  }
  .cmdk-list { max-height: 340px; overflow-y: auto; padding: 8px; }
  .cmdk-row {
    display: flex; align-items: center; gap: 11px; padding: 9px 10px;
    border-radius: 9px; cursor: pointer; transition: background .1s;
  }
  .cmdk-row:hover, .cmdk-row.sel { background: var(--accent-tint); }
  .cmdk-icon {
    width: 30px; height: 30px; border-radius: 8px; background: var(--surface-2);
    display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  }
  .cmdk-icon :global(svg) { width: 14px; height: 14px; color: var(--mist); }
  .cmdk-nm { font-weight: 650; font-size: 13px; flex: 1; }
  .cmdk-ty { font-size: 11.5px; color: var(--mist-2); font-family: 'JetBrains Mono', monospace; }
  .cmdk-empty { padding: 30px; text-align: center; color: var(--mist-2); font-size: 13px; }

  /* ── Toasts ──────────────────────────────────────────────────────────── */
  .toast-stack { position: fixed; bottom: 22px; right: 22px; display: flex; flex-direction: column; gap: 8px; z-index: 300; }
  .toast {
    display: flex; align-items: center; gap: 10px; background: var(--surface);
    border: 1px solid var(--border); box-shadow: var(--shadow-md);
    border-radius: 11px; padding: 11px 15px; font-size: 13px; font-weight: 600;
    min-width: 240px; animation: toastin .25s ease;
  }
  @keyframes toastin { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
  .toast-ic {
    width: 22px; height: 22px; border-radius: 50%;
    display: flex; align-items: center; justify-content: center; flex-shrink: 0;
  }
  .toast-ic :global(svg) { width: 12px; height: 12px; }
  .toast-ok   .toast-ic { background: var(--green); }
  .toast-warn .toast-ic { background: var(--amber); }
  .toast-err  .toast-ic { background: var(--red); }
</style>
