<script lang="ts">
  import type { K8sEvent, K8sPod } from '../types/containers';
  import { containerPorts, containerStatus, podReadyCount, podRestarts, podStatus } from '../utils/k8s';
  import { fmtAge } from '../utils/fmt';

  // Inspect a pod: its placement, every container's state, and its recent events.
  let {
    pod,
    focus,
    events,
    onclose,
    onlogs,
    ondelete,
  }: {
    pod: K8sPod;
    /** Container to scroll to and highlight. */
    focus?: string;
    /** The pod's own events, newest first. */
    events: K8sEvent[];
    onclose: () => void;
    onlogs: (container: string) => void;
    ondelete: () => Promise<void> | void;
  } = $props();

  let status = $derived(podStatus(pod));
  let busy = $state(false);
  let deleteBlock = $derived(
    pod.protected ? `Protected: ${pod.protected_reason}` :
    !pod.owner ? 'This pod has no controller, so it would not be recreated' : ''
  );

  async function del() {
    busy = true;
    try { await ondelete(); } finally { busy = false; }
  }

  function when(iso?: string) {
    return iso ? `${new Date(iso).toLocaleString()} (${fmtAge(iso)} ago)` : '—';
  }

  function onkey(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }
</script>

<svelte:window onkeydown={onkey} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="backdrop" role="presentation" onclick={onclose}></div>

