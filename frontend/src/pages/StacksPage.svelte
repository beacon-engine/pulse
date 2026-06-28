<script lang="ts">
  import { api } from '../lib/api';
  import { getAppState } from '../lib/appContext';
  import type { SavedStack } from '../types/containers';

  const app = getAppState();

  let showStackEditor = $state(false);
  let editingStackId  = $state<string | null>(null);
  let stackName       = $state('');
  let stackContent    = $state('');
  let stackOpBusy     = $state(false);
  let stackOpError    = $state('');

  function openStackEditor(stack?: SavedStack) {
    editingStackId = stack ? stack.id : null;
    stackName      = stack ? stack.name : '';
    stackContent   = stack ? stack.content : '';
    stackOpError   = '';
    showStackEditor = true;
  }

  function closeStackEditor() {
    showStackEditor = false;
    editingStackId = null;
  }

  async function saveStack() {
    if (!app.activeHostId) return;
    stackOpBusy = true; stackOpError = '';
    try {
      if (editingStackId) {
        await api.updateSavedStack(editingStackId, stackContent);
        app.toast('Stack updated');
      } else {
        await api.createSavedStack(app.activeHostId, stackName, stackContent);
        app.toast('Stack created');
      }
      closeStackEditor();
      await app.refresh();
    } catch (e: any) { stackOpError = e?.response?.data ?? String(e); }
    finally { stackOpBusy = false; }
  }

  async function runStackUp(id: string) {
    stackOpBusy = true;
    try {
      await api.stackUp(id);
      app.toast('Stack started');
      await app.refresh();
    } catch (e: any) { app.toast(e?.response?.data ?? String(e), 'err'); }
    finally { stackOpBusy = false; }
  }

  async function runStackDown(id: string) {
    stackOpBusy = true;
    try {
      await api.stackDown(id);
      app.toast('Stack stopped', 'warn');
      await app.refresh();
    } catch (e: any) { app.toast(e?.response?.data ?? String(e), 'err'); }
    finally { stackOpBusy = false; }
  }

  async function removeSavedStack(id: string) {
    if (!confirm('Delete this stack definition?')) return;
    try {
      await api.deleteSavedStack(id);
      app.toast('Stack deleted', 'warn');
      await app.refresh();
    } catch (e: any) { app.toast(e?.response?.data ?? String(e), 'err'); }
  }
</script>

