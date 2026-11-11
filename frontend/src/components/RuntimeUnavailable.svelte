<script lang="ts">
  import { hostsStore } from '../stores/hosts.svelte';
  import { RUNTIME_LABEL, type Requirement } from '../types/environments';

  let { requirement }: { requirement: Requirement } = $props();

  const NEEDS: Record<Requirement, string> = {
    containers: 'a container runtime (Docker or Podman)',
    compose: 'a container runtime with Compose',
    swarm: 'Docker with Swarm mode active',
    kubernetes: 'a reachable Kubernetes cluster',
  };

  let caps = $derived(hostsStore.activeCapabilities);
  let checking = $state(false);

  async function recheck() {
    if (!hostsStore.activeHostId) return;
    checking = true;
    try {
      await hostsStore.probe(hostsStore.activeHostId);
    } finally {
      checking = false;
    }
  }
</script>

<div class="panel">
  {#if caps && !caps.reachable}
    <h2>Can't reach this environment's agent</h2>
    <p>Pulse couldn't talk to the agent for <b>{hostsStore.activeHost?.name}</b>.</p>
    {#if caps.error}<pre>{caps.error}</pre>{/if}
  {:else}
    <h2>Not available on this environment</h2>
    <p>This page needs {NEEDS[requirement]}, and <b>{hostsStore.activeHost?.name}</b> doesn't have one.</p>
    {#if caps && caps.runtimes.length > 0}
      <p class="muted">
        Available here:
        {caps.runtimes.map((r) => `${RUNTIME_LABEL[r.kind] ?? r.kind} ${r.version}`.trim()).join(', ')}
      </p>
    {/if}
    {#if caps && caps.unavailable.length > 0}
      <ul>
        {#each caps.unavailable as u}
          <li><b>{RUNTIME_LABEL[u.kind] ?? u.kind}:</b> <span class="mono">{u.error}</span></li>
        {/each}
      </ul>
    {/if}
  {/if}
  <button class="btn" onclick={recheck} disabled={checking}>
    {checking ? 'Checking…' : 'Check again'}
  </button>
</div>

<style>
  .panel {
    max-width: 640px; margin: 48px auto; padding: 28px 32px;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    display: flex; flex-direction: column; gap: 12px; align-items: flex-start;
  }
  h2 { font-size: 16px; font-weight: 700; color: var(--ink); }
  p { font-size: 13.5px; color: var(--mist); line-height: 1.5; }
  .muted { color: var(--mist-2); }
  ul { margin: 0; padding-left: 18px; font-size: 12.5px; color: var(--mist); display: flex; flex-direction: column; gap: 4px; }
  .mono, pre { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  pre {
    white-space: pre-wrap; word-break: break-word; margin: 0; width: 100%;
    background: var(--surface-2); color: var(--mist); padding: 10px 12px; border-radius: var(--radius-sm);
  }
  .btn {
    margin-top: 4px; padding: 7px 14px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600; cursor: pointer;
  }
  .btn:hover:not(:disabled) { border-color: var(--border-strong); }
  .btn:disabled { opacity: .5; cursor: default; }
</style>
