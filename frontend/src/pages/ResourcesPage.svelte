<script lang="ts">
  import { resourcesStore, type ResourceTab } from "../stores/resource.svelte";
  import { hostsStore } from "../stores/hosts.svelte";
  import Pagination from "../components/Pagination.svelte";
  import { fmtDate, fmtBytes } from "../utils/fmt";

  const PER_PAGE = 50;

  function debounce(fn: () => void, ms = 300) {
    let t: ReturnType<typeof setTimeout>;
    return () => {
      clearTimeout(t);
      t = setTimeout(fn, ms);
    };
  }

  const TABS: ResourceTab[] = ["images", "volumes", "networks", "system"];

  const onImgSearch = debounce(() => {
    resourcesStore.imgPage = 1;
    resourcesStore.loadImages(PER_PAGE);
  });

  function gotoImgPage(p: number) {
    resourcesStore.imgPage = p;
    resourcesStore.loadImages(PER_PAGE);
  }

  const onVolSearch = debounce(() => {
    resourcesStore.volPage = 1;
    resourcesStore.loadVolumes(PER_PAGE);
  });

  function gotoVolPage(p: number) {
    resourcesStore.volPage = p;
    resourcesStore.loadVolumes(PER_PAGE);
  }

  async function handleDeleteImage(id: string) {
    if (!confirm("Delete this image?")) return;
    await resourcesStore.deleteImage(id);
  }

  async function handlePruneImages() {
    if (!confirm("Delete all dangling images?")) return;
    await resourcesStore.pruneImages();
  }

  async function handleDeleteVolume(name: string) {
    if (!confirm(`Delete volume "${name}"?`)) return;
    await resourcesStore.deleteVolume(name);
  }

  async function handlePruneVolumes() {
    if (!confirm("Delete all volumes not attached to a container?")) return;
    await resourcesStore.pruneVolumes();
  }

  const TAB_ICONS: Record<ResourceTab, string> = {
    images: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`,
    volumes: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/><path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3"/></svg>`,
    networks: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M12 2a10 10 0 1 0 0 20A10 10 0 0 0 12 2z"/><path d="M2 12h20M12 2c-2.5 3-4 6.3-4 10s1.5 7 4 10M12 2c2.5 3 4 6.3 4 10s-1.5 7-4 10"/></svg>`,
    system: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14M4.93 19.07a10 10 0 0 1 0-14.14"/><path d="M15.54 8.46a5 5 0 0 1 0 7.07M8.46 15.54a5 5 0 0 1 0-7.07"/></svg>`,
  };

  function switchTab(t: ResourceTab) {
    resourcesStore.tab = t;
    resourcesStore.loadActiveTab();
  }

  // Load once whenever the active host becomes available or changes
  $effect(() => {
    const hostId = hostsStore.activeHostId;
    if (hostId) {
      resourcesStore.loadActiveTab();
    }
  });
</script>

