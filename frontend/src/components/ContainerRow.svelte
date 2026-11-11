<script lang="ts">
  import type { Container } from '../types/containers';

  let {
    container,
    onstart,
    onstop,
    onrestart,
    onlogs,
    oninspect,
  }: {
    container: Container;
    onstart:   (id: string) => void;
    onstop:    (id: string) => void;
    onrestart: (id: string) => void;
    onlogs:    (id: string) => void;
    oninspect: (id: string) => void;
  } = $props();

  let busy = $state(false);

  async function act(fn: () => Promise<void> | void) {
    busy = true;
    try { await fn(); } finally { busy = false; }
  }

  let isRunning = $derived(container.state === 'running');
  let pillClass = $derived(
    container.state === 'running'  ? 'pill pill-green' :
    container.state === 'exited'   ? 'pill pill-red'   :
    container.state === 'paused'   ? 'pill pill-amber'  :
    'pill pill-mist'
  );
  let statusLabel = $derived(
    container.state === 'running'    ? 'Running'    :
    container.state === 'exited'     ? 'Stopped'    :
    container.state === 'paused'     ? 'Paused'     :
    container.state === 'restarting' ? 'Restarting' :
    container.status
  );
</script>

<tr class="row" class:busy>
  <!-- Name + image -->
  <td>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="name-cell" onclick={() => oninspect(container.id)}>
      <div class="row-name">
        {container.name}
        {#if container.protected}<span class="pulse-tag" title={container.protected_reason}>Pulse</span>{/if}
        {#if container.kubernetes}<span class="pulse-tag k8s-tag" title={`Managed by Kubernetes: pod ${container.kubernetes.namespace}/${container.kubernetes.pod}`}>k8s</span>{/if}
      </div>
      <div class="row-sub">{container.image}</div>
    </div>
  </td>

  <!-- Status pill -->
  <td>
    <span class={pillClass}>
      {#if isRunning}<span class="dot"></span>{/if}
      {statusLabel}
    </span>
  </td>

  <!-- Stack -->
  <td>
    {#if container.stack}
      <span class="pill pill-violet">{container.stack}</span>
    {:else}
      <span class="dash">—</span>
    {/if}
  </td>

  <!-- Ports (placeholder — not returned in list, only in inspect) -->
  <td class="cell-mono cell-mist">—</td>

  <!-- Actions (hidden until row hover) -->
  <td>
    <div class="row-actions">
      {#if isRunning}
        <button class="act-btn danger" title={container.protected ? `Protected: ${container.protected_reason}` : 'Stop'} disabled={busy || container.protected}
          onclick={() => act(() => onstop(container.id))}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
        </button>
      {:else}
        <button class="act-btn go" title="Start" disabled={busy}
          onclick={() => act(() => onstart(container.id))}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 4l14 8-14 8V4z"/></svg>
        </button>
      {/if}
      <button class="act-btn" title={container.protected ? `Protected: ${container.protected_reason}` : 'Restart'} disabled={busy || container.protected}
        onclick={() => act(() => onrestart(container.id))}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
      </button>
      <button class="act-btn" title="Logs" disabled={busy}
        onclick={() => onlogs(container.id)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M6 9h12M6 13h9M6 17h6"/></svg>
      </button>
      <button class="act-btn" title="Inspect" disabled={busy}
        onclick={() => oninspect(container.id)}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>
      </button>
    </div>
  </td>
</tr>

<style>
  .row { transition: background .1s; }
  .row.busy { opacity: .6; pointer-events: none; }

  .name-cell { cursor: pointer; }
  .row-name { font-weight: 650; font-size: 13.3px; color: var(--ink); transition: color .12s; }
  .row-sub  { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist-2); margin-top: 2px; max-width: 260px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name-cell:hover .row-name { color: var(--accent); }

  .dash { color: var(--mist-2); font-size: 12px; }
  .cell-mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .cell-mist { color: var(--mist-2); }

  /* Actions: hidden until row hover */
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
  .act-btn.danger:hover:not(:disabled) { background: var(--red-tint);   color: var(--red);   border-color: var(--red); }
  .act-btn.go:hover:not(:disabled)     { background: var(--green-tint); color: var(--green); border-color: var(--green); }
  .pulse-tag {
    margin-left: 6px; font-size: 10px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase;
    color: var(--accent-ink, #4338ca); background: var(--accent-tint, #eef2ff); padding: 1px 6px; border-radius: 20px;
    vertical-align: 1px;
  }
  .k8s-tag { color: var(--blue, #1d4ed8); background: var(--blue-tint, #dbeafe); }
</style>
