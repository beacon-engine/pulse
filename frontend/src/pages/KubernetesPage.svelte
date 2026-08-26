<script lang="ts">
  import { onMount } from "svelte";
  import { kubernetesStore } from "../stores/kubernetes.svelte";
  import type { K8sPod } from "../types/containers";

  onMount(() => {
    kubernetesStore.startPolling(15000);
    return () => kubernetesStore.stopPolling();
  });

  let expanded = $state<Record<string, boolean>>({});
  let deleting = $state<Record<string, boolean>>({});

  function key(p: K8sPod) {
    return `${p.namespace}/${p.name}`;
  }

  function toggle(p: K8sPod) {
    const k = key(p);
    expanded = { ...expanded, [k]: !expanded[k] };
  }

  function containerColor(s: string) {
    if (s === "running") return "var(--green)";
    if (s === "terminated") return "var(--red)";
    return "var(--amber)";
  }

  async function deletePod(p: K8sPod) {
    if (
      !confirm(
        `Delete pod "${p.name}" in ${p.namespace}? Its controller (if any) will recreate it.`
      )
    )
      return;
    const k = key(p);
    deleting = { ...deleting, [k]: true };
    try {
      await kubernetesStore.deletePod(p.namespace, p.name);
    } catch (e: any) {
      // Error is already toasted by store
    } finally {
      deleting = { ...deleting, [k]: false };
    }
  }
</script>

