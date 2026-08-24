<script lang="ts">
    import { onMount } from "svelte";
    import { api } from "../lib/api";
    import { getAppState } from "../lib/appContext";
    import type { SwarmService, SwarmTask } from "../types/containers";

    const app = getAppState();

    let updatingService = $state("");

    // ── Drill-down: tasks (containers) of a service ─────────────────────────────
    let expandedSvc = $state<Record<string, boolean>>({});
    let svcTasks = $state<Record<string, SwarmTask[]>>({});
    let svcTasksLoading = $state<Record<string, boolean>>({});
    let svcTasksError = $state<Record<string, string>>({});

    async function viewService(svc: SwarmService) {
        // const open = !expandedSvc[id];
        // expandedSvc = { ...expandedSvc, [id]: open };
        // if (open && !svcTasks[id]) await loadTasks(id);
        app.selectedSwarmService = svc;
    }

    async function loadTasks(id: string) {
        if (!app.activeHostId) return;
        svcTasksLoading = { ...svcTasksLoading, [id]: true };
        svcTasksError = { ...svcTasksError, [id]: "" };
        try {
            svcTasks = {
                ...svcTasks,
                [id]: await api.getSwarmServiceTasks(app.activeHostId, id),
            };
        } catch (e: any) {
            svcTasksError = {
                ...svcTasksError,
                [id]: e?.response?.data ?? String(e),
            };
        } finally {
            svcTasksLoading = { ...svcTasksLoading, [id]: false };
        }
    }

    // Open the container drawer for a task's backing container (local-node tasks).
    function openTask(svc: SwarmService, t: SwarmTask) {
        if (!t.container_id) return;
        app.selectedContainer = {
            id: t.container_id,
            name: `${svc.name}.${t.slot}`,
            image: t.image,
            state: t.state,
            status: t.state,
        };
    }

    function taskColor(s: string) {
        if (s === "running") return "var(--green)";
        if (
            s === "shutdown" ||
            s === "failed" ||
            s === "rejected" ||
            s === "orphaned"
        )
            return "var(--red)";
        return "var(--amber)";
    }
    // Map<serviceId, token | null>
    let serviceTokens = $state<Record<string, string | null>>({});

    async function loadTokens() {
        if (!app.activeHostId) return;
        const entries = await Promise.all(
            app.swarmServices.map(async (svc) => {
                try {
                    const r = await api.getWebhookToken(
                        app.activeHostId!,
                        "swarm",
                        svc.id,
                    );
                    return [svc.id, r.token] as [string, string | null];
                } catch {
                    return [svc.id, null] as [string, string | null];
                }
            }),
        );
        serviceTokens = Object.fromEntries(entries);
    }

    async function forceUpdate(serviceId: string) {
        if (!app.activeHostId) return;
        updatingService = serviceId;
        try {
            const token = serviceTokens[serviceId] ?? undefined;
            await api.webhookUpdateSwarm(app.activeHostId, serviceId, token);
            app.toast("Service update triggered");
            await app.refreshSwarm();
        } catch (e: any) {
            app.toast(e?.response?.data ?? String(e), "err");
        } finally {
            updatingService = "";
        }
    }

    async function scaleService(serviceId: string, count: number) {
        if (!app.activeHostId) return;
        updatingService = serviceId;
        try {
            const token = serviceTokens[serviceId] ?? undefined;
            await api.webhookScaleSwarm(
                app.activeHostId,
                serviceId,
                count,
                token,
            );
            app.toast("Service update triggered");
            await app.refreshSwarm();
        } catch (e: any) {
            app.toast(e?.response?.data ?? String(e), "err");
        } finally {
            updatingService = "";
        }
    }

    async function enableServiceWebhook(serviceId: string) {
        if (!app.activeHostId) return;
        try {
            const r = await api.generateWebhookToken(
                app.activeHostId,
                "swarm",
                serviceId,
            );
            serviceTokens = { ...serviceTokens, [serviceId]: r.token };
            app.toast("Webhook enabled");
        } catch (e: any) {
            app.toast(String(e), "err");
        }
    }

    async function rotateServiceToken(serviceId: string) {
        if (
            !app.activeHostId ||
            !confirm("Rotate token? Existing webhook URLs will stop working.")
        )
            return;
        try {
            const r = await api.generateWebhookToken(
                app.activeHostId,
                "swarm",
                serviceId,
            );
            serviceTokens = { ...serviceTokens, [serviceId]: r.token };
            app.toast("Token rotated");
        } catch (e: any) {
            app.toast(String(e), "err");
        }
    }

    async function disableServiceWebhook(serviceId: string) {
        if (!app.activeHostId || !confirm("Disable this webhook?")) return;
        try {
            await api.deleteWebhookToken(app.activeHostId, "swarm", serviceId);
            serviceTokens = { ...serviceTokens, [serviceId]: null };
            app.toast("Webhook disabled", "warn");
        } catch (e: any) {
            app.toast(String(e), "err");
        }
    }

    function swarmWebhookUrl(
        hostId: string,
        serviceId: string,
        token: string,
    ): string {
        return `${window.location.origin}/api/webhooks/${hostId}/swarm/${serviceId}/update?token=${token}`;
    }

    onMount(() => loadTokens());

    // Reload tokens only when the *set of services* changes — not on every 10s
    // poll (which replaces the array with the same ids and caused table jank).
    let lastSig = "";
    $effect(() => {
        const sig = app.swarmServices.map((s) => s.id).join(",");
        if (sig && sig !== lastSig) {
            lastSig = sig;
            loadTokens();
        }
    });
