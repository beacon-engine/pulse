<script lang="ts">
  import type { K8sPod, PodContainer } from '../types/containers';
  import { containerPorts, containerStatus } from '../utils/k8s';

  // One container of a pod, laid out like ContainerRow on the Containers page.
  let {
    pod,
    container,
    showPod = true,
    onlogs,
    oninspect,
    onrestart,
  }: {
    pod: K8sPod;
    container: PodContainer;
    /** Hide the Pod column when the row sits under its pod already. */
    showPod?: boolean;
    onlogs: () => void;
    oninspect: () => void;
    /** Deletes the pod so its controller starts a fresh one. */
    onrestart: () => Promise<void> | void;
  } = $props();

  let busy = $state(false);
  let status = $derived(containerStatus(container));
  // Kubernetes restarts containers itself; Pulse can only replace the whole
  // pod, and only when a controller will recreate it.
  let restartBlock = $derived(
    pod.protected ? `Protected: ${pod.protected_reason}` :
    !pod.owner ? 'This pod has no controller, so deleting it would not bring it back' :
    ''
  );

  async function restart() {
    busy = true;
    try { await onrestart(); } finally { busy = false; }
  }
</script>

<tr class="row" class:busy>
  <td>
    <button type="button" class="name-cell" onclick={oninspect}>
      <div class="row-name">
        {container.name}
        {#if pod.protected}<span class="pulse-tag" title={pod.protected_reason}>Pulse</span>{/if}
      </div>
      <div class="row-sub" title={container.image}>{container.image}</div>
    </button>
  </td>

  <td>
    <span class="pill {status.pill}" title={container.message || undefined}>
      {#if status.live}<span class="dot"></span>{/if}
      {status.label}
    </span>
  </td>

  {#if showPod}
    <td>
      <span class="pill pill-violet pod-pill" title={`${pod.namespace}/${pod.name}`}>{pod.name}</span>
      <div class="ns-sub">{pod.namespace}</div>
    </td>
  {/if}

  <td class="cell-mono cell-mist">{containerPorts(container)}</td>

  <td>
    <span class="restarts" class:hot={container.restart_count > 0}
      title={container.last_reason ? `Last exit: ${container.last_reason} (${container.last_exit_code ?? '?'})` : undefined}>
      {container.restart_count}
    </span>
  </td>

  <td>
    <div class="row-actions">
      <button class="act-btn" title={restartBlock || 'Restart (replace the pod)'} disabled={busy || !!restartBlock}
        onclick={restart}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
      </button>
      <button class="act-btn" title="Logs" disabled={busy} onclick={onlogs}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M6 9h12M6 13h9M6 17h6"/></svg>
      </button>
      <button class="act-btn" title="Inspect" disabled={busy} onclick={oninspect}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
      </button>
    </div>
  </td>
</tr>

<style>
  .row { transition: background .1s; }
  .row.busy { opacity: .6; pointer-events: none; }

  .name-cell { cursor: pointer; display: block; border: none; background: none; padding: 0; text-align: left; color: inherit; font: inherit; }
  .row-name { font-weight: 650; font-size: 13.3px; color: var(--ink); transition: color .12s; }
  .row-sub  { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist-2); margin-top: 2px; max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name-cell:hover .row-name { color: var(--accent); }

  .pod-pill { max-width: 240px; overflow: hidden; text-overflow: ellipsis; display: inline-block; padding: 3px 9px; }
  .ns-sub { font-size: 11px; color: var(--mist-2); margin: 2px 0 0 2px; }
  .cell-mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .cell-mist { color: var(--mist-2); }
  .restarts { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--mist-2); }
  .restarts.hot { color: var(--amber); font-weight: 700; }

  .row-actions { display: flex; gap: 5px; justify-content: flex-end; opacity: 0; transition: opacity .12s; }
  .row:hover .row-actions { opacity: 1; }

  .act-btn {
    width: 30px; height: 30px; border-radius: 8px; border: 1px solid var(--border);
    background: var(--surface); display: flex; align-items: center; justify-content: center;
    color: var(--mist); transition: background .12s, color .12s, border-color .12s;
    flex-shrink: 0;
  }
  .act-btn svg { width: 14px; height: 14px; }
  .act-btn:hover:not(:disabled) { background: var(--surface-2); color: var(--ink); border-color: var(--border-strong); }
  .act-btn:disabled { opacity: .3; cursor: not-allowed; }
  .pulse-tag {
    margin-left: 6px; font-size: 10px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase;
    color: var(--accent-ink); background: var(--accent-tint); padding: 1px 6px; border-radius: 20px;
    vertical-align: 1px;
  }
  @media (hover: none) { .row-actions { opacity: 1; } }
</style>