<div class="drawer" role="dialog" aria-label={`Pod ${pod.name}`}>
  <div class="dh">
    <div class="dh-left">
      <span class="state-dot {status.pill}"></span>
      <div class="min0">
        <div class="dh-name">{pod.name}</div>
        <div class="dh-id">{pod.namespace} · pod</div>
      </div>
    </div>
    <div class="dh-actions">
      <button class="hbtn hbtn-danger" title={deleteBlock || 'Delete the pod; its controller starts a new one'}
        disabled={busy || !!deleteBlock} onclick={del}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
        Replace pod
      </button>
      <button class="close-x" onclick={onclose} aria-label="Close">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"/></svg>
      </button>
    </div>
  </div>

  <div class="body">
    <div class="kv-list">
      <div class="kv-row"><span class="k">Status</span>
        <span class="v"><span class="pill {status.pill}">{#if status.live}<span class="dot"></span>{/if}{status.label}</span>
          {#if pod.reason && pod.reason !== pod.status}<span class="muted sm"> phase {pod.status}</span>{/if}</span></div>
      <div class="kv-row"><span class="k">Ready containers</span><span class="v">{podReadyCount(pod)} · {podRestarts(pod)} restarts</span></div>
      <div class="kv-row"><span class="k">Controlled by</span><span class="v">{pod.owner ? `${pod.owner.kind} ${pod.owner.name}` : 'None (bare pod)'}</span></div>
      <div class="kv-row"><span class="k">Node</span><span class="v mono">{pod.node ?? 'Not scheduled'}</span></div>
      <div class="kv-row"><span class="k">Pod IP</span><span class="v mono">{pod.ip ?? '—'}</span></div>
      {#if pod.qos_class}<div class="kv-row"><span class="k">QoS class</span><span class="v">{pod.qos_class}</span></div>{/if}
      <div class="kv-row"><span class="k">Created</span><span class="v">{when(pod.created)}</span></div>
      {#if pod.labels && Object.keys(pod.labels).length}
        <div class="kv-row"><span class="k">Labels</span>
          <span class="v chips">{#each Object.entries(pod.labels) as [k, v] (k)}<span class="chip">{k}={v}</span>{/each}</span></div>
      {/if}
    </div>

    <div class="sec-title">Containers <span class="count">{pod.containers.length}</span></div>
    {#each pod.containers as c (c.name)}
      {@const cs = containerStatus(c)}
      <div class="ct-card" class:focus={focus === c.name}>
        <div class="ct-head">
          <div class="min0">
            <div class="ct-name">{c.name}</div>
            <div class="ct-img">{c.image}</div>
          </div>
          <span class="pill {cs.pill}">{#if cs.live}<span class="dot"></span>{/if}{cs.label}</span>
          <button class="hbtn" onclick={() => onlogs(c.name)}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M6 9h12M6 13h9M6 17h6"/></svg>
            Logs
          </button>
        </div>
        {#if c.message}<div class="ct-msg">{c.message}</div>{/if}
        <div class="ct-grid">
          <span class="k">{c.state === 'terminated' ? 'Finished' : 'Started'}</span><span>{c.started ? fmtAge(c.started) + ' ago' : '—'}</span>
          <span class="k">Restarts</span><span>{c.restart_count}{c.last_reason ? ` · last exit ${c.last_reason} (${c.last_exit_code ?? '?'})` : ''}</span>
          <span class="k">Ready</span><span>{c.ready ? 'Yes' : 'No'}</span>
          <span class="k">Ports</span><span class="mono">{containerPorts(c)}</span>
          <span class="k">CPU</span><span class="mono">{c.resources?.cpu_request ?? '—'} request · {c.resources?.cpu_limit ?? 'no'} limit</span>
          <span class="k">Memory</span><span class="mono">{c.resources?.memory_request ?? '—'} request · {c.resources?.memory_limit ?? 'no'} limit</span>
          {#if c.container_id}<span class="k">Runtime ID</span><span class="mono wrap">{c.container_id}</span>{/if}
        </div>
      </div>
    {/each}

    <div class="sec-title">Events <span class="count">{events.length}</span></div>
    {#if events.length === 0}
      <div class="muted sm">No recent events for this pod.</div>
    {:else}
      <div class="ev-list">
        {#each events as e, i (i)}
          <div class="ev-row">
            <span class="pill {e.type === 'Warning' ? 'pill-red' : 'pill-mist'}">{e.reason}</span>
            <span class="ev-msg">{e.message}</span>
            <span class="muted sm nowrap">{e.count > 1 ? `×${e.count} · ` : ''}{fmtAge(e.last_seen)}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .backdrop { position: fixed; inset: 0; background: rgba(20,22,40,.38); z-index: 60; backdrop-filter: blur(1px); }
  .drawer {
    position: fixed; top: 0; right: 0; bottom: 0; width: 680px; max-width: 100vw;
    background: var(--surface); border-left: 1px solid var(--border);
    box-shadow: var(--shadow-lg); z-index: 61; display: flex; flex-direction: column; overflow: hidden;
  }
  .dh {
    display: flex; justify-content: space-between; align-items: center; gap: 12px;
    padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0;
  }
  .dh-left { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .min0 { min-width: 0; }
  .state-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; background: var(--mist-2); }
  .state-dot.pill-green { background: var(--green); }
  .state-dot.pill-red { background: var(--red); }
  .state-dot.pill-amber { background: var(--amber); }
  .dh-name { font-weight: 700; font-size: 15px; color: var(--ink); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dh-id { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist-2); margin-top: 2px; }
  .dh-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
  .hbtn {
    display: inline-flex; align-items: center; gap: 5px; padding: 5px 11px;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--mist); font-size: 12.5px; font-weight: 600; transition: all .12s;
  }
  .hbtn svg { width: 13px; height: 13px; }
  .hbtn:hover:not(:disabled) { background: var(--surface); color: var(--ink); border-color: var(--border-strong); }
  .hbtn:disabled { opacity: .35; cursor: not-allowed; }
  .hbtn-danger:hover:not(:disabled) { background: var(--red-tint); color: var(--red); border-color: var(--red); }
  .close-x {
    width: 30px; height: 30px; border-radius: 8px; border: none; background: var(--surface-2);
    display: flex; align-items: center; justify-content: center; color: var(--mist);
  }
  .close-x:hover { background: var(--border); color: var(--ink); }
  .close-x svg { width: 14px; height: 14px; }

  .body { flex: 1; overflow-y: auto; padding: 8px 20px 28px; }
  .kv-list { display: flex; flex-direction: column; }
  .kv-row { display: flex; gap: 16px; padding: 10px 0; border-bottom: 1px solid var(--border); align-items: flex-start; }
  .k { font-size: 12px; font-weight: 700; color: var(--mist); min-width: 130px; padding-top: 2px; }
  .v { font-size: 13px; color: var(--ink); flex: 1; min-width: 0; }
  .mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .muted { color: var(--mist-2); }
  .sm { font-size: 12px; }
  .nowrap { white-space: nowrap; }
  .wrap { word-break: break-all; }
  .chips { display: flex; flex-wrap: wrap; gap: 5px; }
  .chip {
    font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--mist);
    background: var(--surface-2); border: 1px solid var(--border); padding: 1px 7px; border-radius: 6px;
  }

  .sec-title {
    display: flex; align-items: center; gap: 8px; margin: 22px 0 10px;
    font-size: 12px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .05em;
  }
  .count { font-size: 10.5px; padding: 1px 7px; border-radius: 20px; background: var(--surface-2); border: 1px solid var(--border); color: var(--mist); }

  .ct-card { border: 1px solid var(--border); border-radius: var(--radius-sm); padding: 12px 14px; margin-bottom: 10px; }
  .ct-card.focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-tint); }
  .ct-head { display: flex; align-items: center; gap: 10px; }
  .ct-head .min0 { flex: 1; }
  .ct-name { font-weight: 650; font-size: 13.5px; color: var(--ink); }
  .ct-img { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist-2); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ct-msg {
    margin-top: 10px; padding: 8px 10px; border-radius: 7px; background: var(--red-tint); color: var(--red);
    font-family: 'JetBrains Mono', monospace; font-size: 11.5px; white-space: pre-wrap; word-break: break-word;
  }
  .ct-grid { display: grid; grid-template-columns: 110px 1fr; gap: 6px 12px; margin-top: 12px; font-size: 12.5px; color: var(--ink); }
  .ct-grid .k { min-width: 0; padding: 0; }

  .ev-list { display: flex; flex-direction: column; border: 1px solid var(--border); border-radius: var(--radius-sm); }
  .ev-row { display: flex; align-items: baseline; gap: 10px; padding: 9px 12px; border-bottom: 1px solid var(--border); }
  .ev-row:last-child { border-bottom: none; }
  .ev-msg { flex: 1; font-size: 12.5px; color: var(--ink); word-break: break-word; }

  @media (max-width: 600px) {
    .k { min-width: 100px; }
    .hbtn { padding: 5px 8px; }
  }
</style>