</script>

{#if app.swarmLoading}
    <div class="state-msg">Loading Swarm…</div>
{:else if app.swarmError}
    <div class="empty-state">
        <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.5"
            ><circle cx="12" cy="5" r="2" /><circle
                cx="19"
                cy="9"
                r="2"
            /><circle cx="19" cy="15" r="2" /><circle
                cx="12"
                cy="19"
                r="2"
            /><circle cx="5" cy="15" r="2" /><circle cx="5" cy="9" r="2" /><path
                d="M7 9l4-2.5M13 9l4 2.5M17 15l-4 2.5M11 15l-4-2.5"
            /></svg
        >
        <p>Could not load Swarm</p>
        <span class="empty-hint">{app.swarmError}</span>
    </div>
{:else}
    <div class="kpi-row">
        <div class="kpi-card">
            <div class="kpi-top">
                <span class="kpi-lbl">Services</span>
                <div class="kpi-icon" style="background:var(--accent-tint)">
                    <svg
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="var(--accent)"
                        stroke-width="2"
                        ><path d="M4 7h16M4 12h16M4 17h16" /></svg
                    >
                </div>
            </div>
            <div class="kpi-val">{app.swarmServices.length}</div>
            <div class="kpi-trend" style="color:var(--mist)">
                swarm services
            </div>
        </div>
        <div class="kpi-card">
            <div class="kpi-top">
                <span class="kpi-lbl">Running</span>
                <div class="kpi-icon" style="background:var(--green-tint)">
                    <svg
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="var(--green)"
                        stroke-width="2"><path d="M5 13l4 4 10-10" /></svg
                    >
                </div>
            </div>
            <div class="kpi-val">
                {app.swarmServices.reduce((a, s) => a + s.running, 0)}
            </div>
            <div class="kpi-trend" style="color:var(--green)">
                running replicas
            </div>
        </div>
        <div class="kpi-card">
            <div class="kpi-top">
                <span class="kpi-lbl">Nodes</span>
                <div class="kpi-icon" style="background:var(--amber-tint)">
                    <svg
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="var(--amber)"
                        stroke-width="2"
                        ><circle cx="12" cy="5" r="2" /><circle
                            cx="19"
                            cy="9"
                            r="2"
                        /><circle cx="5" cy="9" r="2" /></svg
                    >
                </div>
            </div>
            <div class="kpi-val">{app.swarmNodes.length}</div>
            <div class="kpi-trend" style="color:var(--mist)">swarm nodes</div>
        </div>
    </div>

    <div class="k8s-grid">
        <div class="table-wrap">
            <table>
                <thead
                    ><tr
                        ><th>Service</th><th>Image</th><th>Replicas</th><th
                            >State</th
                        ><th>Webhook</th><th></th></tr
                    ></thead
                >
                <tbody>
                    {#each app.swarmServices as svc (svc.id)}
                        {@const token = serviceTokens[svc.id] ?? null}
                        <tr class:row-open={expandedSvc[svc.id]}>
                            <td class="cell-mono fw">
                                <button
                                    class="svc-name-toggle"
                                    type="button"
                                    onclick={() => viewService(svc)}
                                >
                                    {svc.name}
                                </button>
                            </td>
                            <td class="cell-mono cell-mist"
                                >{svc.image.split("@")[0]}</td
                            >
                            <td class="cell-mono"
                                >{svc.running}/{svc.replicas}</td
                            >
                            <td
                                ><span class="pill pill-mist"
                                    >{svc.desired_state || "running"}</span
                                ></td
                            >
                            <td>
                                {#if token}
                                    <div class="webhook-cell">
                                        <span class="wh-active">active</span>
                                        <code class="wh-url-sm"
                                            >{swarmWebhookUrl(
                                                app.activeHostId!,
                                                svc.id,
                                                token,
                                            ).replace(
                                                window.location.origin,
                                                "",
                                            )}</code
                                        >
                                        <button
                                            class="icon-copy"
                                            title="Copy URL"
                                            onclick={() =>
                                                navigator.clipboard.writeText(
                                                    swarmWebhookUrl(
                                                        app.activeHostId!,
                                                        svc.id,
                                                        token!,
                                                    ),
                                                )}
                                        >
                                            <svg
                                                viewBox="0 0 24 24"
                                                fill="none"
                                                stroke="currentColor"
                                                stroke-width="2"
                                                ><rect
                                                    x="9"
                                                    y="9"
                                                    width="13"
                                                    height="13"
                                                    rx="2"
                                                /><path
                                                    d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"
                                                /></svg
                                            >
                                        </button>
                                        <button
                                            class="icon-copy"
                                            title="Rotate token"
                                            onclick={() =>
                                                rotateServiceToken(svc.id)}
                                        >
                                            <svg
                                                viewBox="0 0 24 24"
                                                fill="none"
                                                stroke="currentColor"
                                                stroke-width="2"
                                                ><path
                                                    d="M3 12a9 9 0 1 0 2.6-6.4L3 8"
                                                /><path d="M3 3v5h5" /></svg
                                            >
                                        </button>
                                        <button
                                            class="icon-del"
                                            title="Disable webhook"
                                            onclick={() =>
                                                disableServiceWebhook(svc.id)}
                                        >
                                            <svg
                                                viewBox="0 0 24 24"
                                                fill="none"
                                                stroke="currentColor"
                                                stroke-width="2"
                                                ><path
                                                    d="M6 6l12 12M18 6 6 18"
                                                /></svg
                                            >
                                        </button>
                                    </div>
                                {:else}
                                    <button
                                        class="enable-wh-btn"
                                        onclick={() =>
                                            enableServiceWebhook(svc.id)}
                                        >Enable</button
                                    >
                                {/if}
                            </td>
                            <td>
                                <!-- <div> -->
                                <button
                                    class="update-btn"
                                    onclick={() => forceUpdate(svc.id)}
                                    disabled={updatingService === svc.id}
                                >
                                    {#if updatingService === svc.id}
                                        <svg
                                            class="spin"
                                            viewBox="0 0 24 24"
                                            fill="none"
                                            stroke="currentColor"
                                            stroke-width="2"
                                            ><path
                                                d="M3 12a9 9 0 1 0 2.6-6.4L3 8"
                                            /><path d="M3 3v5h5" /></svg
                                        >
                                    {:else}
                                        <svg
                                            viewBox="0 0 24 24"
                                            fill="none"
                                            stroke="currentColor"
                                            stroke-width="2"
                                            ><path
                                                d="M3 12a9 9 0 1 0 2.6-6.4L3 8"
                                            /><path d="M3 3v5h5" /></svg
                                        >
                                    {/if}
                                    Force update
                                </button>
                                <!-- </div> -->
                                <!-- <div> -->
                                <button
                                    class="update-btn"
                                    onclick={() => forceUpdate(svc.id)}
                                    disabled={updatingService === svc.id}
                                >
                                    {#if updatingService === svc.id}
                                        <svg
                                            class="spin"
                                            viewBox="0 0 24 24"
                                            fill="none"
                                            stroke="currentColor"
                                            stroke-width="2"
                                            ><path
                                                d="M3 12a9 9 0 1 0 2.6-6.4L3 8"
                                            /><path d="M3 3v5h5" /></svg
                                        >
                                    {:else}
                                        <svg
                                            viewBox="0 0 24 24"
                                            fill="none"
                                            stroke="currentColor"
                                            stroke-width="2"
                                            ><path
                                                d="M3 12a9 9 0 1 0 2.6-6.4L3 8"
                                            /><path d="M3 3v5h5" /></svg
                                        >
                                    {/if}
                                    Scale service
                                </button>
                                <!-- </div> -->
                            </td>
                        </tr>
                        <!-- {#if expandedSvc[svc.id]}
                            <tr class="task-tr">
                                <td colspan="6">
                                    {#if svcTasksLoading[svc.id]}
                                        <div class="task-msg">
                                            Loading tasks…
                                        </div>
                                    {:else if svcTasksError[svc.id]}
                                        <div class="task-msg err">
                                            {svcTasksError[svc.id]}
                                        </div>
                                    {:else if (svcTasks[svc.id]?.length ?? 0) === 0}
                                        <div class="task-msg">
                                            No tasks for this service
                                        </div>
                                    {:else}
                                        <div class="task-list">
                                            {#each svcTasks[svc.id] as t (t.id)}
                                                <button
                                                    class="task-row"
                                                    type="button"
                                                    onclick={() =>
                                                        openTask(svc, t)}
                                                    disabled={!t.container_id}
                                                    title={t.container_id
                                                        ? "Open container"
                                                        : "No container yet"}
                                                >
                                                    <span
                                                        class="task-dot"
                                                        style="background:{taskColor(
                                                            t.state,
                                                        )}"
                                                    ></span>
                                                    <span class="task-name"
                                                        >{svc.name}.{t.slot}</span
                                                    >
                                                    <span class="task-cid"
                                                        >{t.container_id
                                                            ? t.container_id.slice(
                                                                  0,
                                                                  12,
                                                              )
                                                            : "—"}</span
                                                    >
                                                    <span class="task-img"
                                                        >{t.image.split(
                                                            "@",
                                                        )[0]}</span
                                                    >
                                                    <span
                                                        class="pill {t.state ===
                                                        'running'
                                                            ? 'pill-green'
                                                            : 'pill-red'}"
                                                        >{t.state}</span
                                                    >
                                                </button>
                                            {/each}
                                        </div>
                                    {/if}
                                </td>
                            </tr>
                        {/if} -->
                    {/each}
                </tbody>
            </table>
            {#if app.swarmServices.length === 0}
                <div class="empty-state small"><p>No services found</p></div>
            {/if}
        </div>

        <div class="table-wrap">
            <table>
                <thead
                    ><tr
                        ><th>Node</th><th>Role</th><th>State</th><th
                            >Availability</th
                        ></tr
                    ></thead
                >
                <tbody>
                    {#each app.swarmNodes as node (node.id)}
                        <tr>
                            <td class="cell-mono">{node.hostname}</td>
                            <td class="cell-mono cell-mist">{node.role}</td>
                            <td
                                ><span class="pill pill-mist">{node.state}</span
                                ></td
                            >
                            <td
                                ><span
                                    class="pill {node.availability === 'active'
                                        ? 'pill-green'
                                        : 'pill-red'}">{node.availability}</span
                                ></td
                            >
                        </tr>
                    {/each}
                </tbody>
            </table>
            {#if app.swarmNodes.length === 0}
                <div class="empty-state small"><p>No nodes found</p></div>
            {/if}
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
    .table-wrap {
        overflow-x: auto;
    }
    table {
        width: 100%;
        border-collapse: collapse;
    }
    thead th {
        text-align: left;
        padding: 10px 18px;
        font-size: 11.5px;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: var(--mist-2);
        font-weight: 700;
        border-bottom: 1px solid var(--border);
        background: var(--surface-2);
        white-space: nowrap;
    }
    :global(tbody td) {
        padding: 11px 18px;
        border-bottom: 1px solid var(--border);
        font-size: 13.3px;
        vertical-align: middle;
    }
    :global(tbody tr:last-child td) {
        border-bottom: none;
    }
    :global(tbody tr:hover td) {
        background: var(--surface-2);
    }
    .cell-mono {
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
    }
    .cell-mist {
        color: var(--mist);
    }
    .fw {
        font-weight: 700;
    }
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
    .pill-red {
        background: var(--red-tint);
        color: var(--red);
    }
    .pill-mist {
        background: var(--surface-2);
        color: var(--mist);
    }

    .kpi-row {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        gap: 16px;
        margin-bottom: 22px;
    }
    .kpi-card {
        background: var(--surface);
        border: 1px solid var(--border);
        border-radius: var(--radius-md);
        padding: 16px;
        box-shadow: var(--shadow-sm);
    }
    .kpi-top {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 8px;
    }
    .kpi-lbl {
        font-size: 12px;
        font-weight: 700;
        color: var(--mist-2);
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }
    .kpi-icon {
        width: 32px;
        height: 32px;
        border-radius: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
    }
    .kpi-icon :global(svg) {
        width: 16px;
        height: 16px;
    }
    .kpi-val {
        font-size: 26px;
        font-weight: 800;
        color: var(--ink);
        line-height: 1;
    }
    .kpi-trend {
        font-size: 12px;
        margin-top: 6px;
    }

    .k8s-grid {
        display: grid;
        grid-template-columns: 3fr 1fr;
        gap: 20px;
    }
    .k8s-grid .table-wrap {
        min-height: 200px;
    }

    /* Webhook cell */
    .webhook-cell {
        display: flex;
        align-items: center;
        gap: 6px;
    }
    .wh-url-sm {
        font-family: "JetBrains Mono", monospace;
        font-size: 10px;
        color: var(--mist);
        max-width: 240px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .icon-copy {
        width: 22px;
        height: 22px;
        border-radius: 5px;
        border: 1px solid var(--border);
        background: transparent;
        color: var(--mist-2);
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        flex-shrink: 0;
        transition: all 0.12s;
    }
    .icon-copy:hover {
        background: var(--accent-tint);
        color: var(--accent);
        border-color: transparent;
    }
    .icon-copy svg {
        width: 11px;
        height: 11px;
    }
    .icon-del {
        width: 22px;
        height: 22px;
        border-radius: 5px;
        border: 1px solid var(--border);
        background: transparent;
        color: var(--mist-2);
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        flex-shrink: 0;
        transition: all 0.12s;
    }
    .icon-del:hover {
        background: var(--red-tint);
        color: var(--red);
        border-color: var(--red);
    }
    .icon-del svg {
        width: 11px;
        height: 11px;
    }
    .wh-active {
        font-size: 9.5px;
        font-weight: 800;
        text-transform: uppercase;
        background: var(--green-tint);
        color: var(--green);
        padding: 2px 6px;
        border-radius: 20px;
        flex-shrink: 0;
    }
    .enable-wh-btn {
        padding: 3px 9px;
        font-size: 11.5px;
        font-weight: 600;
        border-radius: var(--radius-sm);
        border: 1px solid var(--accent);
        background: var(--accent-tint);
        color: var(--accent-ink);
        cursor: pointer;
        transition: all 0.12s;
    }
    .enable-wh-btn:hover {
        background: var(--accent);
        color: #fff;
    }

    /* Force update button */
    .update-btn {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        padding: 5px 11px;
        border-radius: var(--radius-sm);
        border: 1px solid var(--border);
        background: var(--surface);
        color: var(--ink);
        font-size: 12px;
        font-weight: 600;
        cursor: pointer;
        white-space: nowrap;
        transition: all 0.12s;
    }
    .update-btn:hover:not(:disabled) {
        border-color: var(--accent);
        color: var(--accent);
        background: var(--accent-tint);
    }
    .update-btn:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }
    .update-btn svg {
        width: 13px;
        height: 13px;
    }
    .spin {
        animation: spin 1s linear infinite;
    }
    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }

    /* Service drill-down */
    .svc-name-toggle {
        display: inline-flex;
        align-items: center;
        gap: 7px;
        background: none;
        border: none;
        padding: 0;
        margin: 0;
        cursor: pointer;
        font-family: "JetBrains Mono", monospace;
        font-size: 12px;
        font-weight: 700;
        color: var(--ink);
    }
    .svc-name-toggle:hover {
        color: var(--accent);
    }
    .chev {
        width: 13px;
        height: 13px;
        color: var(--mist-2);
        transition: transform 0.15s ease;
        flex-shrink: 0;
    }
    .chev.open {
        transform: rotate(90deg);
    }
    :global(tr.row-open td) {
        background: var(--surface-2);
    }

    .task-tr :global(td) {
        padding: 0 18px 12px !important;
        background: var(--surface-2);
    }
    .task-list {
        display: flex;
        flex-direction: column;
        border: 1px solid var(--border);
        border-radius: var(--radius-sm);
        background: var(--surface);
        overflow: hidden;
    }
    .task-row {
        display: flex;
        align-items: center;
        gap: 12px;
        width: 100%;
        padding: 9px 12px;
        border: none;
        border-bottom: 1px solid var(--border);
        background: none;
        cursor: pointer;
        text-align: left;
        transition: background 0.12s;
    }
    .task-row:last-child {
        border-bottom: none;
    }
    .task-row:hover:not(:disabled) {
        background: var(--surface-2);
    }
    .task-row:disabled {
        cursor: default;
        opacity: 0.7;
    }
    .task-dot {
        width: 7px;
        height: 7px;
        border-radius: 50%;
        flex-shrink: 0;
    }
    .task-name {
        font-weight: 600;
        font-size: 12.5px;
        min-width: 140px;
    }
    .task-cid {
        font-family: "JetBrains Mono", monospace;
        font-size: 11.5px;
        color: var(--mist-2);
        min-width: 100px;
    }
    .task-img {
        font-family: "JetBrains Mono", monospace;
        font-size: 11.5px;
        color: var(--mist);
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .task-msg {
        font-size: 12.5px;
        color: var(--mist);
        padding: 8px 2px;
    }
    .task-msg.err {
        color: var(--red);
    }
</style>
