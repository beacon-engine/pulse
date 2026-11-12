<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';

  let {
    hostId,
    containerId,
    containerName,
    fetchLogs,
    onclose,
  }: {
    hostId?: string;
    containerId?: string;
    containerName: string;
    /** Overrides the Docker fetch, e.g. for a Kubernetes pod's container. */
    fetchLogs?: (tail: number) => Promise<string>;
    onclose: () => void;
  } = $props();

  let logs = $state('');
  let loading = $state(true);
  let tail = $state(200);
  let logEl = $state<HTMLPreElement | null>(null);

  async function load() {
    loading = true;
    try {
      logs = fetchLogs
        ? await fetchLogs(tail)
        : await api.getContainerLogs(hostId!, containerId!, tail);
    } catch (e) {
      const body = (e as any)?.response?.data;
      logs = `Error fetching logs: ${typeof body === 'string' && body ? body : e}`;
    } finally {
      loading = false;
      // scroll to bottom after render
      setTimeout(() => logEl?.scrollTo({ top: logEl.scrollHeight, behavior: 'smooth' }), 50);
    }
  }

  onMount(() => load());
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="overlay" onclick={onclose}>
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <div class="header">
      <div class="title">
        <span class="icon">≡</span>
        <span class="name">{containerName}</span>
        <span class="sub">logs</span>
      </div>
      <div class="controls">
        <select bind:value={tail} onchange={load}>
          <option value={50}>Last 50 lines</option>
          <option value={200}>Last 200 lines</option>
          <option value={500}>Last 500 lines</option>
          <option value={2000}>Last 2000 lines</option>
        </select>
        <button class="refresh" onclick={load} disabled={loading}>↺</button>
        <button class="close" onclick={onclose}>✕</button>
      </div>
    </div>

    <pre class="log-output" bind:this={logEl}>
      {#if loading}
        <span class="muted">Loading…</span>
      {:else if !logs.trim()}
        <span class="muted">No output</span>
      {:else}{logs}{/if}
    </pre>
  </div>
</div>

<style>
  .overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.7);
    backdrop-filter: blur(2px);
    display: flex; align-items: center; justify-content: center;
    z-index: 200; padding: 24px;
  }
  .modal {
    background: #0d1117; border-radius: 12px;
    width: 900px; max-width: 100%; max-height: 80vh;
    display: flex; flex-direction: column;
    box-shadow: 0 32px 80px rgba(0,0,0,0.5);
    border: 1px solid #21262d;
    overflow: hidden;
  }
  .header {
    display: flex; justify-content: space-between; align-items: center;
    padding: 14px 20px; border-bottom: 1px solid #21262d;
    background: #161b22; flex-shrink: 0;
  }
  .title { display: flex; align-items: center; gap: 8px; }
  .icon { color: #58a6ff; font-size: 1rem; }
  .name { color: #e6edf3; font-weight: 600; font-size: 0.9rem; }
  .sub { color: #484f58; font-size: 0.8rem; }
  .controls { display: flex; align-items: center; gap: 8px; }
  select {
    background: #21262d; border: 1px solid #30363d; color: #e6edf3;
    border-radius: 6px; padding: 4px 8px; font-size: 0.78rem; cursor: pointer;
  }
  .refresh, .close {
    background: #21262d; border: 1px solid #30363d; color: #8b949e;
    border-radius: 6px; padding: 4px 9px; font-size: 0.85rem;
    cursor: pointer; transition: all 0.15s;
  }
  .refresh:hover:not(:disabled), .close:hover { color: #e6edf3; border-color: #58a6ff; }
  .refresh:disabled { opacity: 0.4; cursor: not-allowed; }
  .log-output {
    flex: 1; overflow-y: auto; overflow-x: auto;
    padding: 16px 20px; margin: 0;
    font-family: 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
    font-size: 0.75rem; line-height: 1.6;
    color: #c9d1d9; white-space: pre; background: #0d1117;
  }
  .muted { color: #484f58; }
  .log-output::-webkit-scrollbar { width: 6px; height: 6px; }
  .log-output::-webkit-scrollbar-track { background: transparent; }
  .log-output::-webkit-scrollbar-thumb { background: #30363d; border-radius: 3px; }
</style>
