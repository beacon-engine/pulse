<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import { getAppState } from '../lib/appContext';
  import { navigate } from '../lib/router';
  import Pagination from '../components/Pagination.svelte';
  import AddEnvironmentModal from '../components/AddEnvironmentModal.svelte';
  import type { Environment } from '../types/environments';

  const app = getAppState();
  const PER_PAGE = 20;

  let hosts = $state<Environment[]>([]);
  let total = $state(0);
  let page = $state(1);
  let search = $state('');
  let loading = $state(true);
  let error = $state('');
  let showAdd = $state(false);

  async function load() {
    loading = true; error = '';
    try {
      const p = await api.listHosts({ page, perPage: PER_PAGE, search });
      hosts = p.items; total = p.total;
    } catch (e: any) {
      error = e?.response?.data ?? String(e);
    } finally {
      loading = false;
    }
  }

  onMount(load);

  function debounce(fn: () => void, ms = 300) {
    let t: ReturnType<typeof setTimeout>;
    return () => { clearTimeout(t); t = setTimeout(fn, ms); };
  }
  const onSearch = debounce(() => { page = 1; load(); });
  function gotoPage(p: number) { page = p; load(); }

  async function select(host: Environment) {
    await app.switchHost(host);
    navigate('containers');
  }

  async function rename(host: Environment) {
    const name = prompt('Rename host', host.name);
    if (!name || name.trim() === host.name) return;
    try {
      await api.renameHost(host.id, name.trim());
      app.toast(`Renamed to "${name.trim()}"`);
      await load();
    } catch (e: any) { app.toast(e?.response?.data ?? String(e), 'err'); }
  }

  async function remove(host: Environment) {
    if (!confirm(`Remove host "${host.name}"? This only disconnects it from Pulse.`)) return;
    try {
      await api.deleteHost(host.id);
      app.toast(`Host "${host.name}" removed`, 'warn');
      if (page > 1 && hosts.length === 1) page -= 1;
      await load();
    } catch (e: any) { app.toast(e?.response?.data ?? String(e), 'err'); }
  }
</script>

<div class="hosts">
  <div class="toolbar">
    <input class="search-box" placeholder="Search hosts…" bind:value={search} oninput={onSearch} />
    <span class="count">{total} host{total !== 1 ? 's' : ''}</span>
    <button class="btn-primary" onclick={() => (showAdd = true)}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14M5 12h14"/></svg>
      Add host
    </button>
  </div>

  {#if error}<div class="err-box">{error}</div>{/if}

  {#if loading}
    <div class="muted">Loading hosts…</div>
  {:else if hosts.length === 0}
    <div class="muted">{search ? 'No hosts match your search' : 'No hosts yet'}</div>
  {:else}
    <div class="table">
      <div class="thead"><div>Host</div><div>Docker endpoint</div><div>Added</div><div class="ta-r">Actions</div></div>
      {#each hosts as h (h.id)}
        <div class="trow" class:active={app.activeHostId === h.id}>
          <div class="cell-host">
            <span class="dot" class:local={h.is_local}></span>
            <button class="host-name" onclick={() => select(h)} title="Switch to this host">{h.name}</button>
            {#if h.is_local}<span class="tag">local</span>{/if}
            {#if app.activeHostId === h.id}<span class="tag active-tag">active</span>{/if}
          </div>
          <div class="mono muted sm truncate">{h.docker_host}</div>
          <div class="muted sm">{h.created_at ? new Date(h.created_at).toLocaleDateString() : '—'}</div>
          <div class="ta-r">
            <button class="btn-ghost" onclick={() => select(h)}>Select</button>
            <button class="btn-ghost" onclick={() => rename(h)}>Rename</button>
            <button class="btn-danger" onclick={() => remove(h)} disabled={h.is_local}>Delete</button>
          </div>
        </div>
      {/each}
    </div>
    <Pagination {page} perPage={PER_PAGE} {total} onpage={gotoPage} />
  {/if}
</div>

{#if showAdd}
  <AddEnvironmentModal
    onclose={() => (showAdd = false)}
    oncreated={async () => { showAdd = false; await load(); }}
  />
{/if}

<style>
  .hosts { display: flex; flex-direction: column; gap: 16px; }
  .toolbar { display: flex; align-items: center; gap: 12px; }
  .search-box {
    flex: 1; max-width: 360px; padding: 8px 12px; border: 1px solid var(--border);
    border-radius: var(--radius-sm); background: var(--surface-2); color: var(--ink);
    font-size: 13px; font-family: inherit; outline: none;
  }
  .search-box:focus { border-color: var(--accent); background: var(--surface); }
  .count { font-size: 12.5px; color: var(--mist-2); }
  .btn-primary {
    margin-left: auto; display: inline-flex; align-items: center; gap: 6px;
    padding: 8px 14px; border: none; border-radius: var(--radius-sm);
    background: var(--accent); color: #fff; font-size: 13px; font-weight: 650; cursor: pointer;
  }
  .btn-primary:hover { filter: brightness(1.07); }
  .btn-primary :global(svg) { width: 13px; height: 13px; }

  .table { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); overflow: hidden; }
  .thead, .trow { display: grid; grid-template-columns: 2fr 2fr 1fr 1.6fr; align-items: center; gap: 12px; padding: 11px 16px; }
  .thead { font-size: 11px; text-transform: uppercase; letter-spacing: .05em; color: var(--mist-2); font-weight: 700; border-bottom: 1px solid var(--border); background: var(--surface-2); }
  .trow { border-top: 1px solid var(--border); }
  .trow:hover { background: var(--surface-2); }
  .trow.active { background: var(--accent-tint); }
  .cell-host { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--mist-2); flex-shrink: 0; }
  .dot.local { background: var(--accent); }
  .host-name { background: none; border: none; padding: 0; font-weight: 650; font-size: 13.5px; color: var(--ink); cursor: pointer; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .host-name:hover { color: var(--accent); }
  .tag { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; color: var(--mist-2); background: var(--surface-2); padding: 1px 6px; border-radius: 20px; }
  .active-tag { color: var(--accent-ink); background: var(--accent-tint); }
  .mono { font-family: 'JetBrains Mono', monospace; }
  .sm { font-size: 12px; } .muted { color: var(--mist-2); }
  .truncate { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ta-r { text-align: right; }
  .btn-ghost { background: none; border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 5px 10px; font-size: 12.5px; color: var(--mist); margin-left: 6px; cursor: pointer; }
  .btn-ghost:hover { color: var(--ink); border-color: var(--border-strong); }
  .btn-danger { background: none; border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 5px 10px; font-size: 12.5px; color: var(--red); margin-left: 6px; cursor: pointer; }
  .btn-danger:hover:not(:disabled) { background: var(--red-tint); }
  .btn-danger:disabled { opacity: .35; cursor: default; }
  .err-box { background: var(--red-tint); color: var(--red); border-radius: var(--radius-sm); padding: 10px 12px; font-size: 12.5px; }
</style>
