<script lang="ts">
  import ContainerRow from "../components/ContainerRow.svelte";
  import { ContainerFilterStatus } from "../types/containers";
  import { containersStore } from "../stores/container.svelte";
  import { hostsStore } from "../stores/hosts.svelte";
    import { untrack } from "svelte";

  // Only restart polling when hostId ACTUALLY changes, ignoring internal state updates
    $effect(() => {
      const hostId = hostsStore.activeHostId;
      if (hostId) {
        untrack(() => {
          containersStore.startPolling(15000);
        });
      }
      return () => {
        containersStore.stopPolling();
      };
    });

  const PER_PAGE = 20;
  const STATUS_OPTIONS = [
    ContainerFilterStatus.All,
    ContainerFilterStatus.Running,
    ContainerFilterStatus.Stopped,
  ] as const;

  let page = $state(1);
  let filterText = $state("");
  let filterStatus = $state<ContainerFilterStatus>(ContainerFilterStatus.All);

  let filtered = $derived(
    containersStore.containers
      .filter(
        (c) =>
          filterStatus === ContainerFilterStatus.All ||
          (filterStatus === ContainerFilterStatus.Running
            ? c.state === "running"
            : c.state !== "running")
      )
      .filter(
        (c) =>
          !filterText ||
          c.name.toLowerCase().includes(filterText.toLowerCase()) ||
          c.image.toLowerCase().includes(filterText.toLowerCase())
      )
  );

  let totalPages = $derived(
    Math.max(1, Math.ceil(filtered.length / PER_PAGE))
  );

  let paged = $derived(
    filtered.slice((page - 1) * PER_PAGE, page * PER_PAGE)
  );

  let pageNumbers = $derived(
    Array.from({ length: totalPages }, (_, i) => i + 1).filter(
      (p) => p === 1 || p === totalPages || Math.abs(p - page) <= 1
    )
  );

  let running = $derived(
    containersStore.containers.filter((c) => c.state === "running").length
  );
  let stopped = $derived(
    containersStore.containers.filter((c) => c.state === "exited").length
  );
</script>

<div class="kpi-row">
  <div class="kpi-card">
    <div class="kpi-top">
      <span class="kpi-lbl">Running</span>
      <div class="kpi-icon" style="background:var(--green-tint)">
        <svg viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2">
          <path d="M5 12l4 4 10-10" />
        </svg>
      </div>
    </div>
    <div class="kpi-val">
      {running}<span class="kpi-of"> / {containersStore.containers.length}</span>
    </div>
    <div class="kpi-trend" style="color:var(--green)">containers active</div>
  </div>

  <div class="kpi-card">
    <div class="kpi-top">
      <span class="kpi-lbl">Stopped</span>
      <div class="kpi-icon" style="background:var(--red-tint)">
        <svg viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="2">
          <rect x="6" y="6" width="12" height="12" rx="2" />
        </svg>
      </div>
    </div>
    <div class="kpi-val">{stopped}</div>
    <div class="kpi-trend" style="color:var(--mist)">not running</div>
  </div>

  <div class="kpi-card">
    <div class="kpi-top">
      <span class="kpi-lbl">Total</span>
      <div class="kpi-icon" style="background:var(--accent-tint)">
        <svg viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2">
          <path d="M12 2l9 4.5-9 4.5-9-4.5L12 2zM3 12l9 4.5 9-4.5M3 17l9 4.5 9-4.5" />
        </svg>
      </div>
    </div>
    <div class="kpi-val">{containersStore.containers.length}</div>
    <div class="kpi-trend" style="color:var(--mist)">
      on {hostsStore.activeHost?.name ?? "this host"}
    </div>
  </div>

  <div class="kpi-card">
    <div class="kpi-top">
      <span class="kpi-lbl">Stacks</span>
      <div class="kpi-icon" style="background:var(--amber-tint)">
        <svg viewBox="0 0 24 24" fill="none" stroke="var(--amber)" stroke-width="2">
          <rect x="3" y="3" width="7" height="7" rx="1.4" />
          <rect x="14" y="3" width="7" height="7" rx="1.4" />
          <rect x="3" y="14" width="7" height="7" rx="1.4" />
          <rect x="14" y="14" width="7" height="7" rx="1.4" />
        </svg>
      </div>
    </div>
    <div class="kpi-val">{containersStore.stacks.length}</div>
    <div class="kpi-trend" style="color:var(--mist)">compose projects</div>
  </div>
</div>

