<script lang="ts">
  import { getAppState } from '../lib/appContext';

  const app = getAppState();
</script>

{#if app.k8sLoading}
  <div class="state-msg">Loading Kubernetes…</div>
{:else if app.k8sError}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z"/><path d="M12 22V12M20 6.5L12 12M4 6.5L12 12"/></svg>
    <p>Could not load Kubernetes</p>
    <span class="empty-hint">{app.k8sError}</span>
  </div>
{:else}
  <div class="kpi-row">
    <div class="kpi-card">
      <div class="kpi-top"><span class="kpi-lbl">Pods</span><div class="kpi-icon" style="background:var(--accent-tint)"><svg viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2"><path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z"/></svg></div></div>
      <div class="kpi-val">{app.pods.length}</div>
      <div class="kpi-trend" style="color:var(--mist)">cluster pods</div>
    </div>
    <div class="kpi-card">
      <div class="kpi-top"><span class="kpi-lbl">Ready</span><div class="kpi-icon" style="background:var(--green-tint)"><svg viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2"><path d="M5 13l4 4 10-10"/></svg></div></div>
      <div class="kpi-val">{app.pods.filter(p => p.ready).length}</div>
      <div class="kpi-trend" style="color:var(--green)">ready pods</div>
    </div>
    <div class="kpi-card">
      <div class="kpi-top"><span class="kpi-lbl">Namespaces</span><div class="kpi-icon" style="background:var(--amber-tint)"><svg viewBox="0 0 24 24" fill="none" stroke="var(--amber)" stroke-width="2"><rect x="3" y="3" width="7" height="7" rx="1.4"/><rect x="14" y="3" width="7" height="7" rx="1.4"/><rect x="3" y="14" width="7" height="7" rx="1.4"/><rect x="14" y="14" width="7" height="7" rx="1.4"/></svg></div></div>
      <div class="kpi-val">{app.namespaces.length}</div>
      <div class="kpi-trend" style="color:var(--mist)">namespaces</div>
    </div>
  </div>

  <div class="k8s-grid">
    <div class="table-wrap">
      <table>
        <thead><tr><th>Pod</th><th>Namespace</th><th>Status</th><th>Ready</th></tr></thead>
        <tbody>
          {#each app.pods as pod (pod.name)}
            <tr>
              <td class="cell-mono">{pod.name}</td>
              <td class="cell-mono cell-mist">{pod.namespace}</td>
              <td><span class="pill pill-mist">{pod.status}</span></td>
              <td><span class="pill {pod.ready ? 'pill-green' : 'pill-red'}">{pod.ready ? 'Ready' : 'Not ready'}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if app.pods.length === 0}
        <div class="empty-state small"><p>No pods found</p></div>
      {/if}
    </div>

    <div class="table-wrap">
      <table>
        <thead><tr><th>Namespace</th><th>Status</th></tr></thead>
        <tbody>
          {#each app.namespaces as ns (ns.name)}
            <tr>
              <td class="cell-mono">{ns.name}</td>
              <td><span class="pill pill-mist">{ns.status}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if app.namespaces.length === 0}
        <div class="empty-state small"><p>No namespaces found</p></div>
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
