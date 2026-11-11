<script lang="ts">
  import type { Environment } from '../types/environments';

  let {
    hosts,
    activeId,
    total,
    onselect,
    onadd,
    ondelete,
    onrename,
    onseeall,
  }: {
    hosts: Environment[];
    activeId: string | null;
    total: number;
    onselect: (host: Environment) => void;
    onadd: () => void;
    ondelete: (id: string) => void;
    onrename: (id: string, name: string) => void;
    onseeall: () => void;
  } = $props();

  let editingId = $state<string | null>(null);
  let editName = $state('');

  function startEdit(e: MouseEvent, host: Environment) {
    e.stopPropagation();
    editingId = host.id;
    editName = host.name;
  }

  function commitEdit() {
    if (editingId && editName.trim()) {
      onrename(editingId, editName.trim());
    }
    editingId = null;
  }

  function onRenameKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') { e.preventDefault(); commitEdit(); }
    if (e.key === 'Escape') { editingId = null; }
  }
</script>

<div class="host-section">
  <div class="section-label">Hosts</div>

  {#if total > hosts.length}
    <button class="see-all" onclick={onseeall}>See all hosts ({total})</button>
  {:else}
    <button class="see-all" onclick={onseeall}>Manage hosts</button>
  {/if}

  {#each hosts as host (host.id)}
    <div class="host-row" class:active={host.id === activeId}>
      <button class="host-btn" onclick={() => onselect(host)}>
        <span class="dot" class:local={host.is_local} class:offline={host.status === 'offline'}
          title={host.status === 'offline' ? 'Agent offline' : undefined}></span>
        {#if editingId === host.id}
          <!-- svelte-ignore a11y-autofocus -->
          <input
            class="rename-input"
            bind:value={editName}
            autofocus
            onclick={(e) => e.stopPropagation()}
            onkeydown={onRenameKeydown}
            onblur={commitEdit}
          />
        {:else}
          <span class="host-name">{host.name}</span>
          {#if host.is_local}<span class="local-tag">local</span>{/if}
        {/if}
      </button>

      <button class="icon-btn edit-btn" title="Rename" onclick={(e) => startEdit(e, host)}>
        ✎
      </button>
      {#if !host.is_local}
        <button
          class="icon-btn del-btn"
          title="Remove host"
          onclick={(e) => { e.stopPropagation(); ondelete(host.id); }}
        >✕</button>
      {/if}
    </div>
  {/each}

  <button class="add-btn" onclick={onadd}>
    <span>+</span> Add host
  </button>
</div>

<style>
  .host-section { margin-top: 4px; }
  .section-label {
    font-size: 10.5px; font-weight: 700; letter-spacing: .08em;
    text-transform: uppercase; color: var(--mist-2); padding: 14px 12px 6px;
  }
  .host-row {
    display: flex; align-items: center; gap: 2px;
    border-radius: var(--radius-sm); padding: 2px 4px 2px 12px;
    transition: background .15s;
  }
  .host-row:hover, .host-row.active { background: var(--surface-2); }
  .host-btn {
    flex: 1; display: flex; align-items: center; gap: 8px;
    background: none; border: none; padding: 7px 0;
    color: var(--mist); font-size: 13.5px; font-weight: 600;
    cursor: pointer; text-align: left; min-width: 0;
  }
  .host-row.active .host-btn { color: var(--ink); }
  .host-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
  .dot {
    width: 7px; height: 7px; border-radius: 50%;
    background: var(--mist-2); flex-shrink: 0;
  }
  .dot.local { background: var(--accent); }
  .host-row.active .dot { background: var(--accent); }
  .local-tag {
    font-size: 10px; font-weight: 700; text-transform: uppercase;
    letter-spacing: .06em; color: var(--mist-2); flex-shrink: 0;
  }
  .icon-btn {
    background: none; border: none; cursor: pointer;
    font-size: 12px; padding: 4px 5px; border-radius: 6px;
    flex-shrink: 0; transition: color .15s, background .15s;
    opacity: 0; color: var(--mist-2);
  }
  .host-row:hover .icon-btn { opacity: 1; }
  .edit-btn:hover { color: var(--accent); background: var(--accent-tint); }
  .del-btn:hover  { color: var(--red);    background: var(--red-tint); }
  .rename-input {
    flex: 1; background: var(--surface-2); border: 1px solid var(--accent);
    border-radius: 6px; color: var(--ink); font-size: 13px;
    padding: 2px 7px; outline: none; min-width: 0; font-family: inherit;
  }
  .add-btn {
    display: flex; align-items: center; gap: 6px;
    margin: 6px 12px 0; padding: 7px 10px;
    background: none; border: 1px dashed var(--border-strong); border-radius: var(--radius-sm);
    color: var(--mist-2); font-size: 12.5px; font-weight: 600;
    cursor: pointer; width: calc(100% - 24px);
    transition: border-color .15s, color .15s;
  }
  .add-btn:hover { border-color: var(--accent); color: var(--accent); }
  .see-all {
    display: block; width: auto; margin: 8px 12px 0; padding: 6px 10px;
    background: none; border: none; border-radius: var(--radius-sm);
    color: var(--accent-ink); font-size: 12px; font-weight: 650; cursor: pointer;
    transition: background .12s;
  }
  .see-all:hover { background: var(--accent-tint); }
  .dot.offline { background: transparent; box-shadow: inset 0 0 0 1.5px currentColor; opacity: .5; }
</style>
