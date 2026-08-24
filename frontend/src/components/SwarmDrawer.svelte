<script lang="ts">
    import { onMount } from "svelte";
    import { api } from "../lib/api";
    import type { SwarmService, SwarmTask } from "../types/containers";
    import { getAppState } from "../lib/appContext";

    let {
        hostId,
        swarm,
        onclose,
        onrefresh,
    }: {
        hostId: string;
        swarm: SwarmService;
        onclose: () => void;
        onrefresh: () => void;
        // onopentask: (svc: SwarmService, task: SwarmTask) => void;
    } = $props();

    const app = getAppState();

    type Tab = "overview" | "tasks" | "logs";
    let tab = $state<Tab>("overview");

    // ── Detail & Tasks State ──────────────────────────────────────────────────
    let detail = $state<SwarmService | null>(swarm);
    let tasks = $state<SwarmTask[]>([]);
    let loading = $state(true);
    let error = $state("");
    let actionBusy = $state(false);

    // ── Logs State ────────────────────────────────────────────────────────────
    let logs = $state("");
    let logsLoading = $state(false);
    let logsTail = $state(200);

    async function loadData() {
        loading = true;
        error = "";
        try {
            const svcTasks = await api.getSwarmServiceTasks(hostId, swarm.id);
            tasks = svcTasks;
        } catch (e: any) {
            error = e?.response?.data ?? String(e);
        } finally {
            loading = false;
        }
    }

    async function onopentask(svc: SwarmService, task: SwarmTask) {
      const tsk = {
        id: task.container_id,
        name: `${svc.name}.${task.slot}`,
        image: task.image,
        status: task.state,
        state: task.state,
        stack: svc.name,
      };
      app.selectedContainer = tsk;
    }

    async function loadLogs() {
        // logsLoading = true;
        // try {
        //   logs = await api.webhookScaleSwarm(hostId, swarm.id, logsTail);
        // } catch (e: any) {
        //   logs = `[Error loading logs: ${e?.response?.data ?? String(e)}]`;
        // } finally {
        //   logsLoading = false;
        // }
        return;
    }

    async function triggerForceUpdate() {
        actionBusy = true;
        try {
            await api.webhookUpdateSwarm(hostId, swarm.id);
            onrefresh();
            await loadData();
        } catch (e: any) {
            error = e?.response?.data ?? String(e);
        } finally {
            actionBusy = false;
        }
    }

    $effect(() => {
        if (tab === "logs") {
            loadLogs();
        }
    });

    onMount(() => {
        loadData();
    });

    const TABS: Tab[] = ["overview", "tasks", "logs"];
    function tabLabel(t: Tab) {
        return t.charAt(0).toUpperCase() + t.slice(1);
    }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="backdrop" onclick={onclose}></div>

