<script lang="ts">
  import { getAppState } from '../lib/appContext';

  const app = getAppState();
</script>

{#if app.swarmLoading}
  <div class="state-msg">Loading Swarm…</div>
{:else if app.swarmError}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="5" r="2"/><circle cx="19" cy="9" r="2"/><circle cx="19" cy="15" r="2"/><circle cx="12" cy="19" r="2"/><circle cx="5" cy="15" r="2"/><circle cx="5" cy="9" r="2"/><path d="M7 9l4-2.5M13 9l4 2.5M17 15l-4 2.5M11 15l-4-2.5"/></svg>
    <p>Could not load Swarm</p>
    <span class="empty-hint">{app.swarmError}</span>
  </div>
{:else}
  <div class="kpi-row">
    <div class="kpi-card">
      <div class="kpi-top"><span class="kpi-lbl">Services</span><div class="kpi-icon" style="background:var(--accent-tint)"><svg viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2"><path d="M4 7h16M4 12h16M4 17h16"/></svg></div></div>
      <div class="kpi-val">{app.swarmServices.length}</div>
      <div class="kpi-trend" style="color:var(--mist)">swarm services</div>
    </div>
    <div class="kpi-card">
      <div class="kpi-top"><span class="kpi-lbl">Running</span><div class="kpi-icon" style="background:var(--green-tint)"><svg viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2"><path d="M5 13l4 4 10-10"/></svg></div></div>
      <div class="kpi-val">{app.swarmServices.reduce((a, s) => a + s.running, 0)}</div>
      <div class="kpi-trend" style="color:var(--green)">running replicas</div>
    </div>
    <div class="kpi-card">
      <div class="kpi-top"><span class="kpi-lbl">Nodes</span><div class="kpi-icon" style="background:var(--amber-tint)"><svg viewBox="0 0 24 24" fill="none" stroke="var(--amber)" stroke-width="2"><circle cx="12" cy="5" r="2"/><circle cx="19" cy="9" r="2"/><circle cx="5" cy="9" r="2"/></svg></div></div>
      <div class="kpi-val">{app.swarmNodes.length}</div>
      <div class="kpi-trend" style="color:var(--mist)">swarm nodes</div>
    </div>
  </div>

  <div class="k8s-grid">
    <div class="table-wrap">
      <table>
        <thead><tr><th>Service</th><th>Image</th><th>Replicas</th><th>State</th></tr></thead>
        <tbody>
          {#each app.swarmServices as svc (svc.id)}
            <tr>
              <td class="cell-mono">{svc.name}</td>
              <td class="cell-mono cell-mist">{svc.image}</td>
              <td class="cell-mono">{svc.running}/{svc.replicas}</td>
              <td><span class="pill pill-mist">{svc.desired_state}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if app.swarmServices.length === 0}
        <div class="empty-state small"><p>No services found</p></div>
      {/if}
    </div>

    <div class="table-wrap">
      <table>
        <thead><tr><th>Node</th><th>Role</th><th>State</th><th>Availability</th></tr></thead>
        <tbody>
          {#each app.swarmNodes as node (node.id)}
            <tr>
              <td class="cell-mono">{node.hostname}</td>
              <td class="cell-mono cell-mist">{node.role}</td>
              <td><span class="pill pill-mist">{node.state}</span></td>
              <td><span class="pill {node.availability === 'active' ? 'pill-green' : 'pill-red'}">{node.availability}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if app.swarmNodes.length === 0}
        <div class="empty-state small"><p>No nodes found</p></div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .state-msg { color: var(--mist); font-size: 14px; padding: 60px 0; text-align: center; }
  .empty-state {
    display: flex; flex-direction: column; align-items: center; gap: 10px;
    padding: 80px 0; color: var(--mist);
  }
  .empty-state :global(svg) { width: 38px; height: 38px; color: var(--mist-2); opacity: .6; }
  .empty-state p { font-size: 14px; margin: 0; }
  .empty-state.small { padding: 40px 0; }
  .empty-state.small p { font-size: 13px; }
  .empty-hint { font-size: 12.5px; color: var(--mist-2); }
  .table-wrap { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  thead th {
    text-align: left; padding: 10px 18px; font-size: 11.5px; text-transform: uppercase;
    letter-spacing: .05em; color: var(--mist-2); font-weight: 700;
    border-bottom: 1px solid var(--border); background: var(--surface-2); white-space: nowrap;
  }
  :global(tbody td) {
    padding: 11px 18px; border-bottom: 1px solid var(--border);
    font-size: 13.3px; vertical-align: middle;
  }
  :global(tbody tr:last-child td) { border-bottom: none; }
  :global(tbody tr:hover td) { background: var(--surface-2); }
  .cell-mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .cell-mist { color: var(--mist); }
  .pill { display: inline-block; padding: 2px 8px; border-radius: 20px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; }
  .pill-green { background: var(--green-tint); color: var(--green); }
  .pill-red { background: var(--red-tint); color: var(--red); }
  .pill-mist { background: var(--surface-2); color: var(--mist); }

  .kpi-row { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; margin-bottom: 22px; }
  .kpi-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); padding: 16px; box-shadow: var(--shadow-sm); }
  .kpi-top { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
  .kpi-lbl { font-size: 12px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .04em; }
  .kpi-icon { width: 32px; height: 32px; border-radius: 8px; display: flex; align-items: center; justify-content: center; }
  .kpi-icon :global(svg) { width: 16px; height: 16px; }
  .kpi-val { font-size: 26px; font-weight: 800; color: var(--ink); line-height: 1; }
  .kpi-trend { font-size: 12px; margin-top: 6px; }

  .k8s-grid { display: grid; grid-template-columns: 2fr 1fr; gap: 20px; }
  .k8s-grid .table-wrap { min-height: 200px; }
</style>
