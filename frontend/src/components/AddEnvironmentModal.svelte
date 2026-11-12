<script lang="ts">
  import { api } from '../lib/api';
  import { hostsStore } from '../stores/hosts.svelte';
  import { toast } from '../lib/toast.svelte';
  import { RUNTIME_LABEL } from '../types/environments';

  let {
    onclose,
    oncreated,
  }: {
    onclose: () => void;
    oncreated: () => void;
  } = $props();

  // "agent": a Pulse agent in listen mode, which brings whatever runtimes it can
  // reach (Docker, Podman, Kubernetes). "endpoint": a bare container API that
  // the default agent dials on Pulse's behalf.
  let mode = $state<'agent' | 'endpoint'>('agent');
  let name = $state('');
  let agentAddr = $state('');
  let dockerHost = $state('');
  let error = $state('');
  let loading = $state(false);

  async function submit() {
    const target =
      mode === 'agent'
        ? { agentAddr: agentAddr.trim(), dockerHost: dockerHost.trim() }
        : { dockerHost: dockerHost.trim() };
    if (!name.trim()) {
      error = 'Name is required.';
      return;
    }
    if (mode === 'agent' ? !target.agentAddr : !target.dockerHost) {
      error = mode === 'agent' ? 'Agent address is required.' : 'Endpoint URL is required.';
      return;
    }
    loading = true;
    error = '';
    try {
      const env = await api.createHost(name.trim(), target);
      const caps = await hostsStore.probe(env.id);
      if (!caps.reachable) {
        toast.error(`Added "${env.name}", but its agent isn't reachable: ${caps.error ?? 'unknown error'}`);
      } else if (caps.runtimes.length === 0) {
        toast.error(`Added "${env.name}", but no runtime was reachable through it.`);
      } else {
        const found = caps.runtimes.map((r) => RUNTIME_LABEL[r.kind] ?? r.kind).join(', ');
        toast.success(`Added "${env.name}" — ${found}`);
      }
      oncreated();
    } catch (e: any) {
      error = e?.response?.data ?? String(e);
    } finally {
      loading = false;
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="overlay" onclick={onclose}>
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="modal" onclick={(e) => e.stopPropagation()}>
    <div class="modal-header">
      <h2>Add environment</h2>
      <button class="close" onclick={onclose}>✕</button>
    </div>

    <div class="seg" role="tablist">
      <button role="tab" class:on={mode === 'agent'} onclick={() => (mode = 'agent')} disabled={loading}>
        Pulse agent
      </button>
      <button role="tab" class:on={mode === 'endpoint'} onclick={() => (mode = 'endpoint')} disabled={loading}>
        Container API endpoint
      </button>
    </div>

    {#if mode === 'agent'}
      <p class="hint">
        A Pulse agent running in <code>listen</code> mode. Pulse uses whatever it can reach there:
        Docker, Podman or Kubernetes. Agents in <code>dial</code> (push) mode register
        themselves and don't need to be added here.
      </p>
    {:else}
      <p class="hint">
        A Docker or Podman API that Pulse's default agent can reach, such as
        <code>tcp://192.168.1.10:2375</code> or <code>unix:///run/podman/podman.sock</code>.
      </p>
    {/if}

    <label>
      Name
      <input type="text" bind:value={name} placeholder="Production cluster" disabled={loading} />
    </label>

    {#if mode === 'agent'}
      <label>
        Agent address
        <input
          type="text"
          bind:value={agentAddr}
          placeholder="10.0.0.5:9000 or wss://agent.example.com/agent"
          disabled={loading}
        />
      </label>
      <label>
        <span>Docker / Podman socket on the agent's machine <span class="opt">optional</span></span>
        <input
          type="text"
          bind:value={dockerHost}
          placeholder="unix:///run/podman/podman.sock"
          disabled={loading}
        />
        <span class="field-hint">
          Only if the agent should use a container API other than its default
          <code>/var/run/docker.sock</code>. Leave empty for Kubernetes agents.
        </span>
      </label>
    {:else}
      <label>
        Endpoint URL
        <input type="text" bind:value={dockerHost} placeholder="tcp://192.168.1.10:2375" disabled={loading} />
      </label>
    {/if}

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <div class="actions">
      <button class="btn cancel" onclick={onclose} disabled={loading}>Cancel</button>
      <button class="btn primary" onclick={submit} disabled={loading}>
        {loading ? 'Connecting…' : 'Add environment'}
      </button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed; inset: 0;
    background: rgba(15,23,42,0.6);
    backdrop-filter: blur(2px);
    display: flex; align-items: center; justify-content: center;
    z-index: 100;
  }
  .modal {
    background: #fff; border-radius: 16px;
    padding: 28px 32px; width: 440px; max-width: 95vw;
    box-shadow: 0 24px 64px rgba(0,0,0,0.18);
    display: flex; flex-direction: column; gap: 16px;
  }
  .modal-header { display: flex; justify-content: space-between; align-items: center; }
  h2 { font-size: 1.1rem; font-weight: 700; color: #0f172a; letter-spacing: -0.02em; }
  .close {
    background: none; border: none; color: #94a3b8; font-size: 0.85rem;
    cursor: pointer; padding: 4px 8px; border-radius: 6px; transition: color 0.15s;
  }
  .close:hover { color: #0f172a; }
  .hint { font-size: 0.82rem; color: #64748b; line-height: 1.5; }
  code { background: #f1f5f9; padding: 1px 5px; border-radius: 4px; font-size: 0.78rem; }
  label { display: flex; flex-direction: column; gap: 6px; font-size: 0.8rem; font-weight: 600; color: #374151; }
  input {
    padding: 9px 12px; border-radius: 8px;
    border: 1px solid #e2e8f0; background: #f8fafc;
    font-size: 0.875rem; color: #0f172a;
    outline: none; transition: border-color 0.15s, box-shadow 0.15s;
  }
  input:focus { border-color: #818cf8; box-shadow: 0 0 0 3px rgba(129,140,248,0.15); }
  input:disabled { opacity: 0.5; cursor: not-allowed; }
  .seg { display: flex; gap: 4px; background: #f1f5f9; padding: 4px; border-radius: 10px; }
  .seg button {
    flex: 1; padding: 7px 10px; border: none; border-radius: 7px; background: none;
    font-size: 0.8rem; font-weight: 600; color: #64748b; cursor: pointer;
  }
  .seg button.on { background: #fff; color: #0f172a; box-shadow: 0 1px 3px rgba(0,0,0,0.08); }
  .opt { font-weight: 500; color: #94a3b8; margin-left: 4px; }
  .field-hint { font-size: 0.75rem; font-weight: 400; color: #94a3b8; line-height: 1.45; }
  .field-hint code { font-size: 0.72rem; }
  .error { background: #fee2e2; color: #991b1b; border-radius: 8px; padding: 8px 12px; font-size: 0.8rem; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 4px; }
  .btn { padding: 9px 18px; border-radius: 8px; font-size: 0.85rem; font-weight: 600; cursor: pointer; border: none; transition: all 0.15s; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .cancel { background: #f1f5f9; color: #475569; }
  .cancel:hover:not(:disabled) { background: #e2e8f0; }
  .primary { background: #818cf8; color: #fff; }
  .primary:hover:not(:disabled) { background: #6366f1; }
</style>
