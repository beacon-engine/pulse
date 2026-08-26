<script lang="ts">
  import { stacksStore } from "../stores/stack.svelte";
  import { hostsStore } from "../stores/hosts.svelte";
  import { containersStore } from "../stores/container.svelte";
  import { toast } from "../lib/toast.svelte";
  import type { EnvVar, SavedStack } from "../types/containers";

  // Polling lifecycle tied to host readiness
  $effect(() => {
    const hostId = hostsStore.activeHostId;
    if (hostId) {
      stacksStore.startPolling(15000);
    }
    return () => {
      stacksStore.stopPolling();
    };
  });

  // ── Drill-down: containers belonging to a saved stack ───────────────────────
  let expandedStacks = $state<Record<string, boolean>>({});
  function toggleStack(id: string) {
    expandedStacks = { ...expandedStacks, [id]: !expandedStacks[id] };
  }

  function stackContainers(name: string) {
    return stacksStore.containers.filter((c) => c.stack === name);
  }

  function stateColor(s: string) {
    return s === "running"
      ? "var(--green)"
      : s === "exited"
        ? "var(--red)"
        : "var(--amber)";
  }

  // ── Editor state ──────────────────────────────────────────────────────────
  let showStackEditor = $state(false);
  let editingStack = $state<SavedStack | null>(null);
  let stackName = $state("");
  let stackContent = $state("");
  let envVars = $state<EnvVar[]>([]);
  let stackOpBusy = $state(false);
  let stackOpError = $state("");

  function openStackEditor(stack?: SavedStack) {
    editingStack = stack ?? null;
    stackName = stack ? stack.name : "";
    stackContent = stack ? stack.content : "";
    envVars = stack?.env_vars ? JSON.parse(stack.env_vars) : [];
    stackOpError = "";
    showStackEditor = true;
  }

  function closeStackEditor() {
    showStackEditor = false;
    editingStack = null;
  }

  function addEnvRow() {
    envVars = [...envVars, { key: "", value: "" }];
  }
  function removeEnvRow(i: number) {
    envVars = envVars.filter((_, idx) => idx !== i);
  }
  function updateEnvKey(i: number, v: string) {
    envVars = envVars.map((r, idx) => (idx === i ? { ...r, key: v } : r));
  }
  function updateEnvVal(i: number, v: string) {
    envVars = envVars.map((r, idx) => (idx === i ? { ...r, value: v } : r));
  }

  async function saveStack() {
    if (!hostsStore.activeHostId) return;
    stackOpBusy = true;
    stackOpError = "";
    try {
      const validEnvVars = envVars.filter((e) => e.key.trim());
      if (editingStack) {
        await stacksStore.updateStack(editingStack.id, stackContent, validEnvVars);
      } else {
        await stacksStore.createStack(stackName, stackContent);
      }
      closeStackEditor();
    } catch (e: any) {
      stackOpError = e?.response?.data ?? String(e);
    } finally {
      stackOpBusy = false;
    }
  }

  // ── Stack actions ─────────────────────────────────────────────────────────
  async function runStackUp(id: string) {
    stackOpBusy = true;
    try {
      await stacksStore.runStackUp(id);
    } catch (e: any) {
      toast.error(e?.response?.data ?? String(e));
    } finally {
      stackOpBusy = false;
    }
  }

  async function runStackDown(id: string) {
    stackOpBusy = true;
    try {
      await stacksStore.runStackDown(id);
    } catch (e: any) {
      toast.error(e?.response?.data ?? String(e));
    } finally {
      stackOpBusy = false;
    }
  }

  async function removeSavedStack(id: string) {
    if (!confirm("Delete this stack definition?")) return;
    try {
      await stacksStore.deleteStack(id);
    } catch (e: any) {
      toast.error(e?.response?.data ?? String(e));
    }
  }

  // ── Webhook token management ───────────────────────────────────────────────
  async function enableWebhook(s: SavedStack) {
    try {
      await stacksStore.generateToken(s.id);
      toast.success("Webhook enabled");
    } catch (e: any) {
      toast.error(String(e));
    }
  }

  async function rotateToken(s: SavedStack) {
    if (!confirm("Rotate the webhook token? Existing URLs will stop working.")) return;
    try {
      await stacksStore.generateToken(s.id);
      toast.success("Token rotated");
    } catch (e: any) {
      toast.error(String(e));
    }
  }

  async function disableWebhook(s: SavedStack) {
    if (!confirm("Disable this stack's webhook?")) return;
    try {
      await stacksStore.deleteToken(s.id);
      toast.info("Webhook disabled");
    } catch (e: any) {
      toast.error(String(e));
    }
  }

  function webhookUrl(s: SavedStack): string {
    if (!s.webhook_token) return "";
    return `${window.location.origin}/api/webhooks/${s.host_id}/stacks/${s.id}/redeploy?token=${s.webhook_token}`;
  }

  function copyUrl(s: SavedStack) {
    navigator.clipboard.writeText(webhookUrl(s));
    toast.success("Webhook URL copied");
  }