<div class="drawer">
    <!-- Header -->
    <div class="dh">
        <div class="dh-left">
            <span
                class="state-dot"
                class:running={swarm.running > 0 &&
                    swarm.running === swarm.replicas}
                class:degraded={swarm.running > 0 &&
                    swarm.running < swarm.replicas}
                class:stopped={swarm.running === 0}
            ></span>
            <div>
                <div class="dh-name">{swarm.name}</div>
                <div class="dh-id">{swarm.id.slice(0, 12)}</div>
            </div>
        </div>
        <div class="dh-actions">
            <button
                class="hbtn"
                disabled={actionBusy}
                onclick={triggerForceUpdate}
                title="Force restart all service tasks"
            >
                <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                >
                    <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" />
                    <path d="M3 3v5h5" />
                </svg>
                Force Restart
            </button>
            <button class="close-x" onclick={onclose} aria-label="Close">
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
    </div>

    <!-- Tabs -->
    <div class="tabs">
        {#each TABS as t}
            <button
                class="tab-btn"
                class:active={tab === t}
                onclick={() => (tab = t)}
            >
                {tabLabel(t)}
                {#if t === "tasks" && tasks.length > 0}
                    <span class="tab-badge">{tasks.length}</span>
                {/if}
            </button>
        {/each}
    </div>

    <!-- Body -->
    <div class="body">
        {#if loading && tab !== "logs"}
            <div class="state-msg">Loading service details…</div>
        {:else if error}
            <div class="err-box">{error}</div>
        {:else if tab === "overview"}
            {@const current = detail ?? swarm}
            <div class="kv-list">
                <div class="kv-row">
                    <span class="k">Service ID</span>
                    <span class="v mono">{current.id}</span>
                </div>
                <div class="kv-row">
                    <span class="k">Image</span>
                    <span class="v mono">{current.image}</span>
                </div>
                <div class="kv-row">
                    <span class="k">Replicas</span>
                    <span class="v">
                        <span
                            class="pill {current.running === current.replicas
                                ? 'pill-green'
                                : current.running > 0
                                  ? 'pill-amber'
                                  : 'pill-red'}"
                        >
                            {current.running} / {current.replicas} running
                        </span>
                    </span>
                </div>
                <div class="kv-row">
                    <span class="k">Desired State</span>
                    <span class="v mono">{current.desired_state}</span>
                </div>
            </div>
        {:else if tab === "tasks"}
            {#if tasks.length === 0}
                <div class="state-msg">
                    No active tasks found for this service.
                </div>
            {:else}
                <div class="task-list">
                    {#each tasks as t (t.id)}
                        <div
                            class="task-row"
                            class:clickable={!!t.container_id}
                            onclick={() => onopentask(swarm, t)}
                            role="button"
                            tabindex="0"
                            onkeydown={(e) =>
                                e.key === "Enter" &&
                                onopentask(swarm, t)}
                        >
                            <div class="task-left">
                                <span
                                    class="state-dot"
                                    class:running={t.state === "running"}
                                    class:stopped={t.state !== "running"}
                                ></span>
                                <div>
                                    <div class="task-title">
                                        <span class="task-slot"
                                            >#{t.slot || 1}</span
                                        >
                                        <span class="task-name"
                                            >{swarm.name}.{t.slot || 1}</span
                                        >
                                    </div>
                                    <div class="task-sub mono">
                                        node: {t.node_id} <br /> container:
                                        {t.container_id
                                            ? t.container_id
                                            : "none"}
                                    </div>
                                </div>
                            </div>

                            <div class="task-right">
                                <span
                                    class="state-badge"
                                    class:running={t.state === "running"}
                                    >{t.state}</span
                                >
                                {#if t.container_id}
                                    <svg
                                        class="arrow-icon"
                                        viewBox="0 0 24 24"
                                        fill="none"
                                        stroke="currentColor"
                                        stroke-width="2"
                                    >
                                        <path d="M9 18l6-6-6-6" />
                                    </svg>
                                {/if}
                            </div>
                        </div>
                    {/each}
                </div>
            {/if}
        {:else if tab === "logs"}
            <div class="log-toolbar">
                <select
                    class="select-sm"
                    bind:value={logsTail}
                    onchange={loadLogs}
                >
                    <option value={100}>Last 100 lines</option>
                    <option value={200}>Last 200 lines</option>
                    <option value={500}>Last 500 lines</option>
                    <option value={1000}>Last 1000 lines</option>
                </select>
                <button
                    class="btn-ghost btn-sm"
                    onclick={loadLogs}
                    disabled={logsLoading}
                >
                    {logsLoading ? "Refreshing…" : "Refresh logs"}
                </button>
            </div>
            <pre class="terminal">{logs ||
                    (logsLoading
                        ? "Fetching logs…"
                        : "No logs available.")}</pre>
        {/if}
    </div>
</div>

<style>
    .backdrop {
        position: fixed;
        inset: 0;
        background: rgba(20, 22, 40, 0.38);
        z-index: 50;
        backdrop-filter: blur(1px);
    }
    .drawer {
        position: fixed;
        top: 0;
        right: 0;
        bottom: 0;
        width: 980px;
        max-width: 96vw;
        background: var(--surface);
        border-left: 1px solid var(--border);
        box-shadow: var(--shadow-lg);
        z-index: 51;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .dh {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 16px 20px;
        border-bottom: 1px solid var(--border);
        flex-shrink: 0;
        gap: 12px;
    }
    .dh-left {
        display: flex;
        align-items: center;
        gap: 12px;
        min-width: 0;
    }
    .state-dot {
        width: 10px;
        height: 10px;
        border-radius: 50%;
        flex-shrink: 0;
        background: var(--mist-2);
    }
    .state-dot.running {
        background: var(--green);
        box-shadow: 0 0 0 3px rgba(28, 154, 99, 0.2);
    }
    .state-dot.degraded {
        background: var(--amber);
    }
    .state-dot.stopped {
        background: var(--red);
    }

    .dh-name {
        font-family: "Manrope", sans-serif;
        font-weight: 700;
        font-size: 15px;
        color: var(--ink);
    }
    .dh-id {
        font-family: "JetBrains Mono", monospace;
        font-size: 11.5px;
        color: var(--mist-2);
        margin-top: 2px;
    }
    .dh-actions {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .hbtn {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 6px 12px;
        border-radius: var(--radius-sm);
        border: 1px solid var(--border);
        background: var(--surface-2);
        color: var(--ink);
        font-size: 12.5px;
        font-weight: 600;
        cursor: pointer;
        transition: all 0.12s;
    }
    .hbtn svg {
        width: 13px;
        height: 13px;
    }
    .hbtn:hover:not(:disabled) {
        background: var(--surface);
        border-color: var(--border-strong);
    }
    .hbtn:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }

    .close-x {
        width: 30px;
        height: 30px;
        border-radius: 8px;
        border: none;
        background: var(--surface-2);
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--mist);
        cursor: pointer;
        transition:
            background 0.12s,
            color 0.12s;
    }
    .close-x:hover {
        background: var(--border);
        color: var(--ink);
    }
    .close-x svg {
        width: 14px;
        height: 14px;
    }

    .tabs {
        display: flex;
        border-bottom: 1px solid var(--border);
        padding: 0 16px;
        flex-shrink: 0;
        background: var(--surface);
    }
    .tab-btn {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 10px 4px;
        margin-right: 18px;
        border: none;
        background: none;
        font-weight: 650;
        font-size: 13px;
        color: var(--mist);
        border-bottom: 2px solid transparent;
        cursor: pointer;
        white-space: nowrap;
        transition:
            color 0.15s,
            border-color 0.15s;
        margin-bottom: -1px;
    }
    .tab-btn:hover {
        color: var(--ink);
    }
    .tab-btn.active {
        color: var(--accent-ink);
        border-color: var(--accent);
    }
    .tab-badge {
        background: var(--surface-2);
        border: 1px solid var(--border);
        font-size: 10.5px;
        font-weight: 700;
        padding: 1px 6px;
        border-radius: 20px;
        color: var(--mist);
    }
    .tab-btn.active .tab-badge {
        background: var(--accent-tint);
        color: var(--accent-ink);
        border-color: var(--accent-tint);
    }

    .body {
        flex: 1;
        overflow-y: auto;
        padding: 20px;
        display: flex;
        flex-direction: column;
    }
    .state-msg {
        color: var(--mist-2);
        font-size: 13.5px;
        padding: 30px 0;
        text-align: center;
    }
    .err-box {
        background: var(--red-tint);
        color: var(--red);
        border-radius: var(--radius-sm);
        padding: 10px 14px;
        font-size: 13px;
        margin-bottom: 12px;
    }

    .kv-list {
        display: flex;
        flex-direction: column;
    }
    .kv-row {
        display: flex;
        gap: 16px;
        padding: 11px 0;
        border-bottom: 1px solid var(--border);
        align-items: center;
    }
    .k {
        font-size: 12px;
        font-weight: 700;
        color: var(--mist);
        min-width: 130px;
    }
    .v {
        font-size: 13px;
        color: var(--ink);
        flex: 1;
    }
    .mono {
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
    }

    .pill {
        display: inline-block;
        padding: 2px 9px;
        border-radius: 20px;
        font-size: 11.5px;
        font-weight: 700;
    }
    .pill-green {
        background: var(--green-tint);
        color: var(--green);
    }
    .pill-amber {
        background: var(--amber-tint);
        color: var(--amber);
    }
    .pill-red {
        background: var(--red-tint);
        color: var(--red);
    }

    /* Tasks */
    .task-list {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }
    .task-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 12px 14px;
        border-radius: var(--radius-sm);
        background: var(--surface-2);
        border: 1px solid var(--border);
        transition: all 0.12s;
    }
    .task-row.clickable {
        cursor: pointer;
    }
    .task-row.clickable:hover {
        background: var(--surface);
        border-color: var(--accent);
    }
    .task-left {
        display: flex;
        align-items: center;
        gap: 12px;
    }
    .task-title {
        display: flex;
        align-items: center;
        gap: 6px;
    }
    .task-slot {
        font-weight: 800;
        font-size: 12px;
        color: var(--mist-2);
        font-family: "JetBrains Mono", monospace;
    }
    .task-name {
        font-weight: 700;
        font-size: 13.5px;
        color: var(--ink);
    }
    .task-sub {
        font-size: 11px;
        color: var(--mist-2);
        margin-top: 3px;
    }
    .task-right {
        display: flex;
        align-items: center;
        gap: 8px;
    }
    .state-badge {
        font-size: 11px;
        font-weight: 700;
        text-transform: uppercase;
        padding: 2px 7px;
        border-radius: 4px;
        background: var(--surface);
        color: var(--mist);
    }
    .state-badge.running {
        background: var(--green-tint);
        color: var(--green);
    }
    .arrow-icon {
        width: 14px;
        height: 14px;
        color: var(--mist-2);
    }

    /* Logs */
    .log-toolbar {
        display: flex;
        gap: 8px;
        align-items: center;
        margin-bottom: 12px;
        flex-shrink: 0;
    }
    .select-sm {
        padding: 5px 10px;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        font-size: 12.5px;
        background: var(--surface-2);
        color: var(--ink);
    }
    .btn-ghost {
        padding: 6px 12px;
        border-radius: var(--radius-sm);
        border: 1px solid var(--border);
        background: var(--surface-2);
        color: var(--ink);
        font-size: 12.5px;
        font-weight: 600;
        cursor: pointer;
        transition: background 0.12s;
    }
    .btn-ghost:hover:not(:disabled) {
        background: var(--surface);
    }
    .btn-ghost:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }
    .terminal {
        flex: 1;
        background: #15171f;
        border-radius: var(--radius-md);
        padding: 14px 16px;
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
        line-height: 1.6;
        color: #c7cbda;
        white-space: pre-wrap;
        overflow-y: auto;
        min-height: 320px;
        margin: 0;
    }
</style>