{#if app.loading}
  <div class="state-msg">Loading stacks…</div>
{:else if app.activeHostId === null}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="3" width="7" height="7" rx="1.4"/><rect x="14" y="3" width="7" height="7" rx="1.4"/><rect x="3" y="14" width="7" height="7" rx="1.4"/><rect x="14" y="14" width="7" height="7" rx="1.4"/></svg>
    <p>No host selected — add one in the sidebar</p>
  </div>
{:else}
  <div class="stack-toolbar">
    <h2 class="section-title">Saved stacks</h2>
    <button class="btn-primary" onclick={() => openStackEditor()}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14M5 12h14"/></svg>
      New stack
    </button>
  </div>

  {#if app.savedStacks.length === 0}
    <div class="empty-state small">
      <p>No saved stacks yet</p>
      <span class="empty-hint">Create a compose definition and run it from Beacon</span>
    </div>
  {:else}
    <div class="saved-stack-list">
      {#each app.savedStacks as s (s.id)}
        <div class="saved-stack-card">
          <div class="saved-stack-hdr">
            <div>
              <span class="saved-stack-name">{s.name}</span>
              <span class="saved-stack-status pill {s.status === 'running' ? 'pill-green' : 'pill-mist'}">{s.status}</span>
            </div>
            <div class="saved-stack-actions">
              <button class="btn-ghost btn-sm" onclick={() => runStackUp(s.id)} disabled={stackOpBusy}>Start</button>
              <button class="btn-ghost btn-sm" onclick={() => runStackDown(s.id)} disabled={stackOpBusy}>Stop</button>
              <button class="btn-ghost btn-sm" onclick={() => openStackEditor(s)}>Edit</button>
              <button class="btn-ghost btn-sm btn-danger" onclick={() => removeSavedStack(s.id)}>Delete</button>
            </div>
          </div>
          <div class="saved-stack-meta">Updated {new Date(s.updated_at).toLocaleString()}</div>
        </div>
      {/each}
    </div>
  {/if}

  <h2 class="section-title" style="margin-top:28px">Detected compose projects</h2>
  {#if app.stacks.length === 0}
    <div class="empty-state small">
      <p>No compose projects detected</p>
      <span class="empty-hint">Requires <code>com.docker.compose.project</code> label</span>
    </div>
  {:else}
    <div class="stack-grid">
      {#each app.stacks as stack (stack.name)}
        <div class="stack-card">
          <div class="stack-hdr">
            <span class="stack-name">{stack.name}</span>
            <span class="stack-ct">{stack.services.length} service{stack.services.length !== 1 ? 's' : ''}</span>
          </div>
          {#each stack.services as svc (svc.id)}
            <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
            <div class="svc-row" onclick={() => app.selectedContainer = svc}>
              <span class="svc-dot" style="background:{svc.state === 'running' ? 'var(--green)' : svc.state === 'exited' ? 'var(--red)' : 'var(--amber)'}"></span>
              <span class="svc-name">{svc.name}</span>
              <span class="svc-img">{svc.image}</span>
              <span class="svc-stat pill {svc.state === 'running' ? 'pill-green' : 'pill-red'}">{svc.status}</span>
            </div>
          {/each}
        </div>
      {/each}
    </div>
  {/if}
{/if}

{#if showStackEditor}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="modal-backdrop" onclick={closeStackEditor}></div>
  <div class="modal" role="dialog">
    <div class="modal-h">
      <h3>{editingStackId ? 'Edit stack' : 'New stack'}</h3>
      <button class="close-x" onclick={closeStackEditor}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6 6 18"/></svg>
      </button>
    </div>
    <div class="modal-body">
      {#if !editingStackId}
        <label class="field-label">Stack name</label>
        <input class="text-input" bind:value={stackName} placeholder="e.g. monitoring" />
      {/if}
      <label class="field-label">Compose YAML</label>
      <textarea class="code-textarea" bind:value={stackContent} rows="14" placeholder="services:\n  web:\n    image: nginx:latest\n    ports:\n      - 8080:80"></textarea>
      {#if stackOpError}<div class="err-box">{stackOpError}</div>{/if}
    </div>
    <div class="modal-ft">
      <button class="btn-ghost" onclick={closeStackEditor}>Cancel</button>
      <button class="btn-primary" onclick={saveStack} disabled={stackOpBusy || (!editingStackId && !stackName.trim())}>
        {stackOpBusy ? 'Saving…' : editingStackId ? 'Update' : 'Create'}
      </button>
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
  .empty-hint :global(code) { background: var(--surface-2); padding: 1px 5px; border-radius: 4px; font-family: 'JetBrains Mono', monospace; font-size: 11.5px; }

  .section-title { font-size: 15px; font-weight: 700; margin: 0; }
  .stack-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
  .saved-stack-list { display: flex; flex-direction: column; gap: 10px; }
  .saved-stack-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); padding: 14px 16px; }
  .saved-stack-hdr { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
  .saved-stack-name { font-weight: 700; font-size: 14px; margin-right: 8px; }
  .saved-stack-status { font-size: 11px; }
  .saved-stack-actions { display: flex; gap: 6px; }
  .saved-stack-meta { font-size: 11.5px; color: var(--mist-2); margin-top: 8px; }
  .stack-grid { display: flex; flex-direction: column; gap: 14px; }
  .stack-card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); box-shadow: var(--shadow-sm); overflow: hidden; }
  .stack-hdr { display: flex; justify-content: space-between; align-items: center; padding: 12px 18px; background: var(--surface-2); border-bottom: 1px solid var(--border); }
  .stack-name { font-family: 'Manrope', sans-serif; font-weight: 700; font-size: 14px; }
  .stack-ct { font-size: 12px; color: var(--mist); }
  .svc-row {
    display: flex; align-items: center; gap: 12px; padding: 10px 18px;
    border-bottom: 1px solid var(--border); cursor: pointer; transition: background .12s;
  }
  .svc-row:last-child { border-bottom: none; }
  .svc-row:hover { background: var(--surface-2); }
  .svc-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
  .svc-name { font-weight: 600; font-size: 13px; min-width: 160px; }
  .svc-img { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--mist); flex: 1; }
  .svc-stat { font-size: 12px; flex-shrink: 0; }

  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 7px 12px; border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600;
    transition: background .12s, border-color .12s; cursor: pointer;
  }
  .btn-ghost:hover:not(:disabled) { background: var(--surface); border-color: var(--border-strong); }
  .btn-ghost:disabled { opacity: .4; cursor: not-allowed; }
  .btn-ghost.btn-sm { padding: 5px 10px; font-size: 12px; }
  .btn-danger:hover:not(:disabled) { background: var(--red-tint); color: var(--red); border-color: var(--red); }
  .btn-primary {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 7px 14px; border-radius: var(--radius-sm); border: none;
    background: var(--accent); color: #fff; font-size: 13px; font-weight: 650;
    transition: filter .15s; cursor: pointer;
  }
  .btn-primary:hover:not(:disabled) { filter: brightness(1.07); }
  .btn-primary:disabled { opacity: .45; cursor: not-allowed; }
  .btn-primary svg, .btn-ghost svg { width: 13px; height: 13px; }

  .modal-backdrop { position: fixed; inset: 0; background: rgba(20,22,40,.45); z-index: 200; backdrop-filter: blur(2px); }
  .modal {
    position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%);
    width: 620px; max-width: 92vw; max-height: 90vh; background: var(--surface);
    border: 1px solid var(--border); border-radius: var(--radius-lg); box-shadow: var(--shadow-lg);
    z-index: 201; display: flex; flex-direction: column; overflow: hidden;
  }
  .modal-h { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid var(--border); }
  .modal-h h3 { margin: 0; font-size: 15px; }
  .modal-body { padding: 18px 20px; overflow-y: auto; display: flex; flex-direction: column; gap: 12px; }
  .modal-ft { display: flex; justify-content: flex-end; gap: 10px; padding: 14px 20px; border-top: 1px solid var(--border); }
  .field-label { font-size: 12px; font-weight: 700; color: var(--mist); }
  .text-input {
    padding: 9px 12px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface-2); color: var(--ink); font-size: 13px; outline: none;
  }
  .text-input:focus { border-color: var(--accent); background: var(--surface); }
  .code-textarea {
    font-family: 'JetBrains Mono', monospace; font-size: 12px; line-height: 1.5;
    padding: 12px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface-2); color: var(--ink); resize: vertical; min-height: 260px;
    outline: none;
  }
  .code-textarea:focus { border-color: var(--accent); background: var(--surface); }
  .err-box { padding: 10px 12px; background: var(--red-tint); color: var(--red); border-radius: var(--radius-sm); font-size: 12px; }
  .close-x {
    width: 28px; height: 28px; border: none; background: transparent; color: var(--mist);
    display: flex; align-items: center; justify-content: center; cursor: pointer;
  }
  .close-x:hover { color: var(--ink); }
  .close-x svg { width: 16px; height: 16px; }

  .pill { display: inline-block; padding: 2px 8px; border-radius: 20px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; }
  .pill-green { background: var(--green-tint); color: var(--green); }
  .pill-mist { background: var(--surface-2); color: var(--mist); }
  .pill-red { background: var(--red-tint); color: var(--red); }
</style>