{#if kubernetesStore.loading && kubernetesStore.pods.length === 0}
  <div class="state-msg">Loading Kubernetes…</div>
{:else if kubernetesStore.error && kubernetesStore.pods.length === 0}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
      <path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z" />
      <path d="M12 22V12M20 6.5L12 12M4 6.5L12 12" />
    </svg>
    <p>Could not load Kubernetes</p>
    <span class="empty-hint">{kubernetesStore.error}</span>
    <button class="btn-ghost" onclick={() => kubernetesStore.refresh()}>Retry</button>
  </div>
{:else}
  <div class="kpi-row">
    <div class="kpi-card">
      <div class="kpi-top">
        <span class="kpi-lbl">Pods</span>
        <div class="kpi-icon" style="background:var(--accent-tint)">
          <svg viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2">
            <path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z" />
          </svg>
        </div>
      </div>
      <div class="kpi-val">{kubernetesStore.pods.length}</div>
      <div class="kpi-trend" style="color:var(--mist)">cluster pods</div>
    </div>
    <div class="kpi-card">
      <div class="kpi-top">
        <span class="kpi-lbl">Ready</span>
        <div class="kpi-icon" style="background:var(--green-tint)">
          <svg viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2">
            <path d="M5 13l4 4 10-10" />
          </svg>
        </div>
      </div>
      <div class="kpi-val">{kubernetesStore.pods.filter((p) => p.ready).length}</div>
      <div class="kpi-trend" style="color:var(--green)">ready pods</div>
    </div>
    <div class="kpi-card">
      <div class="kpi-top">
        <span class="kpi-lbl">Namespaces</span>
        <div class="kpi-icon" style="background:var(--amber-tint)">
          <svg viewBox="0 0 24 24" fill="none" stroke="var(--amber)" stroke-width="2">
            <rect x="3" y="3" width="7" height="7" rx="1.4" />
            <rect x="14" y="3" width="7" height="7" rx="1.4" />
            <rect x="3" y="14" width="7" height="7" rx="1.4" />
            <rect x="14" y="14" width="7" height="7" rx="1.4" />
          </svg>
        </div>
      </div>
      <div class="kpi-val">{kubernetesStore.namespaces.length}</div>
      <div class="kpi-trend" style="color:var(--mist)">namespaces</div>
    </div>
  </div>

  <div class="toolbar">
    <button class="btn-ghost btn-sm" onclick={() => kubernetesStore.refresh()}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" />
        <path d="M3 3v5h5" />
      </svg>
      Refresh
    </button>
  </div>

  <div class="k8s-grid">
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Pod</th>
            <th>Namespace</th>
            <th>Status</th>
            <th>Ready</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each kubernetesStore.pods as pod (pod.namespace + "/" + pod.name)}
            <tr class:row-open={expanded[key(pod)]}>
              <td class="cell-mono">
                <button class="pod-toggle" type="button" onclick={() => toggle(pod)}>
                  <svg
                    class="chev"
                    class:open={expanded[key(pod)]}
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path d="M9 6l6 6-6 6" />
                  </svg>
                  {pod.name}
                  <span class="ct-badge">{pod.containers.length}</span>
                </button>
              </td>
              <td class="cell-mono cell-mist">{pod.namespace}</td>
              <td><span class="pill pill-mist">{pod.status}</span></td>
              <td>
                <span class="pill {pod.ready ? 'pill-green' : 'pill-red'}">
                  {pod.ready ? "Ready" : "Not ready"}
                </span>
              </td>
              <td class="ta-r">
                <button
                  class="del-btn"
                  title="Delete pod"
                  disabled={deleting[key(pod)]}
                  onclick={() => deletePod(pod)}
                >
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="3 6 5 6 21 6" />
                    <path d="M19 6l-1 14H6L5 6" />
                  </svg>
                </button>
              </td>
            </tr>
            {#if expanded[key(pod)]}
              <tr class="sub-tr">
                <td colspan="5">
                  {#if pod.containers.length === 0}
                    <div class="sub-msg">No container statuses reported</div>
                  {:else}
                    <div class="sub-list">
                      {#each pod.containers as c (c.name)}
                        <div class="sub-row">
                          <span class="sub-dot" style="background:{containerColor(c.state)}"></span>
                          <span class="sub-name">{c.name}</span>
                          <span class="sub-img">{c.image}</span>
                          <span
                            class="pill {c.state === 'running'
                              ? 'pill-green'
                              : c.state === 'terminated'
                                ? 'pill-red'
                                : 'pill-mist'}"
                          >
                            {c.state}
                          </span>
                          <span class="sub-meta">
                            {c.ready ? "ready" : "not ready"} · {c.restart_count} restart{c.restart_count !== 1 ? "s" : ""}
                          </span>
                        </div>
                      {/each}
                    </div>
                  {/if}
                </td>
              </tr>
            {/if}
          {/each}
        </tbody>
      </table>
      {#if kubernetesStore.pods.length === 0}
        <div class="empty-state small"><p>No pods found</p></div>
      {/if}
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>Namespace</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {#each kubernetesStore.namespaces as ns (ns.name)}
            <tr>
              <td class="cell-mono">{ns.name}</td>
              <td><span class="pill pill-mist">{ns.status}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
      {#if kubernetesStore.namespaces.length === 0}
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
  .toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px; padding: 7px 12px;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600; cursor: pointer;
  }
  .btn-ghost:hover { background: var(--surface); border-color: var(--border-strong); }
  .btn-ghost.btn-sm { padding: 5px 10px; font-size: 12px; }
  .btn-ghost :global(svg) { width: 13px; height: 13px; }
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
  .ta-r { text-align: right; }
  .pill { display: inline-block; padding: 2px 8px; border-radius: 20px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; }
  .pill-green { background: var(--green-tint); color: var(--green); }
  .pill-red { background: var(--red-tint); color: var(--red); }
  .pill-mist { background: var(--surface-2); color: var(--mist); }

  .pod-toggle {
    display: inline-flex; align-items: center; gap: 7px; background: none; border: none;
    padding: 0; margin: 0; cursor: pointer; font-family: 'JetBrains Mono', monospace;
    font-size: 12px; font-weight: 700; color: var(--ink);
  }
  .pod-toggle:hover { color: var(--accent); }
  .chev { width: 13px; height: 13px; color: var(--mist-2); transition: transform .15s ease; flex-shrink: 0; }
  .chev.open { transform: rotate(90deg); }
  .ct-badge { font-size: 10px; font-weight: 700; color: var(--mist-2); background: var(--surface-2); padding: 1px 6px; border-radius: 20px; }
  :global(tr.row-open td) { background: var(--surface-2); }
  .del-btn {
    width: 28px; height: 28px; border-radius: 6px; border: none; background: transparent;
    color: var(--mist-2); display: inline-flex; align-items: center; justify-content: center;
    cursor: pointer; opacity: 0; transition: all .12s;
  }
  tr:hover .del-btn { opacity: 1; }
  .del-btn:hover:not(:disabled) { background: var(--red-tint); color: var(--red); }
  .del-btn:disabled { opacity: .3; cursor: not-allowed; }
  .del-btn :global(svg) { width: 13px; height: 13px; }

  .sub-tr :global(td) { padding: 0 18px 12px !important; background: var(--surface-2); }
  .sub-list { display: flex; flex-direction: column; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); overflow: hidden; }
  .sub-row { display: flex; align-items: center; gap: 12px; padding: 9px 12px; border-bottom: 1px solid var(--border); }
  .sub-row:last-child { border-bottom: none; }
  .sub-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
  .sub-name { font-weight: 600; font-size: 12.5px; min-width: 120px; }
  .sub-img { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sub-meta { font-size: 11.5px; color: var(--mist-2); white-space: nowrap; }
  .sub-msg { font-size: 12.5px; color: var(--mist); padding: 8px 2px; }

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
