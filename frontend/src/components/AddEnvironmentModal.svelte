<script lang="ts">
  import { api } from '../lib/api';

  let {
    onclose,
    oncreated,
  }: {
    onclose: () => void;
    oncreated: () => void;
  } = $props();

  let name = $state('');
  let dockerHost = $state('');
  let error = $state('');
  let loading = $state(false);

  async function submit() {
    if (!name.trim() || !dockerHost.trim()) {
      error = 'Both fields are required.';
      return;
    }
    loading = true;
    error = '';
    try {
      await api.createHost(name.trim(), dockerHost.trim());
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
      <h2>Add host</h2>
      <button class="close" onclick={onclose}>✕</button>
    </div>

    <p class="hint">
      Connect Beacon to a remote Docker host. The daemon must be accessible over
      TCP — e.g.&nbsp;<code>tcp://192.168.1.10:2375</code>.
    </p>

    <label>
      Name
      <input
        type="text"
        bind:value={name}
        placeholder="Production server"
        disabled={loading}
      />
    </label>

    <label>
      Docker host URL
      <input
        type="text"
        bind:value={dockerHost}
        placeholder="tcp://192.168.1.10:2375"
        disabled={loading}
      />
    </label>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <div class="actions">
      <button class="btn cancel" onclick={onclose} disabled={loading}>Cancel</button>
      <button class="btn primary" onclick={submit} disabled={loading}>
        {loading ? 'Connecting…' : 'Add host'}
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
  .error { background: #fee2e2; color: #991b1b; border-radius: 8px; padding: 8px 12px; font-size: 0.8rem; }
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 4px; }
  .btn { padding: 9px 18px; border-radius: 8px; font-size: 0.85rem; font-weight: 600; cursor: pointer; border: none; transition: all 0.15s; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .cancel { background: #f1f5f9; color: #475569; }
  .cancel:hover:not(:disabled) { background: #e2e8f0; }
  .primary { background: #818cf8; color: #fff; }
  .primary:hover:not(:disabled) { background: #6366f1; }
</style>