{#if containersStore.loading && containersStore.containers.length === 0}
  <div class="state-msg">Connecting to {hostsStore.activeHost?.name ?? "host"}…</div>
{:else if hostsStore.activeHostId === null}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
      <rect x="3" y="7" width="18" height="13" rx="2" />
      <path d="M3 11h18" />
    </svg>
    <p>No host selected — add one in the sidebar</p>
  </div>
{:else if containersStore.containers.length === 0}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
      <rect x="3" y="7" width="18" height="13" rx="2" />
      <path d="M3 11h18M8 7V4.5A1.5 1.5 0 0 1 9.5 3h5A1.5 1.5 0 0 1 16 4.5V7" />
    </svg>
    <p>No containers on <strong>{hostsStore.activeHost?.name}</strong></p>
  </div>
{:else}
  <div class="table-wrap">
    <div class="table-toolbar">
      <input
        class="filter-input"
        placeholder="Filter by name or image…"
        bind:value={filterText}
      />
      <div class="seg">
        {#each STATUS_OPTIONS as s}
          <button
            class:active={filterStatus === s}
            onclick={() => {
              filterStatus = s;
              page = 1;
            }}
          >
            {s.charAt(0).toUpperCase() + s.slice(1)}
          </button>
        {/each}
      </div>
    </div>
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Status</th>
          <th>Stack</th>
          <th>Ports</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each paged as c (c.id)}
          <ContainerRow
            container={c}
            onstart={() => containersStore.startContainer(c.id)}
            onstop={() => containersStore.stopContainer(c.id)}
            onrestart={() => containersStore.restartContainer(c.id)}
            onlogs={() => containersStore.selectContainer(c)}
            oninspect={() => containersStore.selectContainer(c)}
          />
        {/each}
      </tbody>
    </table>
    {#if totalPages > 1}
      <div class="pagination">
        <span class="page-info">
          {(page - 1) * PER_PAGE + 1}–{Math.min(
            page * PER_PAGE,
            filtered.length
          )} of {filtered.length}
        </span>
        <div class="page-btns">
          <button onclick={() => (page -= 1)} disabled={page === 1}>←</button>
          {#each pageNumbers as p, i}
            {#if i > 0 && pageNumbers[i - 1] !== p - 1}
              <span class="ellipsis">…</span>
            {/if}
            <button
              class:current={p === page}
              onclick={() => (page = p)}>{p}</button>
          {/each}
          <button
            onclick={() => (page += 1)}
            disabled={page === totalPages}>→</button>
        </div>
      </div>
    {/if}
  </div>
{/if}

<style>
    .kpi-row {
        display: grid;
        grid-template-columns: repeat(4, minmax(0, 1fr));
        gap: 16px;
        margin-bottom: 22px;
    }
    .kpi-card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        padding: 16px;
        box-shadow: var(--shadow-sm);
    }
    .kpi-top {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 8px;
    }
    .kpi-lbl {
        font-size: 12px;
        font-weight: 700;
        color: var(--mist-2);
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }
    .kpi-icon {
        width: 32px;
        height: 32px;
        border-radius: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
    }
    .kpi-icon :global(svg) {
        width: 16px;
        height: 16px;
    }
    .kpi-val {
        font-size: 26px;
        font-weight: 800;
        color: var(--ink);
        line-height: 1;
    }
    .kpi-of {
        font-size: 14px;
        font-weight: 600;
        color: var(--mist);
    }
    .kpi-trend {
        font-size: 12px;
        margin-top: 6px;
    }

    .state-msg {
        color: var(--mist);
        font-size: 14px;
        padding: 60px 0;
        text-align: center;
    }
    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 10px;
        padding: 80px 0;
        color: var(--mist);
    }
    .empty-state :global(svg) {
        width: 38px;
        height: 38px;
        color: var(--mist-2);
        opacity: 0.6;
    }
    .empty-state p {
        font-size: 14px;
        margin: 0;
    }
    .empty-state strong {
        color: var(--ink);
    }

    .table-wrap {
        overflow-x: auto;
    }
    .table-toolbar {
        display: flex;
        gap: 10px;
        align-items: center;
        margin-bottom: 12px;
        flex-wrap: wrap;
        justify-content: space-between;
    }
    .filter-input {
        padding: 7px 12px;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        background: var(--surface);
        color: var(--ink);
        font-size: 13px;
        min-width: 220px;
    }
    .seg {
        display: inline-flex;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        overflow: hidden;
    }
    .seg button {
        padding: 6px 12px;
        border: none;
        background: var(--surface);
        color: var(--mist);
        font-size: 12px;
        font-weight: 650;
        cursor: pointer;
    }
    .seg button.active {
        background: var(--accent);
        color: #fff;
    }
    .seg button:not(:last-child) {
        border-right: 1px solid var(--border);
    }

    table {
        width: 100%;
        border-collapse: collapse;
    }
    thead th {
        text-align: left;
        padding: 10px 18px;
        font-size: 11.5px;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--mist-2);
        font-weight: 700;
        border-bottom: 1px solid var(--border);
        background: var(--surface-2);
        white-space: nowrap;
    }
    :global(tbody td) {
        padding: 11px 18px;
        border-bottom: 1px solid var(--border);
        font-size: 13.3px;
        vertical-align: middle;
    }
    :global(tbody tr:last-child td) {
        border-bottom: none;
    }
    :global(tbody tr:hover td) {
        background: var(--surface-2);
    }

    .pagination {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 14px 18px;
        border-top: 1px solid var(--border);
        background: var(--surface-2);
    }
    .page-info {
        font-size: 12px;
        color: var(--mist);
    }
    .page-btns {
        display: flex;
        gap: 6px;
        align-items: center;
    }
    .page-btns button {
        min-width: 28px;
        height: 28px;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        background: var(--surface);
        color: var(--ink);
        font-size: 12px;
        font-weight: 600;
        cursor: pointer;
        transition: all 0.12s;
    }
    .page-btns button:hover:not(:disabled) {
        background: var(--surface-2);
        border-color: var(--border-strong);
    }
    .page-btns button:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }
    .page-btns button.current {
        background: var(--accent);
        color: #fff;
        border-color: var(--accent);
    }
    .ellipsis {
        color: var(--mist);
        font-size: 12px;
        padding: 0 2px;
    }
</style>