</script>

{#if stacksStore.loading && stacksStore.savedStacks.length === 0 && stacksStore.stacks.length === 0}
  <div class="state-msg">Loading stacks…</div>
{:else if hostsStore.activeHostId === null}
  <div class="empty-state">
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.5"
    >
      <rect x="3" y="3" width="7" height="7" rx="1.4" />
      <rect x="14" y="3" width="7" height="7" rx="1.4" />
      <rect x="3" y="14" width="7" height="7" rx="1.4" />
      <rect x="14" y="14" width="7" height="7" rx="1.4" />
    </svg>
    <p>No host selected — add one in the sidebar</p>
  </div>
{:else}
  <div class="stack-toolbar">
    <h2 class="section-title">Saved stacks</h2>
    <button class="btn-primary" onclick={() => openStackEditor()}>
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
      >
        <path d="M12 5v14M5 12h14" />
      </svg>
      New stack
    </button>
  </div>

  {#if stacksStore.savedStacks.length === 0}
    <div class="empty-state small">
      <p>No saved stacks yet</p>
      <span class="empty-hint">Create a compose definition and run it from Pulse</span>
    </div>
  {:else}
    <div class="saved-stack-list">
      {#each stacksStore.savedStacks as s (s.id)}
        <div class="saved-stack-card">
          <div class="saved-stack-hdr">
            <button
              class="stack-title-row stack-toggle"
              type="button"
              onclick={() => toggleStack(s.id)}
            >
              <svg
                class="chev"
                class:open={expandedStacks[s.id]}
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <path d="M9 6l6 6-6 6" />
              </svg>
              <span class="saved-stack-name">{s.name}</span>
              <span
                class="saved-stack-status pill {s.status === 'running'
                  ? 'pill-green'
                  : 'pill-mist'}">{s.status}</span
              >
              {#if s.env_vars && JSON.parse(s.env_vars).length > 0}
                <span
                  class="pill pill-blue"
                  title="Has environment variables"
                >
                  {JSON.parse(s.env_vars).length} env
                </span>
              {/if}
              <span class="child-count">
                {stackContainers(s.name).length} container{stackContainers(s.name).length !== 1 ? "s" : ""}
              </span>
            </button>
            <div class="saved-stack-actions">
              <button
                class="btn-ghost btn-sm"
                onclick={() => runStackUp(s.id)}
                disabled={stackOpBusy}>Start</button
              >
              <button
                class="btn-ghost btn-sm"
                onclick={() => runStackDown(s.id)}
                disabled={stackOpBusy}>Stop</button
              >
              <button
                class="btn-ghost btn-sm"
                onclick={() => openStackEditor(s)}>Edit</button
              >
              <button
                class="btn-ghost btn-sm btn-danger"
                onclick={() => removeSavedStack(s.id)}>Delete</button
              >
            </div>
          </div>

          <!-- Webhook section -->
          <div class="webhook-row">
            {#if s.webhook_token}
              <div class="webhook-enabled">
                <span class="webhook-label">
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                    width="12"
                    height="12"
                  >
                    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
                  </svg>
                  Webhook active
                </span>
                <code class="webhook-url-preview">
                  {webhookUrl(s).replace(window.location.origin, "")}
                </code>
                <div class="webhook-actions">
                  <button
                    class="btn-ghost btn-xs"
                    onclick={() => copyUrl(s)}
                    title="Copy webhook URL">Copy URL</button
                  >
                  <button
                    class="btn-ghost btn-xs"
                    onclick={() => rotateToken(s)}
                    title="Rotate token">Rotate</button
                  >
                  <button
                    class="btn-ghost btn-xs btn-danger"
                    onclick={() => disableWebhook(s)}
                    title="Disable webhook">Disable</button
                  >
                </div>
              </div>
            {:else}
              <button
                class="btn-ghost btn-xs webhook-enable-btn"
                onclick={() => enableWebhook(s)}
              >
                <svg
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  width="12"
                  height="12"
                >
                  <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                  <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
                </svg>
                Enable webhook
              </button>
            {/if}
          </div>

          <div class="saved-stack-meta">
            Updated {new Date(s.updated_at).toLocaleString()}
          </div>

          {#if expandedStacks[s.id]}
            <div class="child-list">
              {#if stackContainers(s.name).length === 0}
                <div class="child-empty">
                  No running containers — start the stack to see them here
                </div>
              {:else}
                {#each stackContainers(s.name) as c (c.id)}
                  <button
                    class="child-row"
                    type="button"
                    onclick={() => (containersStore.selectedContainer = c)}
                  >
                    <span
                      class="child-dot"
                      style="background:{stateColor(c.state)}"
                    ></span>
                    <span class="child-name">{c.name}</span>
                    <span class="child-img">{c.image}</span>
                    <span
                      class="child-stat pill {c.state === 'running'
                        ? 'pill-green'
                        : 'pill-red'}">{c.state}</span
                    >
                  </button>
                {/each}
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <h2 class="section-title" style="margin-top:28px">Detected compose projects</h2>
  {#if stacksStore.stacks.length === 0}
    <div class="empty-state small">
      <p>No compose projects detected</p>
      <span class="empty-hint">Requires <code>com.docker.compose.project</code> label</span>
    </div>
  {:else}
    <div class="stack-grid">
      {#each stacksStore.stacks as stack (stack.name)}
        <div class="stack-card">
          <div class="stack-hdr">
            <span class="stack-name">{stack.name}</span>
            <span class="stack-ct">
              {stack.services.length} service{stack.services.length !== 1 ? "s" : ""}
            </span>
          </div>
          {#each stack.services as svc (svc.id)}
            <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
            <div
              class="svc-row"
              onclick={() => (containersStore.selectedContainer = svc)}
            >
              <span
                class="svc-dot"
                style="background:{svc.state === 'running'
                  ? 'var(--green)'
                  : svc.state === 'exited'
                    ? 'var(--red)'
                    : 'var(--amber)'}"
              ></span>
              <span class="svc-name">{svc.name}</span>
              <span class="svc-img">{svc.image}</span>
              <span
                class="svc-stat pill {svc.state === 'running'
                  ? 'pill-green'
                  : 'pill-red'}">{svc.status}</span
              >
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
      <h3>
        {editingStack ? `Edit — ${editingStack.name}` : "New stack"}
      </h3>
      <button class="close-x" onclick={closeStackEditor}>
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M6 6l12 12M18 6 6 18" />
        </svg>
      </button>
    </div>
    <div class="modal-body">
      {#if !editingStack}
        <label class="field-label">Stack name</label>
        <input
          class="text-input"
          bind:value={stackName}
          placeholder="e.g. monitoring"
        />
      {/if}

      <label class="field-label">Compose YAML</label>
      <textarea
        class="code-textarea"
        bind:value={stackContent}
        rows="12"
        placeholder="services:&#10;  web:&#10;    image: nginx:latest&#10;    ports:&#10;      - 8080:80"
      ></textarea>

      <!-- Env vars editor -->
      <div class="env-section">
        <div class="env-header">
          <span class="field-label">Environment variables</span>
          <button class="btn-ghost btn-xs" onclick={addEnvRow}>+ Add</button>
        </div>
        {#if envVars.length === 0}
          <p class="env-hint">
            Variables declared here are written to a <code>.env</code> file and available as <code>{"${VAR}"}</code> in the YAML above.
          </p>
        {:else}
          <div class="env-table">
            {#each envVars as row, i (i)}
              <div class="env-row">
                <input
                  class="env-input key-input"
                  value={row.key}
                  oninput={(e) => updateEnvKey(i, (e.target as HTMLInputElement).value)}
                  placeholder="KEY"
                />
                <span class="env-eq">=</span>
                <input
                  class="env-input val-input"
                  value={row.value}
                  oninput={(e) => updateEnvVal(i, (e.target as HTMLInputElement).value)}
                  placeholder="value"
                />
                <button
                  class="env-del"
                  onclick={() => removeEnvRow(i)}
                  title="Remove"
                >
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  >
                    <path d="M6 6l12 12M18 6 6 18" />
                  </svg>
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      {#if stackOpError}<div class="err-box">{stackOpError}</div>{/if}
    </div>
    <div class="modal-ft">
      <button class="btn-ghost" onclick={closeStackEditor}>Cancel</button>
      <button
        class="btn-primary"
        onclick={saveStack}
        disabled={stackOpBusy || (!editingStack && !stackName.trim())}
      >
        {stackOpBusy ? "Saving…" : editingStack ? "Update" : "Create"}
      </button>
    </div>
  </div>
{/if}

<style>
    .state-msg {
        color: var(--mist);
        font-size: 14px;
        padding: 60px 0;
        text-align: center;
    }
    .empty-state {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 10px;
        padding: 80px 0;
        color: var(--mist);
    }
    .empty-state :global(svg) {
        width: 38px;
        height: 38px;
        color: var(--mist-2);
        opacity: 0.6;
    }
    .empty-state p {
        font-size: 14px;
        margin: 0;
    }
    .empty-state.small {
        padding: 40px 0;
    }
    .empty-state.small p {
        font-size: 13px;
    }
    .empty-hint {
        font-size: 12.5px;
        color: var(--mist-2);
    }
    .empty-hint :global(code) {
        background: var(--surface-2);
        padding: 1px 5px;
        border-radius: 4px;
        font-family: "JetBrains Mono", monospace;
        font-size: 11.5px;
    }

    .section-title {
        font-size: 15px;
        font-weight: 700;
        margin: 0;
    }
    .stack-toolbar {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 14px;
    }
    .saved-stack-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }
    .saved-stack-card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        padding: 14px 16px;
    }
    .saved-stack-hdr {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
    }
    .stack-title-row {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
    }
    .saved-stack-name {
        font-weight: 700;
        font-size: 14px;
    }
    .saved-stack-status {
        font-size: 11px;
    }
    .saved-stack-actions {
        display: flex;
        gap: 6px;
        flex-shrink: 0;
    }
    .saved-stack-meta {
        font-size: 11.5px;
        color: var(--mist-2);
        margin-top: 10px;
    }

    /* Drill-down toggle + child container rows (shared look with Swarm) */
    .stack-toggle {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
        background: none;
        border: none;
        padding: 0;
        margin: 0;
        cursor: pointer;
        text-align: left;
        flex: 1;
        min-width: 0;
    }
    .chev {
        width: 14px;
        height: 14px;
        color: var(--mist-2);
        transition: transform 0.15s ease;
        flex-shrink: 0;
    }
    .chev.open {
        transform: rotate(90deg);
    }
    .child-count {
        font-size: 11.5px;
        color: var(--mist-2);
    }
    .child-list {
        margin-top: 12px;
        border-top: 1px solid var(--border);
        padding-top: 4px;
    }
    .child-empty {
        padding: 12px 4px;
        font-size: 12.5px;
        color: var(--mist-2);
    }
    .child-row {
        display: flex;
        align-items: center;
        gap: 12px;
        width: 100%;
        padding: 9px 8px;
        border: none;
        background: none;
        cursor: pointer;
        text-align: left;
        border-radius: 7px;
        transition: background 0.12s;
    }
    .child-row:hover {
        background: var(--surface-2);
    }
    .child-dot {
        width: 7px;
        height: 7px;
        border-radius: 50%;
        flex-shrink: 0;
    }
    .child-name {
        font-weight: 600;
        font-size: 13px;
        min-width: 150px;
    }
    .child-img {
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
        color: var(--mist);
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .child-stat {
        font-size: 11px;
        flex-shrink: 0;
    }

    /* Webhook row */
    .webhook-row {
        margin-top: 10px;
        padding-top: 10px;
        border-top: 1px solid var(--border);
    }
    .webhook-enable-btn {
        color: var(--mist);
        font-size: 11.5px;
    }
    .webhook-enabled {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-wrap: wrap;
    }
    .webhook-label {
        display: inline-flex;
        align-items: center;
        gap: 5px;
        font-size: 11.5px;
        font-weight: 700;
        color: var(--green);
        white-space: nowrap;
    }
    .webhook-url-preview {
        font-family: "JetBrains Mono", monospace;
        font-size: 10.5px;
        color: var(--mist);
        background: var(--surface-2);
        padding: 3px 8px;
        border-radius: 4px;
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        max-width: 340px;
    }
    .webhook-actions {
        display: flex;
        gap: 6px;
    }

    .stack-grid {
        display: flex;
        flex-direction: column;
        gap: 14px;
    }
    .stack-card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        box-shadow: var(--shadow-sm);
        overflow: hidden;
    }
    .stack-hdr {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 12px 18px;
        background: var(--surface-2);
        border-bottom: 1px solid var(--border);
    }
    .stack-name {
        font-family: "Manrope", sans-serif;
        font-weight: 700;
        font-size: 14px;
    }
    .stack-ct {
        font-size: 12px;
        color: var(--mist);
    }
    .svc-row {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 18px;
        border-bottom: 1px solid var(--border);
        cursor: pointer;
        transition: background 0.12s;
    }
    .svc-row:last-child {
        border-bottom: none;
    }
    .svc-row:hover {
        background: var(--surface-2);
    }
    .svc-dot {
        width: 7px;
        height: 7px;
        border-radius: 50%;
        flex-shrink: 0;
    }
    .svc-name {
        font-weight: 600;
        font-size: 13px;
        min-width: 160px;
    }
    .svc-img {
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
        color: var(--mist);
        flex: 1;
    }
    .svc-stat {
        font-size: 12px;
        flex-shrink: 0;
    }

    /* Buttons */
    .btn-ghost {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 7px 12px;
        border-radius: var(--radius-sm);
        border: 1px solid var(--border);
        background: var(--surface-2);
        color: var(--ink);
        font-size: 13px;
        font-weight: 600;
        transition:
            background 0.12s,
            border-color 0.12s;
        cursor: pointer;
    }
    .btn-ghost:hover:not(:disabled) {
        background: var(--surface);
        border-color: var(--border-strong);
    }
    .btn-ghost:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }
    .btn-ghost.btn-sm {
        padding: 5px 10px;
        font-size: 12px;
    }
    .btn-ghost.btn-xs {
        padding: 3px 8px;
        font-size: 11.5px;
    }
    .btn-danger:hover:not(:disabled) {
        background: var(--red-tint);
        color: var(--red);
        border-color: var(--red);
    }
    .btn-primary {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 7px 14px;
        border-radius: var(--radius-sm);
        border: none;
        background: var(--accent);
        color: #fff;
        font-size: 13px;
        font-weight: 650;
        transition: filter 0.15s;
        cursor: pointer;
    }
    .btn-primary:hover:not(:disabled) {
        filter: brightness(1.07);
    }
    .btn-primary:disabled {
        opacity: 0.45;
        cursor: not-allowed;
    }
    .btn-primary :global(svg),
    .btn-ghost :global(svg) {
        width: 13px;
        height: 13px;
    }

    /* Modal */
    .modal-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(20, 22, 40, 0.45);
        z-index: 200;
        backdrop-filter: blur(2px);
    }
    .modal {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        width: 680px;
        max-width: 92vw;
        max-height: 90vh;
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--radius-lg);
        box-shadow: var(--shadow-lg);
        z-index: 201;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }
    .modal-h {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 16px 20px;
        border-bottom: 1px solid var(--border);
    }
    .modal-h h3 {
        margin: 0;
        font-size: 15px;
    }
    .modal-body {
        padding: 18px 20px;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 12px;
    }
    .modal-ft {
        display: flex;
        justify-content: flex-end;
        gap: 10px;
        padding: 14px 20px;
        border-top: 1px solid var(--border);
    }
    .field-label {
        font-size: 12px;
        font-weight: 700;
        color: var(--mist);
    }
    .text-input {
        padding: 9px 12px;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        background: var(--surface-2);
        color: var(--ink);
        font-size: 13px;
        outline: none;
    }
    .text-input:focus {
        border-color: var(--accent);
        background: var(--surface);
    }
    .code-textarea {
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
        line-height: 1.5;
        padding: 12px;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        background: var(--surface-2);
        color: var(--ink);
        resize: vertical;
        min-height: 220px;
        outline: none;
    }
    .code-textarea:focus {
        border-color: var(--accent);
        background: var(--surface);
    }
    .err-box {
        padding: 10px 12px;
        background: var(--red-tint);
        color: var(--red);
        border-radius: var(--radius-sm);
        font-size: 12px;
    }
    .close-x {
        width: 28px;
        height: 28px;
        border: none;
        background: transparent;
        color: var(--mist);
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
    }
    .close-x:hover {
        color: var(--ink);
    }
    .close-x :global(svg) {
        width: 16px;
        height: 16px;
    }

    /* Env vars editor */
    .env-section {
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        overflow: hidden;
    }
    .env-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 8px 12px;
        background: var(--surface-2);
        border-bottom: 1px solid var(--border);
    }
    .env-hint {
        font-size: 12px;
        color: var(--mist-2);
        margin: 10px 12px;
    }
    .env-hint code {
        background: var(--surface-2);
        padding: 1px 4px;
        border-radius: 3px;
        font-family: "JetBrains Mono", monospace;
        font-size: 11px;
    }
    .env-table {
        display: flex;
        flex-direction: column;
    }
    .env-row {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 6px 10px;
        border-bottom: 1px solid var(--border);
    }
    .env-row:last-child {
        border-bottom: none;
    }
    .env-input {
        padding: 5px 8px;
        border: 1px solid var(--border);
        border-radius: 4px;
        background: var(--surface);
        color: var(--ink);
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
        outline: none;
    }
    .env-input:focus {
        border-color: var(--accent);
    }
    .key-input {
        width: 140px;
        flex-shrink: 0;
    }
    .val-input {
        flex: 1;
    }
    .env-eq {
        color: var(--mist-2);
        font-size: 13px;
        font-weight: 600;
        padding: 0 2px;
    }
    .env-del {
        width: 24px;
        height: 24px;
        border: none;
        background: transparent;
        color: var(--mist-2);
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        border-radius: 4px;
        flex-shrink: 0;
    }
    .env-del:hover {
        background: var(--red-tint);
        color: var(--red);
    }
    .env-del :global(svg) {
        width: 12px;
        height: 12px;
    }

    /* Pills */
    .pill {
        display: inline-block;
        padding: 2px 8px;
        border-radius: 20px;
        font-size: 11px;
        font-weight: 700;
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }
    .pill-green {
        background: var(--green-tint);
        color: var(--green);
    }
    .pill-mist {
        background: var(--surface-2);
        color: var(--mist);
    }
    .pill-red {
        background: var(--red-tint);
        color: var(--red);
    }
    .pill-blue {
        background: var(--accent-tint);
        color: var(--accent-ink);
    }
</style>