{#if !hostsStore.activeHostId}
  <div class="empty-state"><p>Select a host to manage its resources</p></div>
{:else}
  <!-- Tabs -->
  <div class="res-tabs">
    {#each TABS as t}
      <button class="rtab" class:active={resourcesStore.tab === t} onclick={() => switchTab(t)}>
        {@html TAB_ICONS[t]}
        {t.charAt(0).toUpperCase() + t.slice(1)}
        {#if t === "images" && resourcesStore.danglingCount > 0}
          <span class="rtab-badge">{resourcesStore.danglingCount} dangling</span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- IMAGES -->
  {#if resourcesStore.tab === "images"}
    <div class="tab-toolbar">
      {#if resourcesStore.danglingCount > 0}
        <button class="btn-danger" onclick={handlePruneImages} disabled={resourcesStore.imgBusy}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14H6L5 6" /><path d="M10 11v6M14 11v6M9 6V4h6v2" />
          </svg>
          Prune {resourcesStore.danglingCount} dangling ({fmtBytes(resourcesStore.danglingSize)})
        </button>
      {/if}
      <button class="btn-ghost btn-sm" onclick={() => resourcesStore.loadImages(PER_PAGE)} disabled={resourcesStore.imgBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" /><path d="M3 3v5h5" />
        </svg>
        Refresh
      </button>
      <input class="search-box" placeholder="Search images…" bind:value={resourcesStore.imgSearch} oninput={onImgSearch} />
      {#if resourcesStore.imgBusy}<span class="loading-hint">Loading…</span>{/if}
    </div>
    {#if resourcesStore.imgError}<div class="err-box">{resourcesStore.imgError}</div>{/if}
    {#if resourcesStore.images.length === 0 && !resourcesStore.imgBusy}
      <div class="empty-hint">{resourcesStore.imgSearch ? "No images match your search" : "No images found"}</div>
    {:else}
      <table class="res-table">
        <thead>
          <tr><th>ID</th><th>Tags / Status</th><th>Size</th><th>Created</th><th></th></tr>
        </thead>
        <tbody>
          {#each resourcesStore.images as img (img.id)}
            <tr class:dangling-row={img.dangling}>
              <td class="mono small id-cell">{img.id.replace("sha256:", "").slice(0, 12)}</td>
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
                <button class="row-del" onclick={() => handleDeleteImage(img.id)} disabled={resourcesStore.imgBusy} title="Delete image">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14H6L5 6" />
                  </svg>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      <Pagination page={resourcesStore.imgPage} perPage={PER_PAGE} total={resourcesStore.imgTotal} onpage={gotoImgPage} />
    {/if}

  <!-- VOLUMES -->
  {:else if resourcesStore.tab === "volumes"}
    <div class="tab-toolbar">
      <button class="btn-danger" onclick={handlePruneVolumes} disabled={resourcesStore.volBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14H6L5 6" />
        </svg>
        Prune unused
      </button>
      <button class="btn-ghost btn-sm" onclick={() => resourcesStore.loadVolumes(PER_PAGE)} disabled={resourcesStore.volBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" /><path d="M3 3v5h5" />
        </svg>
        Refresh
      </button>
      <input class="search-box" placeholder="Search volumes…" bind:value={resourcesStore.volSearch} oninput={onVolSearch} />
      {#if resourcesStore.volBusy}<span class="loading-hint">Loading…</span>{/if}
    </div>
    {#if resourcesStore.volError}<div class="err-box">{resourcesStore.volError}</div>{/if}
    {#if resourcesStore.volumes.length === 0 && !resourcesStore.volBusy}
      <div class="empty-hint">{resourcesStore.volSearch ? "No volumes match your search" : "No volumes found"}</div>
    {:else}
      <table class="res-table">
        <thead><tr><th>Name</th><th>Driver</th><th>Mount point</th><th>Created</th><th></th></tr></thead>
        <tbody>
          {#each resourcesStore.volumes as vol (vol.name)}
            <tr>
              <td class="mono small">{vol.name}</td>
              <td class="small">{vol.driver}</td>
              <td class="mono small muted truncate">{vol.mountpoint}</td>
              <td class="small muted">{vol.created ? new Date(vol.created).toLocaleDateString() : "—"}</td>
              <td>
                <button class="row-del" onclick={() => handleDeleteVolume(vol.name)} disabled={resourcesStore.volBusy} title="Delete volume">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14H6L5 6" />
                  </svg>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
      <Pagination page={resourcesStore.volPage} perPage={PER_PAGE} total={resourcesStore.volTotal} onpage={gotoVolPage} />
    {/if}

  <!-- NETWORKS -->
  {:else if resourcesStore.tab === "networks"}
    <div class="tab-toolbar">
      <button class="btn-ghost btn-sm" onclick={() => resourcesStore.loadNetworks()} disabled={resourcesStore.netBusy}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" /><path d="M3 3v5h5" />
        </svg>
        Refresh
      </button>
      {#if resourcesStore.netBusy}<span class="loading-hint">Loading…</span>{/if}
    </div>
    {#if resourcesStore.netError}<div class="err-box">{resourcesStore.netError}</div>{/if}
    {#if resourcesStore.networks.length === 0 && !resourcesStore.netBusy}
      <div class="empty-hint">No networks found</div>
    {:else}
      <table class="res-table">
        <thead><tr><th>Name</th><th>ID</th><th>Driver</th><th>Scope</th><th>Internal</th></tr></thead>
        <tbody>
          {#each resourcesStore.networks as net (net.id)}
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
  {:else if resourcesStore.tab === "system"}
    {#if resourcesStore.sysError}<div class="err-box">{resourcesStore.sysError}</div>
    {:else if resourcesStore.sysLoading}<div class="empty-hint">Loading…</div>
    {:else if resourcesStore.sysInfo}
      <div class="sys-grid">
        <div class="sys-card"><div class="sys-lbl">Docker version</div><div class="sys-val mono">{resourcesStore.sysInfo.docker_version || "—"}</div></div>
        <div class="sys-card"><div class="sys-lbl">API version</div><div class="sys-val mono">{resourcesStore.sysInfo.api_version || "—"}</div></div>
        <div class="sys-card"><div class="sys-lbl">OS</div><div class="sys-val">{resourcesStore.sysInfo.os || "—"}</div></div>
        <div class="sys-card"><div class="sys-lbl">Architecture</div><div class="sys-val mono">{resourcesStore.sysInfo.arch || "—"}</div></div>
        <div class="sys-card"><div class="sys-lbl">Kernel</div><div class="sys-val mono">{resourcesStore.sysInfo.kernel || "—"}</div></div>
        <div class="sys-card"><div class="sys-lbl">CPUs</div><div class="sys-val">{resourcesStore.sysInfo.cpus}</div></div>
        <div class="sys-card"><div class="sys-lbl">Total memory</div><div class="sys-val">{fmtBytes(resourcesStore.sysInfo.total_memory)}</div></div>
        <div class="sys-card green"><div class="sys-lbl">Running containers</div><div class="sys-val">{resourcesStore.sysInfo.containers_running}</div></div>
        <div class="sys-card red"><div class="sys-lbl">Stopped containers</div><div class="sys-val">{resourcesStore.sysInfo.containers_stopped}</div></div>
        <div class="sys-card"><div class="sys-lbl">Images</div><div class="sys-val">{resourcesStore.sysInfo.images}</div></div>
      </div>
      <div class="tab-toolbar" style="margin-top: 16px;">
        <button class="btn-ghost btn-sm" onclick={() => resourcesStore.loadSystem()}>Refresh</button>
      </div>
    {:else}
      <div class="empty-hint">Click refresh to load system info</div>
      <div class="tab-toolbar" style="margin-top: 8px;">
        <button class="btn-ghost btn-sm" onclick={() => resourcesStore.loadSystem()}>Load system info</button>
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
