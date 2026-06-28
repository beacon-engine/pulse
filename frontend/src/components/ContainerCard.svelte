<script lang="ts">
  import StatusBadge from './StatusBadge.svelte';
  import type { Container } from '../types/containers';

  let {
    container,
    onstart,
    onstop,
    onrestart,
  }: {
    container: Container;
    onstart: (id: string) => void;
    onstop: (id: string) => void;
    onrestart: (id: string) => void;
  } = $props();

  const borderColor: Record<string, string> = {
    running: '#10b981',
    exited:  '#f43f5e',
    paused:  '#f59e0b',
  };

  let accent = $derived(borderColor[container.state] ?? '#94a3b8');
  let busy = $state(false);

  async function act(fn: (id: string) => Promise<void> | void) {
    busy = true;
    try { await fn(container.id); }
    finally { busy = false; }
  }
</script>

<div class="card" style="--accent:{accent}">
  <div class="card-top">
    <div>
      <div class="name">{container.name}</div>
      <div class="image">{container.image}</div>
    </div>
    <StatusBadge state={container.state} />
  </div>

  <div class="status-line">{container.status}</div>

  <div class="actions">
    <button
      class="btn start"
      disabled={container.state === 'running' || busy}
      onclick={() => act(onstart)}
    >Start</button>
    <button
      class="btn stop"
      disabled={container.state !== 'running' || busy}
      onclick={() => act(onstop)}
    >Stop</button>
    <button
      class="btn restart"
      disabled={busy}
      onclick={() => act(onrestart)}
    >Restart</button>
  </div>
</div>

<style>
  .card {
    background: #fff;
    border: 1px solid #e2e8f0;
    border-left: 3px solid var(--accent);
    border-radius: 12px;
    padding: 18px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    box-shadow: 0 1px 3px rgba(0,0,0,0.04);
    transition: box-shadow 0.2s, transform 0.2s;
  }
  .card:hover { box-shadow: 0 8px 24px rgba(0,0,0,0.08); transform: translateY(-1px); }
  .card-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; }
  .name { font-weight: 700; font-size: 0.95rem; color: #0f172a; }
  .image { font-size: 0.75rem; color: #94a3b8; font-family: monospace; margin-top: 2px; }
  .status-line { font-size: 0.78rem; color: #94a3b8; }
  .actions { display: flex; gap: 6px; }
  .btn {
    flex: 1;
    padding: 6px 0;
    border-radius: 7px;
    border: 1px solid #e2e8f0;
    background: #f8fafc;
    color: #475569;
    font-size: 0.78rem;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s;
  }
  .btn:hover:not(:disabled) { background: #f1f5f9; border-color: #cbd5e1; color: #0f172a; }
  .btn:disabled { opacity: 0.35; cursor: not-allowed; }
  .btn.start:hover:not(:disabled)   { background: #d1fae5; border-color: #6ee7b7; color: #065f46; }
  .btn.stop:hover:not(:disabled)    { background: #fee2e2; border-color: #fca5a5; color: #991b1b; }
  .btn.restart:hover:not(:disabled) { background: #ede9fe; border-color: #c4b5fd; color: #5b21b6; }
</style>
