<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import type { Container, ContainerDetail, ContainerPort } from '../types/containers';

  let {
    hostId,
    container,
    onclose,
    onrefresh,
  }: {
    hostId: string;
    container: Container;
    onclose: () => void;
    onrefresh: () => void;
  } = $props();

  type Tab = 'overview' | 'ports' | 'volumes' | 'env' | 'raw' | 'logs';
  let tab = $state<Tab>('overview');

  // ── Inspect ───────────────────────────────────────────────────────────────
  let detail       = $state<ContainerDetail | null>(null);
  let detailError  = $state('');
  let detailLoading = $state(true);

  async function loadDetail() {
    detailLoading = true; detailError = '';
    try { detail = await api.inspectContainer(hostId, container.id); }
    catch (e: any) { detailError = String(e); }
    finally { detailLoading = false; }
  }

  // ── Raw inspect ───────────────────────────────────────────────────────────
  let rawJson     = $state<unknown | null>(null);
  let rawLoading  = $state(false);
  let rawError    = $state('');

  async function loadRaw() {
    rawLoading = true; rawError = '';
    try { rawJson = await api.inspectRawContainer(hostId, container.id); }
    catch (e) { rawError = String(e); }
    finally { rawLoading = false; }
  }

  // ── Logs ──────────────────────────────────────────────────────────────────
  let logs        = $state('');
  let logsLoading = $state(false);
  let logsTail    = $state(200);
  let logEl       = $state<HTMLPreElement | null>(null);

  async function loadLogs() {
    logsLoading = true;
    try { logs = await api.getContainerLogs(hostId, container.id, logsTail); }
    catch (e) { logs = `Error: ${e}`; }
    finally {
      logsLoading = false;
      setTimeout(() => logEl?.scrollTo({ top: logEl.scrollHeight }), 30);
    }
  }

  $effect(() => { if (tab === 'logs' && !logs) loadLogs(); });
  $effect(() => { if (tab === 'raw' && !rawJson) loadRaw(); });

  // ── Port editing ──────────────────────────────────────────────────────────
  let editedPorts   = $state<ContainerPort[]>([]);
  let recreating    = $state(false);
  let recreateError = $state('');
  let recreateOk    = $state(false);

  $effect(() => { if (detail) editedPorts = detail.ports.map(p => ({ ...p })); });

  async function doRecreate() {
    recreating = true; recreateError = ''; recreateOk = false;
    try {
      await api.recreateContainer(hostId, container.id,
        editedPorts.map(p => ({ host_port: p.host_port, container_port: p.container_port, protocol: p.protocol }))
      );
      recreateOk = true; onrefresh();
      setTimeout(onclose, 1200);
    } catch (e: any) { recreateError = e?.response?.data ?? String(e); }
    finally { recreating = false; }
  }

  // ── Env editing ───────────────────────────────────────────────────────────
  let showSecrets = $state(false);
  let editedEnv   = $state<string[]>([]);
  let envSaving   = $state(false);
  let envError    = $state('');
  let envOk       = $state(false);
  const SENSITIVE = /password|secret|token|key|auth|pw/i;

  $effect(() => { if (detail) editedEnv = [...detail.env]; });

  function addEnv() { editedEnv = [...editedEnv, '']; }
  function removeEnv(i: number) { editedEnv = editedEnv.filter((_, idx) => idx !== i); }
  function updateEnv(i: number, val: string) { editedEnv[i] = val; editedEnv = [...editedEnv]; }

  async function saveEnv() {
    envSaving = true; envError = ''; envOk = false;
    try {
      const valid = editedEnv.filter(l => l.trim() && l.includes('='));
      await api.updateContainerEnv(hostId, container.id, valid);
      envOk = true; onrefresh(); await loadDetail();
      setTimeout(() => envOk = false, 2000);
    } catch (e: any) { envError = e?.response?.data ?? String(e); }
    finally { envSaving = false; }
  }

  function maskEnv(line: string): string {
    const eq = line.indexOf('=');
    if (eq === -1) return line;
    const key = line.slice(0, eq);
    const val = line.slice(eq + 1);
    return SENSITIVE.test(key) ? `${key}=••••••••` : `${key}=${val}`;
  }

  // ── Container actions ──────────────────────────────────────────────────────
  let actionBusy = $state(false);
  async function action(fn: () => Promise<void>) {
    actionBusy = true;
    try { await fn(); onrefresh(); await loadDetail(); }
    finally { actionBusy = false; }
  }

  onMount(() => loadDetail());

  const TABS: Tab[] = ['overview', 'ports', 'volumes', 'env', 'raw', 'logs'];
  function tabLabel(t: Tab) { return t === 'raw' ? 'Inspect' : t.charAt(0).toUpperCase() + t.slice(1); }
  function tabBadge(t: Tab) {
    if (!detail) return 0;
    if (t === 'ports') return detail.ports.length;
    if (t === 'volumes') return detail.mounts.length;
    if (t === 'env') return detail.env.length;
    return 0;
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="backdrop" onclick={onclose}></div>

<div class="drawer">
  <!-- Header -->
  <div class="dh">
    <div class="dh-left">
      <span class="state-dot" class:running={container.state === 'running'} class:stopped={container.state === 'exited'} class:other={container.state !== 'running' && container.state !== 'exited'}></span>
      <div>
        <div class="dh-name">{container.name}</div>
        <div class="dh-id">{container.id.slice(0, 12)}</div>
      </div>
    </div>
    <div class="dh-actions">
      <button class="hbtn" class:hbtn-go={true} disabled={container.state === 'running' || actionBusy}
        onclick={() => action(() => api.startContainer(hostId, container.id))}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 4l14 8-14 8V4z"/></svg>
        Start
      </button>
      <button class="hbtn" class:hbtn-danger={true} disabled={container.state !== 'running' || actionBusy}
        onclick={() => action(() => api.stopContainer(hostId, container.id))}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>
        Stop
      </button>
      <button class="hbtn" disabled={actionBusy}
        onclick={() => action(() => api.restartContainer(hostId, container.id))}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
        Restart
      </button>
      <button class="close-x" onclick={onclose}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6 6 18"/></svg>
      </button>
    </div>
  </div>

  <!-- Tabs -->
  <div class="tabs">
    {#each TABS as t}
      <button class="tab-btn" class:active={tab === t} onclick={() => (tab = t)}>
        {tabLabel(t)}
        {#if tabBadge(t) > 0}
          <span class="tab-badge">{tabBadge(t)}</span>
        {/if}
      </button>
    {/each}
  </div>

  <!-- Body -->
  <div class="body">
    {#if detailLoading && tab !== 'logs'}
      <div class="state-msg">Loading…</div>
    {:else if detailError}
      <div class="err-box">{detailError}</div>

    {:else if tab === 'overview' && detail}
      <div class="kv-list">
        <div class="kv-row"><span class="k">Image</span><span class="v mono">{detail.image}</span></div>
        <div class="kv-row"><span class="k">State</span>
          <span class="v">
            <span class="pill {container.state === 'running' ? 'pill-green' : container.state === 'exited' ? 'pill-red' : 'pill-amber'}">
              {#if container.state === 'running'}<span class="dot"></span>{/if}
              {detail.state}
            </span>
          </span>
        </div>
        <div class="kv-row"><span class="k">Created</span><span class="v">{new Date(detail.created).toLocaleString()}</span></div>
        <div class="kv-row"><span class="k">Started</span><span class="v">{detail.started_at ? new Date(detail.started_at).toLocaleString() : '—'}</span></div>
        {#if detail.state !== 'running' && detail.finished_at}
          <div class="kv-row"><span class="k">Finished</span><span class="v">{new Date(detail.finished_at).toLocaleString()}</span></div>
        {/if}
        <div class="kv-row"><span class="k">Restarts</span><span class="v">{detail.restart_count}</span></div>
        <div class="kv-row"><span class="k">Restart policy</span><span class="v">{detail.restart_policy || 'no'}</span></div>
        <div class="kv-row"><span class="k">Network mode</span><span class="v mono">{detail.network_mode}</span></div>
        {#if detail.cmd.length}
          <div class="kv-row"><span class="k">Command</span><span class="v mono wrap">{detail.cmd.join(' ')}</span></div>
        {/if}
      </div>

    {:else if tab === 'ports' && detail}
      {#if detail.ports.length === 0}
        <div class="state-msg">No ports exposed</div>
      {:else}
        <table class="detail-table">
          <thead><tr><th>Container</th><th>Protocol</th><th>Host IP</th><th>Host port</th></tr></thead>
          <tbody>
            {#each editedPorts as port, i}
              <tr>
                <td class="mono">{port.container_port}</td>
                <td><span class="proto-tag">{port.protocol}</span></td>
                <td class="mono muted">{port.host_ip || '0.0.0.0'}</td>
                <td>
                  <input class="port-input" type="text" bind:value={editedPorts[i].host_port} placeholder="host port" />
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
        <div class="recreate-bar">
          {#if recreateOk}
            <span class="ok-msg">✓ Container recreated successfully</span>
          {:else}
            {#if recreateError}<span class="err-inline">{recreateError}</span>{/if}
            <button class="btn-primary" onclick={doRecreate} disabled={recreating}>
              {recreating ? 'Recreating…' : 'Recreate with new ports'}
            </button>
          {/if}
        </div>
        <p class="hint">Recreating stops and replaces the container with the same image but updated port mappings.</p>
      {/if}

    {:else if tab === 'volumes' && detail}
      {#if detail.mounts.length === 0}
        <div class="state-msg">No volumes mounted</div>
      {:else}
        <table class="detail-table">
          <thead><tr><th>Source</th><th>Destination</th><th>Mode</th></tr></thead>
          <tbody>
            {#each detail.mounts as m}
              <tr>
                <td class="mono small">{m.source || '(anonymous)'}</td>
                <td class="mono small">{m.destination}</td>
                <td><span class="proto-tag" class:rw={m.rw}>{m.rw ? 'rw' : 'ro'}</span></td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}

    {:else if tab === 'env' && detail}
      {#if editedEnv.length === 0}
        <div class="state-msg">No environment variables</div>
      {:else}
        <div class="env-toolbar">
          <button class="btn-ghost btn-sm" onclick={() => (showSecrets = !showSecrets)}>
            {showSecrets ? 'Hide secrets' : 'Show secrets'}
          </button>
        </div>
        <div class="env-list">
          {#each editedEnv as line, i}
            {@const parts = line.split('=')}
            {@const key   = parts[0]}
            {@const val   = parts.slice(1).join('=')}
            {@const sensitive = SENSITIVE.test(key) && !showSecrets}
            <div class="env-row">
              <input class="env-input" type={showSecrets ? 'text' : 'password'} value={line} oninput={(e) => updateEnv(i, e.currentTarget.value)} />
              <button class="env-del" onclick={() => removeEnv(i)} aria-label="Remove">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"/></svg>
              </button>
            </div>
            <div class="env-row">
              <span class="env-key">{key}</span>
              <span class="env-val" class:redacted={sensitive}>{sensitive ? '••••••••' : val}</span>
            </div>
          {/each}
        </div>
        <button class="btn-ghost btn-sm" onclick={addEnv} style="margin-top:8px">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14M5 12h14"/></svg>
          Add variable
        </button>
        <div class="recreate-bar" style="margin-top:14px">
          {#if envOk}<span class="ok-msg">✓ Environment updated — container restarted</span>{/if}
          {#if envError}<span class="err-inline">{envError}</span>{/if}
          <button class="btn-primary" onclick={saveEnv} disabled={envSaving}>
            {envSaving ? 'Saving…' : 'Save & restart'}
          </button>
        </div>
        <p class="hint">Saving recreates the container with the updated environment variables and starts it.</p>
      {/if}

    {:else if tab === 'raw'}
      {#if rawLoading}
        <div class="state-msg">Loading raw inspect…</div>
      {:else if rawError}
        <div class="err-box">{rawError}</div>
      {:else}
        <div class="raw-toolbar">
          <button class="btn-ghost btn-sm" onclick={loadRaw} disabled={rawLoading}>Refresh</button>
        </div>
        <pre class="raw-json">{JSON.stringify(rawJson, null, 2)}</pre>
      {/if}

    {:else if tab === 'logs'}
      <div class="log-toolbar">
        <select class="select-sm" bind:value={logsTail} onchange={loadLogs}>
          <option value={50}>50 lines</option>
          <option value={200}>200 lines</option>
          <option value={500}>500 lines</option>
          <option value={2000}>2000 lines</option>
        </select>
        <button class="btn-ghost btn-sm" onclick={loadLogs} disabled={logsLoading}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5"/></svg>
          Refresh
        </button>
      </div>
      <pre class="terminal" bind:this={logEl}>{#if logsLoading}<span class="muted">Loading…</span>{:else if !logs.trim()}<span class="muted">No output</span>{:else}{logs}{/if}</pre>
    {/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed; inset: 0; background: rgba(20,22,40,.38);
    z-index: 50; backdrop-filter: blur(1px);
  }
  .drawer {
    position: fixed; top: 0; right: 0; bottom: 0; width: 580px; max-width: 95vw;
    background: var(--surface); border-left: 1px solid var(--border);
    box-shadow: var(--shadow-lg); z-index: 51;
    display: flex; flex-direction: column; overflow: hidden;
  }

  /* Header */
  .dh {
    display: flex; justify-content: space-between; align-items: center;
    padding: 16px 20px; border-bottom: 1px solid var(--border); flex-shrink: 0; gap: 12px;
  }
  .dh-left { display: flex; align-items: center; gap: 12px; min-width: 0; }
  .state-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; background: var(--mist-2); }
  .state-dot.running { background: var(--green); animation: pulse-dot 2s infinite; }
  .state-dot.stopped { background: var(--red); }
  .state-dot.other   { background: var(--amber); }
  @keyframes pulse-dot {
    0%   { box-shadow: 0 0 0 0 rgba(28,154,99,.4); }
    70%  { box-shadow: 0 0 0 6px rgba(28,154,99,0); }
    100% { box-shadow: 0 0 0 0 rgba(28,154,99,0); }
  }
  .dh-name { font-family: 'Manrope', sans-serif; font-weight: 700; font-size: 15px; color: var(--ink); }
  .dh-id   { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist-2); margin-top: 2px; }
  .dh-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }

  .hbtn {
    display: flex; align-items: center; gap: 5px;
    padding: 5px 11px; border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--mist); font-size: 12.5px; font-weight: 600;
    transition: all .12s;
  }
  .hbtn svg { width: 13px; height: 13px; }
  .hbtn:hover:not(:disabled) { background: var(--surface); color: var(--ink); border-color: var(--border-strong); }
  .hbtn:disabled { opacity: .35; cursor: not-allowed; }
  .hbtn-go:hover:not(:disabled)     { background: var(--green-tint); color: var(--green); border-color: var(--green); }
  .hbtn-danger:hover:not(:disabled) { background: var(--red-tint);   color: var(--red);   border-color: var(--red); }
  .close-x {
    width: 30px; height: 30px; border-radius: 8px; border: none;
    background: var(--surface-2); display: flex; align-items: center; justify-content: center;
    color: var(--mist); transition: background .12s, color .12s;
  }
  .close-x:hover { background: var(--border); color: var(--ink); }
  .close-x svg { width: 14px; height: 14px; }

  /* Tabs */
  .tabs {
    display: flex; border-bottom: 1px solid var(--border);
    padding: 0 16px; flex-shrink: 0; overflow-x: auto;
    background: var(--surface);
  }
  .tab-btn {
    display: flex; align-items: center; gap: 5px;
    padding: 10px 4px; margin-right: 18px; border: none; background: none;
    font-weight: 650; font-size: 13px; color: var(--mist);
    border-bottom: 2px solid transparent; cursor: pointer; white-space: nowrap;
    transition: color .15s, border-color .15s; margin-bottom: -1px;
  }
  .tab-btn:hover { color: var(--ink); }
  .tab-btn.active { color: var(--accent-ink); border-color: var(--accent); }
  .tab-badge {
    background: var(--surface-2); border: 1px solid var(--border);
    font-size: 10.5px; font-weight: 700; padding: 1px 6px;
    border-radius: 20px; color: var(--mist);
  }
  .tab-btn.active .tab-badge { background: var(--accent-tint); color: var(--accent-ink); border-color: var(--accent-tint); }

  /* Body */
  .body { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 0; }
  .state-msg { color: var(--mist-2); font-size: 13.5px; padding: 30px 0; text-align: center; }
  .err-box { background: var(--red-tint); color: var(--red); border-radius: var(--radius-sm); padding: 10px 14px; font-size: 13px; }

  /* KV overview */
  .kv-list { display: flex; flex-direction: column; }
  .kv-row { display: flex; gap: 16px; padding: 10px 0; border-bottom: 1px solid var(--border); align-items: flex-start; }
  .kv-row:last-child { border-bottom: none; }
  .k { font-size: 12px; font-weight: 700; color: var(--mist); min-width: 130px; padding-top: 2px; }
  .v { font-size: 13px; color: var(--ink); flex: 1; }
  .mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .muted { color: var(--mist-2); }
  .wrap { word-break: break-all; }

  /* Detail table */
  .detail-table { width: 100%; border-collapse: collapse; margin-bottom: 14px; }
  .detail-table th {
    text-align: left; font-size: 11px; font-weight: 700; text-transform: uppercase;
    letter-spacing: .05em; color: var(--mist-2); padding: 8px 12px;
    background: var(--surface-2); border-bottom: 1px solid var(--border);
  }
  .detail-table td { padding: 9px 12px; font-size: 13px; border-bottom: 1px solid var(--border); }
  .detail-table tr:last-child td { border-bottom: none; }
  .small { font-size: 11.5px; word-break: break-all; }
  .proto-tag {
    display: inline-block; padding: 2px 8px; border-radius: 20px;
    font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em;
    background: var(--accent-tint); color: var(--accent-ink);
  }
  .proto-tag.rw { background: var(--green-tint); color: var(--green); }
  .port-input {
    padding: 5px 8px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    font-size: 12.5px; font-family: 'JetBrains Mono', monospace; width: 100px;
    color: var(--ink); background: var(--surface-2); outline: none; transition: border-color .15s;
  }
  .port-input:focus { border-color: var(--accent); background: var(--surface); }

  /* Recreate */
  .recreate-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }
  .ok-msg    { color: var(--green); font-size: 13px; font-weight: 600; }
  .err-inline { color: var(--red); font-size: 12.5px; flex: 1; }
  .hint { font-size: 12px; color: var(--mist-2); line-height: 1.6; margin: 0; }

  /* Buttons */
  .btn-primary {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 7px 14px; border-radius: var(--radius-sm); border: none;
    background: var(--accent); color: #fff; font-size: 13px; font-weight: 650;
    transition: filter .15s;
  }
  .btn-primary:hover:not(:disabled) { filter: brightness(1.07); }
  .btn-primary:disabled { opacity: .45; cursor: not-allowed; }
  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 7px 12px; border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600;
    transition: background .12s, border-color .12s;
  }
  .btn-ghost:hover:not(:disabled) { background: var(--surface); border-color: var(--border-strong); }
  .btn-ghost:disabled { opacity: .4; cursor: not-allowed; }
  .btn-ghost svg { width: 13px; height: 13px; }
  .btn-sm { padding: 5px 10px; font-size: 12px; }

  /* Env */
  .env-toolbar { display: flex; justify-content: flex-end; margin-bottom: 10px; }
  .env-list { display: flex; flex-direction: column; }
  .env-row { display: flex; padding: 7px 0; border-bottom: 1px solid var(--border); align-items: flex-start; min-width: 0; }
  .env-row:last-child { border-bottom: none; }
  .env-key { font-family: 'JetBrains Mono', monospace; font-size: 12px; font-weight: 600; color: var(--accent); min-width: 170px; flex-shrink: 0; padding-right: 10px; word-break: break-all; }
  .env-val { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--mist); word-break: break-all; flex: 1; }
  .env-val.redacted { color: var(--border-strong); letter-spacing: .1em; }
  .env-input {
    flex: 1; padding: 6px 9px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface-2); color: var(--ink); font-family: 'JetBrains Mono', monospace; font-size: 12px;
    outline: none; transition: border-color .15s;
  }
  .env-input:focus { border-color: var(--accent); background: var(--surface); }
  .env-del {
    width: 28px; height: 28px; border-radius: 6px; border: none; background: var(--surface-2);
    color: var(--mist); display: flex; align-items: center; justify-content: center; margin-left: 6px;
    cursor: pointer; transition: all .12s;
  }
  .env-del:hover { background: var(--red-tint); color: var(--red); }
  .env-del svg { width: 13px; height: 13px; }

  /* Raw inspect */
  .raw-toolbar { display: flex; justify-content: flex-end; margin-bottom: 10px; }
  .raw-json {
    flex: 1; background: #15171F; border-radius: var(--radius-md);
    padding: 14px 16px; font-family: 'JetBrains Mono', monospace; font-size: 11px;
    line-height: 1.55; color: #C7CBDA; overflow-y: auto; min-height: 320px; margin: 0;
    white-space: pre;
  }

  /* Logs */
  .log-toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-shrink: 0; }
  .select-sm {
    padding: 5px 10px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    font-size: 12.5px; background: var(--surface-2); color: var(--ink); font-family: inherit;
  }
  .terminal {
    flex: 1; background: #15171F; border-radius: var(--radius-md);
    padding: 14px 16px; font-family: 'JetBrains Mono', monospace; font-size: 12px;
    line-height: 1.6; color: #C7CBDA; white-space: pre; overflow-y: auto;
    min-height: 300px; margin: 0;
  }
  .terminal .muted { color: #5F6580; }
  .terminal::-webkit-scrollbar { width: 5px; }
  .terminal::-webkit-scrollbar-thumb { background: #2D3040; border-radius: 4px; }
</style>
