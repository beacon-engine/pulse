<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import { getAppState } from '../lib/appContext';
  import Pagination from '../components/Pagination.svelte';
  import type { DockerImage, DockerVolume, HostNetwork, SystemInfo } from '../types/containers';

  const app = getAppState();
  const PER_PAGE = 50;

  // Debounce helper for search inputs.
  function debounce(fn: () => void, ms = 300) {
    let t: ReturnType<typeof setTimeout>;
    return () => { clearTimeout(t); t = setTimeout(fn, ms); };
  }

  type Tab = 'images' | 'volumes' | 'networks' | 'system';
  const TABS: Tab[] = ['images', 'volumes', 'networks', 'system'];
  let tab = $state<Tab>('images');

  // ── System info ───────────────────────────────────────────────────────────
  let sysInfo    = $state<SystemInfo | null>(null);
  let sysError   = $state('');
  let sysLoading = $state(false);

  async function loadSystem() {
    if (!app.activeHostId) return;
    sysLoading = true; sysError = '';
    try { sysInfo = await api.getSystemInfo(app.activeHostId); }
    catch (e: any) { sysError = String(e); }
    finally { sysLoading = false; }
  }

  // ── Images ────────────────────────────────────────────────────────────────
  let images   = $state<DockerImage[]>([]);
  let imgTotal = $state(0);
  let imgPage  = $state(1);
  let imgSearch = $state('');
  let imgError = $state('');
  let imgBusy  = $state(false);

  async function loadImages() {
    if (!app.activeHostId) return;
    imgBusy = true; imgError = '';
    try {
      const p = await api.getImages(app.activeHostId, { page: imgPage, perPage: PER_PAGE, search: imgSearch });
      images = p.items; imgTotal = p.total;
    }
    catch (e: any) { imgError = String(e); }
    finally { imgBusy = false; }
  }

  const onImgSearch = debounce(() => { imgPage = 1; loadImages(); });
  function gotoImgPage(p: number) { imgPage = p; loadImages(); }

  async function deleteImage(id: string) {
    if (!app.activeHostId) return;
    if (!confirm('Delete this image?')) return;
    imgBusy = true;
    try { await api.deleteImage(app.activeHostId, id); await loadImages(); app.toast('Image deleted', 'warn'); }
    catch (e: any) { imgError = e?.response?.data ?? String(e); imgBusy = false; }
  }

  async function pruneImages() {
    if (!app.activeHostId || !confirm('Delete all dangling images?')) return;
    imgBusy = true;
    try {
      const r = await api.pruneDanglingImages(app.activeHostId);
      await loadImages();
      app.toast(`Pruned — ${fmtBytes(r.reclaimed)} freed`);
    } catch (e: any) { imgError = String(e); imgBusy = false; }
  }

  // ── Volumes ───────────────────────────────────────────────────────────────
  let volumes  = $state<DockerVolume[]>([]);
  let volTotal = $state(0);
  let volPage  = $state(1);
  let volSearch = $state('');
  let volError = $state('');
  let volBusy  = $state(false);

  async function loadVolumes() {
    if (!app.activeHostId) return;
    volBusy = true; volError = '';
    try {
      const p = await api.getVolumes(app.activeHostId, { page: volPage, perPage: PER_PAGE, search: volSearch });
      volumes = p.items; volTotal = p.total;
    }
    catch (e: any) { volError = String(e); }
    finally { volBusy = false; }
  }

  const onVolSearch = debounce(() => { volPage = 1; loadVolumes(); });
  function gotoVolPage(p: number) { volPage = p; loadVolumes(); }

  async function deleteVolume(name: string) {
    if (!app.activeHostId) return;
    if (!confirm(`Delete volume "${name}"?`)) return;
    volBusy = true;
    try { await api.deleteVolume(app.activeHostId, name); await loadVolumes(); app.toast('Volume deleted', 'warn'); }
    catch (e: any) { volError = e?.response?.data ?? String(e); volBusy = false; }
  }

  async function pruneVolumes() {
    if (!app.activeHostId || !confirm('Delete all volumes not attached to a container?')) return;
    volBusy = true;
    try {
      const r = await api.pruneVolumes(app.activeHostId);
      await loadVolumes();
      app.toast(`Pruned — ${fmtBytes(r.reclaimed)} freed`);
    } catch (e: any) { volError = String(e); volBusy = false; }
  }

  // ── Networks ──────────────────────────────────────────────────────────────
  let networks  = $state<HostNetwork[]>([]);
  let netError  = $state('');
  let netBusy   = $state(false);

  async function loadNetworks() {
    if (!app.activeHostId) return;
    netBusy = true; netError = '';
    try { networks = await api.listHostNetworks(app.activeHostId); }
    catch (e: any) { netError = String(e); }
    finally { netBusy = false; }
  }

  const TAB_ICONS: Record<Tab, string> = {
    images:   `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`,
    volumes:  `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/></svg>`,
    networks: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M12 2a10 10 0 1 0 0 20A10 10 0 0 0 12 2z"/><path d="M2 12h20M12 2c-2.5 3-4 6.3-4 10s1.5 7 4 10M12 2c2.5 3 4 6.3 4 10s-1.5 7-4 10"/></svg>`,
    system:   `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14M4.93 19.07a10 10 0 0 1 0-14.14"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07M8.46 15.54a5 5 0 0 1 0-7.07"/></svg>`,
  };

  // ── Tab switching ─────────────────────────────────────────────────────────
  function switchTab(t: Tab) {
    tab = t;
    if (t === 'images')  loadImages();
    if (t === 'volumes') loadVolumes();
    if (t === 'networks') loadNetworks();
    if (t === 'system')  loadSystem();
  }

  // ── Helpers ───────────────────────────────────────────────────────────────
  function fmtBytes(b: number): string {
    if (b === 0) return '0 B';
    if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`;
    if (b < 1073741824) return `${(b / 1048576).toFixed(1)} MB`;
    return `${(b / 1073741824).toFixed(2)} GB`;
  }

  function fmtDate(ts: number): string {
    if (!ts) return '—';
    return new Date(ts * 1000).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  }

  // Dangling stats reflect the current page (the server sorts + paginates).
  let danglingCount = $derived(images.filter(i => i.dangling).length);
  let danglingSize  = $derived(images.filter(i => i.dangling).reduce((a, i) => a + i.size, 0));

  onMount(() => {
    if (app.activeHostId) loadImages();
  });

  // If host loads after mount, kick off initial load
  let prevHostId = '';
  $effect(() => {
    const hid = app.activeHostId;
    if (hid && hid !== prevHostId) {
      prevHostId = hid;
      loadImages();
      tab = 'images';
    }
  });
</script>

{#if !app.activeHostId}
  <div class="empty-state"><p>Select a host to manage its resources</p></div>
{:else}
  <!-- Tabs -->
  <div class="res-tabs">
    {#each TABS as t}
      <button class="rtab" class:active={tab === t} onclick={() => switchTab(t)}>
        {@html TAB_ICONS[t]}
        {t.charAt(0).toUpperCase() + t.slice(1)}
        {#if t === 'images' && danglingCount > 0}
          <span class="rtab-badge">{danglingCount} dangling</span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- IMAGES -->
  {#if tab === 'images'}
    <div class="tab-toolbar">
      {#if danglingCount > 0}
        <button class="btn-danger" onclick={pruneImages} disabled={imgBusy}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/><path d="M10 11v6M14 11v6M9 6V4h6v2"/></svg>
          Prune {danglingCount} dangling ({fmtBytes(danglingSize)})
        </button>
      {/if}
      <button class="btn-ghost btn-sm" onclick={loadImages} disabled={imgBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
        Refresh
      </button>
      <input class="search-box" placeholder="Search images…" bind:value={imgSearch} oninput={onImgSearch} />
      {#if imgBusy}<span class="loading-hint">Loading…</span>{/if}
    </div>
    {#if imgError}<div class="err-box">{imgError}</div>{/if}
    {#if images.length === 0 && !imgBusy}
      <div class="empty-hint">{imgSearch ? 'No images match your search' : 'No images found'}</div>
    {:else}
      <table class="res-table">
        <thead>
          <tr><th>ID</th><th>Tags / Status</th><th>Size</th><th>Created</th><th></th></tr>
        </thead>
        <tbody>
          {#each images as img (img.id)}
            <tr class:dangling-row={img.dangling}>
              <td class="mono small id-cell">{img.id.replace('sha256:', '').slice(0, 12)}</td>
              <td>
                {#if img.dangling}
                  <span class="pill-tag dangling">dangling</span>
                {:else if img.tags.length === 0}
                  <span class="muted small">no tags</span>
                {:else}
                  {#each img.tags as tag}
                    <span class="pill-tag">{tag}</span>
                  {/each}
                {/if}
              </td>
              <td class="mono small">{fmtBytes(img.size)}</td>
              <td class="small muted">{fmtDate(img.created)}</td>
              <td>
                <button class="row-del" onclick={() => deleteImage(img.id)} disabled={imgBusy} title="Delete image">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/></svg>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      <Pagination page={imgPage} perPage={PER_PAGE} total={imgTotal} onpage={gotoImgPage} />
    {/if}

  <!-- VOLUMES -->
  {:else if tab === 'volumes'}
    <div class="tab-toolbar">
      <button class="btn-danger" onclick={pruneVolumes} disabled={volBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/></svg>
        Prune unused
      </button>
      <button class="btn-ghost btn-sm" onclick={loadVolumes} disabled={volBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
        Refresh
      </button>
      <input class="search-box" placeholder="Search volumes…" bind:value={volSearch} oninput={onVolSearch} />
      {#if volBusy}<span class="loading-hint">Loading…</span>{/if}
    </div>
    {#if volError}<div class="err-box">{volError}</div>{/if}
    {#if volumes.length === 0 && !volBusy}
      <div class="empty-hint">{volSearch ? 'No volumes match your search' : 'No volumes found'}</div>
    {:else}
      <table class="res-table">
        <thead><tr><th>Name</th><th>Driver</th><th>Mount point</th><th>Created</th><th></th></tr></thead>
        <tbody>
          {#each volumes as vol (vol.name)}
            <tr>
              <td class="mono small">{vol.name}</td>
              <td class="small">{vol.driver}</td>
              <td class="mono small muted truncate">{vol.mountpoint}</td>
              <td class="small muted">{vol.created ? new Date(vol.created).toLocaleDateString() : '—'}</td>
              <td>
                <button class="row-del" onclick={() => deleteVolume(vol.name)} disabled={volBusy} title="Delete volume">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6l-1 14H6L5 6"/></svg>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      <Pagination page={volPage} perPage={PER_PAGE} total={volTotal} onpage={gotoVolPage} />
    {/if}

  <!-- NETWORKS -->
  {:else if tab === 'networks'}
    <div class="tab-toolbar">
      <button class="btn-ghost btn-sm" onclick={loadNetworks} disabled={netBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
        Refresh
      </button>
      {#if netBusy}<span class="loading-hint">Loading…</span>{/if}
    </div>
    {#if netError}<div class="err-box">{netError}</div>{/if}
    {#if networks.length === 0 && !netBusy}
      <div class="empty-hint">No networks found</div>
    {:else}
      <table class="res-table">
        <thead><tr><th>Name</th><th>ID</th><th>Driver</th><th>Scope</th><th>Internal</th></tr></thead>
        <tbody>
          {#each networks as net (net.id)}
            <tr>
              <td class="fw">{net.name}</td>
              <td class="mono small muted">{net.id.slice(0, 12)}</td>
              <td class="small">{net.driver}</td>
              <td class="small">{net.scope}</td>
              <td>
                {#if net.internal}<span class="pill-tag">internal</span>
                {:else}<span class="muted small">—</span>{/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}

  <!-- SYSTEM INFO -->
  {:else if tab === 'system'}
    {#if sysError}<div class="err-box">{sysError}</div>
    {:else if sysLoading}<div class="empty-hint">Loading…</div>
    {:else if sysInfo}
      <div class="sys-grid">
        <div class="sys-card"><div class="sys-lbl">Docker version</div><div class="sys-val mono">{sysInfo.docker_version || '—'}</div></div>
        <div class="sys-card"><div class="sys-lbl">API version</div><div class="sys-val mono">{sysInfo.api_version || '—'}</div></div>
        <div class="sys-card"><div class="sys-lbl">OS</div><div class="sys-val">{sysInfo.os || '—'}</div></div>
        <div class="sys-card"><div class="sys-lbl">Architecture</div><div class="sys-val mono">{sysInfo.arch || '—'}</div></div>
        <div class="sys-card"><div class="sys-lbl">Kernel</div><div class="sys-val mono">{sysInfo.kernel || '—'}</div></div>
        <div class="sys-card"><div class="sys-lbl">CPUs</div><div class="sys-val">{sysInfo.cpus}</div></div>
        <div class="sys-card"><div class="sys-lbl">Total memory</div><div class="sys-val">{fmtBytes(sysInfo.total_memory)}</div></div>
        <div class="sys-card green"><div class="sys-lbl">Running containers</div><div class="sys-val">{sysInfo.containers_running}</div></div>
        <div class="sys-card red"><div class="sys-lbl">Stopped containers</div><div class="sys-val">{sysInfo.containers_stopped}</div></div>
        <div class="sys-card"><div class="sys-lbl">Images</div><div class="sys-val">{sysInfo.images}</div></div>
      </div>
      <div class="tab-toolbar" style="margin-top: 16px;">
        <button class="btn-ghost btn-sm" onclick={loadSystem}>Refresh</button>
      </div>
    {:else}
      <div class="empty-hint">Click refresh to load system info</div>
      <div class="tab-toolbar" style="margin-top: 8px;">
        <button class="btn-ghost btn-sm" onclick={loadSystem}>Load system info</button>
      </div>
    {/if}
  {/if}
{/if}

<style>
  .empty-state { display: flex; justify-content: center; padding: 80px 0; color: var(--mist); font-size: 14px; }
  .empty-hint  { text-align: center; color: var(--mist-2); font-size: 13px; padding: 28px 0; }
  .loading-hint { font-size: 12px; color: var(--mist-2); }

  /* Tab strip */
  .res-tabs { display: flex; gap: 4px; margin-bottom: 18px; flex-wrap: wrap; }
  .rtab {
    display: flex; align-items: center; gap: 8px; padding: 9px 16px;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface); color: var(--mist); font-size: 13px; font-weight: 650;
    cursor: pointer; transition: all .12s;
  }
  .rtab :global(svg) { width: 16px; height: 16px; }
  .rtab:hover { background: var(--surface-2); color: var(--ink); }
  .rtab.active { background: var(--accent-tint); color: var(--accent-ink); border-color: var(--accent-tint); }
  .rtab-badge {
    font-size: 10.5px; font-weight: 700; background: var(--red-tint); color: var(--red);
    padding: 1px 7px; border-radius: 20px; margin-left: 2px;
  }

  /* Toolbar */
  .tab-toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 14px; }
  .search-box {
    margin-left: auto; padding: 6px 11px; border: 1px solid var(--border);
    border-radius: var(--radius-sm); background: var(--surface-2); color: var(--ink);
    font-size: 12.5px; font-family: inherit; outline: none; min-width: 200px;
  }
  .search-box:focus { border-color: var(--accent); background: var(--surface); }
  .btn-danger {
    display: inline-flex; align-items: center; gap: 6px; padding: 7px 13px;
    border-radius: var(--radius-sm); border: 1px solid var(--red);
    background: var(--red-tint); color: var(--red); font-size: 12.5px; font-weight: 650;
    cursor: pointer; transition: all .12s;
  }
  .btn-danger:hover:not(:disabled) { background: var(--red); color: #fff; }
  .btn-danger:disabled { opacity: .4; cursor: not-allowed; }
  .btn-danger :global(svg) { width: 13px; height: 13px; }
  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px; padding: 7px 12px;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600;
    cursor: pointer; transition: all .12s;
  }
  .btn-ghost:hover:not(:disabled) { background: var(--surface); border-color: var(--border-strong); }
  .btn-ghost:disabled { opacity: .4; cursor: not-allowed; }
  .btn-ghost :global(svg) { width: 13px; height: 13px; }
  .btn-sm { padding: 5px 10px; font-size: 12px; }

  /* Table */
  .res-table { width: 100%; border-collapse: collapse; }
  .res-table th {
    text-align: left; padding: 9px 14px; font-size: 11px; font-weight: 700;
    text-transform: uppercase; letter-spacing: .05em; color: var(--mist-2);
    border-bottom: 1px solid var(--border); background: var(--surface-2); white-space: nowrap;
  }
  .res-table td { padding: 10px 14px; border-bottom: 1px solid var(--border); vertical-align: middle; }
  .res-table tr:last-child td { border-bottom: none; }
  .res-table tr:hover td { background: var(--surface-2); }
  .dangling-row td { opacity: .75; }
  .mono { font-family: 'JetBrains Mono', monospace; }
  .small { font-size: 12px; }
  .muted { color: var(--mist-2); }
  .fw { font-weight: 650; }
  .truncate { max-width: 240px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .id-cell { font-size: 11.5px; color: var(--mist); }

  .pill-tag {
    display: inline-block; padding: 2px 8px; border-radius: 20px; font-size: 11px;
    font-weight: 700; background: var(--accent-tint); color: var(--accent-ink);
    margin: 1px 3px 1px 0; max-width: 280px; overflow: hidden; text-overflow: ellipsis;
    white-space: nowrap; vertical-align: middle;
  }
  .pill-tag.dangling { background: var(--amber-tint); color: var(--amber); }

  .row-del {
    width: 28px; height: 28px; border-radius: 6px; border: none;
    background: transparent; color: var(--mist-2); display: flex; align-items: center;
    justify-content: center; cursor: pointer; opacity: 0; transition: all .12s;
  }
  .res-table tr:hover .row-del { opacity: 1; }
  .row-del:hover:not(:disabled) { background: var(--red-tint); color: var(--red); }
  .row-del:disabled { opacity: .3; cursor: not-allowed; }
  .row-del :global(svg) { width: 13px; height: 13px; }

  /* System info */
  .sys-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(190px, 1fr)); gap: 12px; }
  .sys-card { background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 14px 16px; }
  .sys-card.green { border-color: var(--green); background: var(--green-tint); }
  .sys-card.red   { border-color: var(--red);   background: var(--red-tint); }
  .sys-lbl { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; color: var(--mist-2); margin-bottom: 6px; }
  .sys-card.green .sys-lbl { color: var(--green); }
  .sys-card.red   .sys-lbl { color: var(--red); }
  .sys-val { font-size: 15px; font-weight: 700; color: var(--ink); }
  .sys-val.mono { font-family: 'JetBrains Mono', monospace; font-size: 13px; }
  .err-box { padding: 10px 12px; background: var(--red-tint); color: var(--red); border-radius: var(--radius-sm); font-size: 12.5px; margin-bottom: 14px; }
</style>
