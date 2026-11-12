<script lang="ts">
  import { onMount, untrack } from "svelte";
  import { kubernetesStore, type K8sSection } from "../stores/kubernetes.svelte";
  import { hostsStore } from "../stores/hosts.svelte";
  import { api } from "../lib/api";
  import LogsModal from "../components/LogsModal.svelte";
  import Pagination from "../components/Pagination.svelte";
  import K8sContainerRow from "../components/K8sContainerRow.svelte";
  import K8sPodDrawer from "../components/K8sPodDrawer.svelte";
  import K8sWebhookModal from "../components/K8sWebhookModal.svelte";
  import type { K8sPod, K8sWorkload, PodContainer } from "../types/containers";
  import { fmtAge } from "../utils/fmt";
  import {
    connectionOf,
    containerStatus,
    fmtMemory,
    podReadyCount,
    podRestarts,
    podStatus,
  } from "../utils/k8s";

  // Reload as soon as the environment changes, not at the next poll.
  $effect(() => {
    if (!hostsStore.activeHostId) return;
    untrack(() => kubernetesStore.startPolling(15000));
    return () => kubernetesStore.stopPolling();
  });

  onMount(() => {
    // Keep the Connected / Offline badge current.
    const statusTimer = setInterval(() => {
      if (document.visibilityState === "visible") void hostsStore.refreshActive();
    }, 15000);
    return () => clearInterval(statusTimer);
  });

  type Tab = "workloads" | "pods" | "containers" | "services" | "nodes" | "events";
  const TABS: { id: Tab; label: string }[] = [
    { id: "workloads", label: "Workloads" },
    { id: "pods", label: "Pods" },
    { id: "containers", label: "Containers" },
    { id: "services", label: "Services" },
    { id: "nodes", label: "Nodes" },
    { id: "events", label: "Events" },
  ];
  const TAB_KEY = "pulse.k8s.tab";
  const PER_PAGE = 25;

  let tab = $state<Tab>(readTab());
  /** "" = all namespaces */
  let namespace = $state("");
  let search = $state("");
  /** Set from a workload's "Pods" button: show only that workload's pods. */
  let ownerFilter = $state<K8sWorkload | null>(null);
  let containerFilter = $state<"all" | "running" | "problem">("all");
  let warningsOnly = $state(false);
  let page = $state(1);

  let expanded = $state<Record<string, boolean>>({});
  let busy = $state<Record<string, boolean>>({});
  let logsFor = $state<{ pod: K8sPod; container?: string } | null>(null);
  let inspecting = $state<{ key: string; focus?: string } | null>(null);

  function readTab(): Tab {
    try {
      const t = localStorage.getItem(TAB_KEY) as Tab | null;
      if (t && TABS.some((x) => x.id === t)) return t;
    } catch {}
    return "workloads";
  }

  function setTab(t: Tab) {
    tab = t;
    page = 1;
    try {
      localStorage.setItem(TAB_KEY, t);
    } catch {}
  }

  // Any filter change goes back to the first page.
  $effect(() => {
    void [namespace, search, ownerFilter, containerFilter, warningsOnly];
    page = 1;
  });

  // ── Cluster and connection ───────────────────────────────────────────────
  let env = $derived(hostsStore.activeHost);
  let conn = $derived(connectionOf(env));
  let runtime = $derived(
    hostsStore.activeCapabilities?.runtimes.find((r) => r.kind === "kubernetes") ?? null,
  );

  // ── Filtering ────────────────────────────────────────────────────────────
  let q = $derived(search.trim().toLowerCase());
  const has = (...fields: (string | undefined)[]) =>
    !q || fields.some((f) => f?.toLowerCase().includes(q));
  const inNs = (ns: string) => !namespace || ns === namespace;

  let workloads = $derived(
    kubernetesStore.workloads.filter(
      (w) => inNs(w.namespace) && has(w.name, w.namespace, w.kind, ...w.images),
    ),
  );

  let scopedPods = $derived(
    kubernetesStore.pods.filter(
      (p) =>
        inNs(p.namespace) &&
        (!ownerFilter ||
          (p.namespace === ownerFilter.namespace &&
            p.owner?.kind === ownerFilter.kind &&
            p.owner?.name === ownerFilter.name)),
    ),
  );

  let pods = $derived(
    scopedPods.filter((p) =>
      has(p.name, p.namespace, p.node, p.owner?.name, p.reason, ...p.containers.flatMap((c) => [c.name, c.image])),
    ),
  );

  let containers = $derived(
    scopedPods
      .flatMap((pod) => pod.containers.map((c) => ({ pod, c })))
      .filter(({ pod, c }) => has(c.name, c.image, pod.name, pod.namespace))
      .filter(({ c }) => {
        if (containerFilter === "running") return c.state === "running";
        if (containerFilter === "problem") return !c.ready || c.restart_count > 0;
        return true;
      }),
  );

  let services = $derived(
    kubernetesStore.services.filter(
      (s) => inNs(s.namespace) && has(s.name, s.namespace, s.type, s.cluster_ip, ...(s.external ?? [])),
    ),
  );

  let nodes = $derived(
    kubernetesStore.nodes.filter((n) => has(n.name, n.internal_ip, n.os_image, ...n.roles)),
  );

  let events = $derived(
    kubernetesStore.events.filter(
      (e) =>
        inNs(e.namespace) &&
        (!warningsOnly || e.type === "Warning") &&
        has(e.reason, e.message, e.object_name, e.object_kind),
    ),
  );

  function paged<T>(items: T[]): T[] {
    return items.slice((page - 1) * PER_PAGE, page * PER_PAGE);
  }

  // ── Summary ──────────────────────────────────────────────────────────────
  let nsPods = $derived(kubernetesStore.pods.filter((p) => inNs(p.namespace)));
  let nsWorkloads = $derived(kubernetesStore.workloads.filter((w) => inNs(w.namespace)));
  let allContainers = $derived(nsPods.flatMap((p) => p.containers));
  let healthyWorkloads = $derived(nsWorkloads.filter((w) => w.ready >= w.replicas).length);
  let runningPods = $derived(nsPods.filter((p) => podStatus(p).pill === "pill-green").length);
  let failingPods = $derived(nsPods.filter((p) => podStatus(p).pill === "pill-red").length);
  let readyContainers = $derived(allContainers.filter((c) => c.ready).length);
  let restarts = $derived(allContainers.reduce((n, c) => n + c.restart_count, 0));
  let readyNodes = $derived(kubernetesStore.nodes.filter((n) => n.status === "Ready").length);
  let warnings = $derived(
    kubernetesStore.events.filter((e) => e.type === "Warning" && inNs(e.namespace)).length,
  );
  let podsPerNode = $derived(
    kubernetesStore.pods.reduce<Record<string, number>>((acc, p) => {
      if (p.node && p.status !== "Succeeded" && p.status !== "Failed") acc[p.node] = (acc[p.node] ?? 0) + 1;
      return acc;
    }, {}),
  );
  let podCount = $derived(
    kubernetesStore.pods.reduce<Record<string, number>>((acc, p) => {
      acc[p.namespace] = (acc[p.namespace] ?? 0) + 1;
      return acc;
    }, {}),
  );

  function tabCount(t: Tab): number {
    switch (t) {
      case "workloads": return workloads.length;
      case "pods": return pods.length;
      case "containers": return containers.length;
      case "services": return services.length;
      case "nodes": return nodes.length;
      case "events": return events.length;
    }
  }

  // ── Helpers ──────────────────────────────────────────────────────────────
  const key = (p: { namespace: string; name: string }) => `${p.namespace}/${p.name}`;
  const wkey = (w: K8sWorkload) => `${w.kind}/${w.namespace}/${w.name}`;

  let inspectedPod = $derived(
    inspecting ? kubernetesStore.pods.find((p) => key(p) === inspecting!.key) ?? null : null,
  );
  let inspectedEvents = $derived(
    inspectedPod
      ? kubernetesStore.events.filter(
          (e) => e.object_kind === "Pod" && e.object_name === inspectedPod!.name && e.namespace === inspectedPod!.namespace,
        )
      : [],
  );

  function scalable(w: K8sWorkload) {
    return w.kind === "Deployment" || w.kind === "StatefulSet";
  }

  function workloadPill(w: K8sWorkload) {
    if (w.replicas === 0) return "pill-mist";
    return w.ready >= w.replicas ? "pill-green" : w.ready === 0 ? "pill-red" : "pill-amber";
  }

  function readyPct(w: K8sWorkload) {
    return w.replicas === 0 ? 0 : Math.min(100, Math.round((w.ready / w.replicas) * 100));
  }

  function showPods(w: K8sWorkload) {
    ownerFilter = w;
    search = "";
    setTab("pods");
  }

  function openLogs(pod: K8sPod, c?: PodContainer) {
    if (!c && pod.containers.length > 1) {
      // A multi-container pod has no single log; pick the container in the drawer.
      inspecting = { key: key(pod) };
      return;
    }
    logsFor = { pod, container: c?.name };
  }

  async function scaleTo(w: K8sWorkload, n: number) {
    if (n < 0) return;
    if (n === 0 && !confirm(`Scale ${w.kind} "${w.name}" to 0? All of its pods will stop.`)) return;
    busy = { ...busy, [wkey(w)]: true };
    try {
      await kubernetesStore.scale(w, n);
    } finally {
      busy = { ...busy, [wkey(w)]: false };
    }
  }

  let webhookFor = $state<K8sWorkload | null>(null);

  async function restart(w: K8sWorkload) {
    if (!confirm(`Rolling restart of ${w.kind} "${w.name}"? Pods are replaced one by one.`)) return;
    busy = { ...busy, [wkey(w)]: true };
    try {
      await kubernetesStore.restart(w);
    } finally {
      busy = { ...busy, [wkey(w)]: false };
    }
  }

  /** Delete a pod so its controller replaces it: the Kubernetes way to restart its containers. */
  async function replacePod(p: K8sPod) {
    const others = p.containers.length > 1 ? ` All ${p.containers.length} of its containers restart.` : "";
    if (!confirm(`Replace pod "${p.name}"? It is deleted and ${p.owner?.kind ?? "its controller"} ${p.owner?.name ?? ""} starts a new one.${others}`))
      return;
    busy = { ...busy, [key(p)]: true };
    try {
      await kubernetesStore.deletePod(p.namespace, p.name);
      if (inspecting?.key === key(p)) inspecting = null;
    } catch {
      // toasted by the store
    } finally {
      busy = { ...busy, [key(p)]: false };
    }
  }

  function fetchPodLogs(pod: K8sPod, container?: string) {
    return (tail: number) =>
      api.getK8sPodLogs(hostsStore.activeHostId!, pod.namespace, pod.name, container, tail);
  }

  function svcPorts(s: (typeof services)[number]) {
    if (!s.ports.length) return "—";
    return s.ports
      .map((p) => `${p.port}${p.node_port ? `:${p.node_port}` : ""}→${p.target_port}/${p.protocol}`)
      .join(", ");
  }
</script>

{#snippet sectionNotice(section: K8sSection)}
  {#if kubernetesStore.sectionErrors[section]}
    <div class="notice">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9" /><path d="M12 8v5M12 16.5v.5" /></svg>
      <div>
        <strong>Couldn't load {section}.</strong>
        {kubernetesStore.sectionErrors[section]}
        <div class="notice-hint">
          The agent's ClusterRole needs read access to {section}. Re-apply the ClusterRole from
          <code>deploy/kubernetes/agent-push.yaml</code> (or <code>agent-pull.yaml</code>), and
          update the agent if it's older than this page.
        </div>
      </div>
    </div>
  {/if}
{/snippet}

{#snippet empty(text: string)}
  <div class="empty-row">{text}</div>
{/snippet}

{#if kubernetesStore.loading && kubernetesStore.pods.length === 0}
  <div class="state-msg">Loading Kubernetes…</div>
{:else if kubernetesStore.error && kubernetesStore.pods.length === 0}
  <div class="empty-state">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
      <path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z" />
      <path d="M12 22V12M20 6.5L12 12M4 6.5L12 12" />
    </svg>
    <p>Could not load Kubernetes</p>
    <span class="empty-hint">{kubernetesStore.error}</span>
    <button class="btn-ghost" onclick={() => kubernetesStore.refresh()}>Retry</button>
  </div>
{:else}
  <!-- Cluster -->
  <div class="cluster">
    <div class="cluster-icon">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
        <path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z" />
        <path d="M12 22V12M20 6.5L12 12M4 6.5L12 12" />
      </svg>
    </div>
    <div class="cluster-main">
      <div class="cluster-name">
        {env?.name ?? "Cluster"}
        <span class="pill {env?.status === 'offline' ? 'pill-red' : 'pill-green'}">
          {#if env?.status !== "offline"}<span class="dot"></span>{/if}
          {env?.status === "offline" ? "Offline" : "Connected"}
        </span>
      </div>
      <div class="cluster-meta">
        {#if runtime}<span>Kubernetes {runtime.version}</span>{/if}
        {#if runtime?.endpoint}<span class="mono" title="API server">{runtime.endpoint}</span>{/if}
        {#if kubernetesStore.nodes.length}<span>{kubernetesStore.nodes.length} node{kubernetesStore.nodes.length === 1 ? "" : "s"}</span>{/if}
        {#if env?.last_seen}<span>seen {fmtAge(env.last_seen)} ago</span>{/if}
      </div>
    </div>
    <div class="conn conn-{conn.mode}" title={conn.detail}>
      {#if conn.mode === "push"}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 19V5M5 12l7-7 7 7" /></svg>
      {:else if conn.mode === "pull"}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 5v14M19 12l-7 7-7-7" /></svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3" /><path d="M12 3v3M12 18v3M3 12h3M18 12h3" /></svg>
      {/if}
      <div>
        <div class="conn-label">{conn.label}</div>
        <div class="conn-sub">
          {conn.mode === "push" ? "cluster dials the gateway" : conn.mode === "pull" ? (env?.agent_addr ?? "gateway dials the agent") : "built into the gateway"}
        </div>
      </div>
    </div>
  </div>

  <!-- KPIs -->
  <div class="kpi-row">
    <button class="kpi-card" onclick={() => setTab("workloads")}>
      <span class="kpi-lbl">Workloads</span>
      <div class="kpi-val">{healthyWorkloads}<span class="kpi-of"> / {nsWorkloads.length}</span></div>
      <div class="kpi-trend" class:warn={healthyWorkloads < nsWorkloads.length}>
        {healthyWorkloads < nsWorkloads.length ? `${nsWorkloads.length - healthyWorkloads} not fully ready` : "all healthy"}
      </div>
    </button>
    <button class="kpi-card" onclick={() => setTab("pods")}>
      <span class="kpi-lbl">Pods</span>
      <div class="kpi-val">{runningPods}<span class="kpi-of"> / {nsPods.length}</span></div>
      <div class="kpi-trend" class:bad={failingPods > 0}>
        {failingPods ? `${failingPods} failing` : "running and ready"}
      </div>
    </button>
    <button class="kpi-card" onclick={() => setTab("containers")}>
      <span class="kpi-lbl">Containers</span>
      <div class="kpi-val">{readyContainers}<span class="kpi-of"> / {allContainers.length}</span></div>
      <div class="kpi-trend" class:warn={restarts > 0}>{restarts} restart{restarts === 1 ? "" : "s"}</div>
    </button>
    <button class="kpi-card" onclick={() => setTab("nodes")}>
      <span class="kpi-lbl">Nodes</span>
      <div class="kpi-val">
        {#if kubernetesStore.sectionErrors.nodes}—{:else}{readyNodes}<span class="kpi-of"> / {kubernetesStore.nodes.length}</span>{/if}
      </div>
      <div class="kpi-trend">ready</div>
    </button>
    <button class="kpi-card" onclick={() => { warningsOnly = true; setTab("events"); }}>
      <span class="kpi-lbl">Warnings</span>
      <div class="kpi-val" class:bad-val={warnings > 0}>{kubernetesStore.sectionErrors.events ? "—" : warnings}</div>
      <div class="kpi-trend">recent events</div>
    </button>
  </div>

  <!-- Tabs and filters -->
  <div class="bar">
    <div class="tabs" role="tablist">
      {#each TABS as t (t.id)}
        <button role="tab" class="tab" class:active={tab === t.id} aria-selected={tab === t.id} onclick={() => setTab(t.id)}>
          {t.label}
          <span class="tab-badge" class:tab-badge-red={t.id === "events" && warnings > 0}>{tabCount(t.id)}</span>
        </button>
      {/each}
    </div>
    <div class="filters">
      <input class="filter-input" placeholder="Filter…" bind:value={search} aria-label="Filter" />
      {#if tab !== "nodes"}
        <select class="ns-select" bind:value={namespace} aria-label="Namespace">
          <option value="">All namespaces</option>
          {#each kubernetesStore.namespaces as ns (ns.name)}
            <option value={ns.name}>{ns.name} ({podCount[ns.name] ?? 0})</option>
          {/each}
        </select>
      {/if}
      <button class="icon-btn" title="Refresh" aria-label="Refresh" onclick={() => kubernetesStore.refresh()}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" /><path d="M3 3v5h5" /></svg>
      </button>
    </div>
  </div>

  {#if ownerFilter && (tab === "pods" || tab === "containers")}
    <div class="chip-row">
      <span class="filter-chip">
        {ownerFilter.kind} <strong>{ownerFilter.namespace}/{ownerFilter.name}</strong>
        <button aria-label="Clear workload filter" onclick={() => (ownerFilter = null)}>×</button>
      </span>
    </div>
  {/if}

  <div class="panel">
    {#if tab === "workloads"}
      {@render sectionNotice("workloads")}
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Name</th><th>Namespace</th><th>Ready</th><th>Replicas</th><th>Age</th><th></th></tr>
          </thead>
          <tbody>
            {#each paged(workloads) as w (wkey(w))}
              <tr class="row">
                <td>
                  <div class="row-name">
                    {w.name}
                    <span class="kind-tag">{w.kind}</span>
                    {#if w.protected}<span class="pulse-tag" title={w.protected_reason}>Pulse</span>{/if}
                  </div>
                  <div class="row-sub" title={w.images.join(", ")}>{w.images.join(", ")}</div>
                </td>
                <td class="cell-mono cell-mist">{w.namespace}</td>
                <td>
                  <div class="ready-cell">
                    <span class="pill {workloadPill(w)}">{w.ready}/{w.replicas}</span>
                    <span class="bar-track"><span class="bar-fill {workloadPill(w)}" style="width:{readyPct(w)}%"></span></span>
                  </div>
                </td>
                <td>
                  {#if scalable(w)}
                    <div class="stepper" title={w.protected ? `Protected: ${w.protected_reason}` : "Scale"}>
                      <button type="button" aria-label="Scale down" disabled={w.protected || busy[wkey(w)] || w.replicas === 0}
                        onclick={() => scaleTo(w, w.replicas - 1)}>−</button>
                      <span>{w.replicas}</span>
                      <button type="button" aria-label="Scale up" disabled={w.protected || busy[wkey(w)]}
                        onclick={() => scaleTo(w, w.replicas + 1)}>+</button>
                    </div>
                  {:else}
                    <span class="cell-mist small" title="A DaemonSet runs one pod per matching node">one per node</span>
                  {/if}
                </td>
                <td class="cell-mist">{fmtAge(w.created)}</td>
                <td>
                  <div class="row-actions">
                    <button class="act-btn" title="Show its pods" onclick={() => showPods(w)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z" /></svg>
                    </button>
                    <button class="act-btn" title={w.protected ? `Protected: ${w.protected_reason}` : "Rolling restart"}
                      disabled={w.protected || busy[wkey(w)]} onclick={() => restart(w)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" /><path d="M3 3v5h5" /></svg>
                    </button>
                    <button class="act-btn" title="Restart webhook for CI/CD" aria-label="Restart webhook" onclick={() => (webhookFor = w)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" /><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" /></svg>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if workloads.length === 0}{@render empty("No workloads match")}{/if}
      <Pagination {page} perPage={PER_PAGE} total={workloads.length} onpage={(p) => (page = p)} />

    {:else if tab === "pods"}
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>Pod</th><th>Status</th><th>Ready</th><th>Restarts</th><th>Controlled by</th><th>Node</th><th>Age</th><th></th></tr>
          </thead>
          <tbody>
            {#each paged(pods) as pod (key(pod))}
              {@const st = podStatus(pod)}
              <tr class="row" class:row-open={expanded[key(pod)]}>
                <td>
                  <button class="pod-toggle" type="button" onclick={() => (expanded = { ...expanded, [key(pod)]: !expanded[key(pod)] })}>
                    <svg class="chev" class:open={expanded[key(pod)]} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 6l6 6-6 6" /></svg>
                    <span class="min0">
                      <span class="row-name block">
                        {pod.name}
                        {#if pod.protected}<span class="pulse-tag" title={pod.protected_reason}>Pulse</span>{/if}
                      </span>
                      <span class="ns-sub">{pod.namespace}</span>
                    </span>
                  </button>
                </td>
                <td><span class="pill {st.pill}">{#if st.live}<span class="dot"></span>{/if}{st.label}</span></td>
                <td class="cell-mono">{podReadyCount(pod)}</td>
                <td><span class="restarts" class:hot={podRestarts(pod) > 0}>{podRestarts(pod)}</span></td>
                <td class="cell-mist small">{#if pod.owner}<span class="kind-tag">{pod.owner.kind}</span> {pod.owner.name}{:else}—{/if}</td>
                <td class="cell-mono cell-mist">{pod.node ?? "—"}</td>
                <td class="cell-mist">{fmtAge(pod.created)}</td>
                <td>
                  <div class="row-actions">
                    <button class="act-btn" title={pod.containers.length > 1 ? "Logs (pick a container)" : "Logs"} onclick={() => openLogs(pod)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="4" width="18" height="16" rx="2" /><path d="M6 9h12M6 13h9M6 17h6" /></svg>
                    </button>
                    <button class="act-btn" title="Inspect" onclick={() => (inspecting = { key: key(pod) })}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7" /><path d="M21 21l-4.3-4.3" /></svg>
                    </button>
                    <button class="act-btn danger" title={pod.protected ? `Protected: ${pod.protected_reason}` : "Delete pod"}
                      disabled={busy[key(pod)] || pod.protected} onclick={() => replacePod(pod)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6" /><path d="M19 6l-1 14H6L5 6" /></svg>
                    </button>
                  </div>
                </td>
              </tr>
              {#if expanded[key(pod)]}
                <tr class="sub-tr">
                  <td colspan="8">
                    {#if pod.containers.length === 0}
                      <div class="sub-msg">No container statuses reported yet.</div>
                    {:else}
                      <table class="sub-table">
                        <thead><tr><th>Container</th><th>Status</th><th>Ports</th><th>Restarts</th><th></th></tr></thead>
                        <tbody>
                          {#each pod.containers as c (c.name)}
                            <K8sContainerRow {pod} container={c} showPod={false}
                              onlogs={() => openLogs(pod, c)}
                              oninspect={() => (inspecting = { key: key(pod), focus: c.name })}
                              onrestart={() => replacePod(pod)} />
                          {/each}
                        </tbody>
                      </table>
                    {/if}
                  </td>
                </tr>
              {/if}
            {/each}
          </tbody>
        </table>
      </div>
      {#if pods.length === 0}{@render empty("No pods match")}{/if}
      <Pagination {page} perPage={PER_PAGE} total={pods.length} onpage={(p) => (page = p)} />

    {:else if tab === "containers"}
      <div class="panel-tools">
        <div class="seg">
          {#each [["all", "All"], ["running", "Running"], ["problem", "Restarting / not ready"]] as [v, label] (v)}
            <button class:active={containerFilter === v} onclick={() => (containerFilter = v as typeof containerFilter)}>{label}</button>
          {/each}
        </div>
        <span class="tools-hint">Kubernetes restarts containers itself. Restarting one here replaces its pod.</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>Name</th><th>Status</th><th>Pod</th><th>Ports</th><th>Restarts</th><th></th></tr></thead>
          <tbody>
            {#each paged(containers) as { pod, c } (`${key(pod)}/${c.name}`)}
              <K8sContainerRow {pod} container={c}
                onlogs={() => openLogs(pod, c)}
                oninspect={() => (inspecting = { key: key(pod), focus: c.name })}
                onrestart={() => replacePod(pod)} />
            {/each}
          </tbody>
        </table>
      </div>
      {#if containers.length === 0}{@render empty("No containers match")}{/if}
      <Pagination {page} perPage={PER_PAGE} total={containers.length} onpage={(p) => (page = p)} />

    {:else if tab === "services"}
      {@render sectionNotice("services")}
      <div class="table-wrap">
        <table>
          <thead><tr><th>Name</th><th>Type</th><th>Cluster IP</th><th>External</th><th>Ports</th><th>Selects</th><th>Age</th></tr></thead>
          <tbody>
            {#each paged(services) as s (key(s))}
              <tr class="row">
                <td>
                  <div class="row-name">{s.name}</div>
                  <div class="ns-sub">{s.namespace}</div>
                </td>
                <td><span class="pill {s.type === 'LoadBalancer' ? 'pill-violet' : s.type === 'NodePort' ? 'pill-amber' : 'pill-mist'}">{s.type}</span></td>
                <td class="cell-mono cell-mist">{s.cluster_ip === "None" ? "None (headless)" : (s.cluster_ip ?? "—")}</td>
                <td class="cell-mono">{s.external?.length ? s.external.join(", ") : "—"}</td>
                <td class="cell-mono cell-mist">{svcPorts(s)}</td>
                <td>
                  {#if s.selector && Object.keys(s.selector).length}
                    <div class="chips">{#each Object.entries(s.selector) as [k, v] (k)}<span class="chip">{k}={v}</span>{/each}</div>
                  {:else}<span class="cell-mist">—</span>{/if}
                </td>
                <td class="cell-mist">{fmtAge(s.created)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if services.length === 0 && !kubernetesStore.sectionErrors.services}{@render empty("No services match")}{/if}
      <Pagination {page} perPage={PER_PAGE} total={services.length} onpage={(p) => (page = p)} />

    {:else if tab === "nodes"}
      {@render sectionNotice("nodes")}
      <div class="table-wrap">
        <table>
          <thead><tr><th>Node</th><th>Status</th><th>Version</th><th>System</th><th>CPU</th><th>Memory</th><th>Pods</th><th>Age</th></tr></thead>
          <tbody>
            {#each nodes as n (n.name)}
              {@const used = podsPerNode[n.name] ?? 0}
              {@const cap = parseInt(n.pods) || 0}
              <tr class="row">
                <td>
                  <div class="row-name">
                    {n.name}
                    {#each n.roles as r (r)}<span class="kind-tag">{r}</span>{/each}
                  </div>
                  <div class="row-sub">{n.internal_ip ?? ""}</div>
                </td>
                <td>
                  <span class="pill {n.status === 'Ready' ? 'pill-green' : 'pill-red'}">{#if n.status === "Ready"}<span class="dot"></span>{/if}{n.status}</span>
                  {#if n.unschedulable}<div class="ns-sub warn-txt">scheduling disabled</div>{/if}
                  {#each n.pressure ?? [] as p (p)}<div class="ns-sub warn-txt">{p}</div>{/each}
                </td>
                <td class="cell-mono">{n.version}</td>
                <td class="small">
                  <div>{n.os_image}</div>
                  <div class="ns-sub">{n.arch} · {n.container_runtime}</div>
                </td>
                <td class="cell-mono">{n.cpu} <span class="cell-mist">cores</span></td>
                <td class="cell-mono">{fmtMemory(n.memory)}</td>
                <td>
                  <div class="ready-cell">
                    <span class="cell-mono">{used}/{cap || "?"}</span>
                    {#if cap}<span class="bar-track"><span class="bar-fill pill-green" style="width:{Math.min(100, (used / cap) * 100)}%"></span></span>{/if}
                  </div>
                </td>
                <td class="cell-mist">{fmtAge(n.created)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if nodes.length === 0 && !kubernetesStore.sectionErrors.nodes}{@render empty("No nodes match")}{/if}

    {:else if tab === "events"}
      {@render sectionNotice("events")}
      <div class="panel-tools">
        <div class="seg">
          <button class:active={!warningsOnly} onclick={() => (warningsOnly = false)}>All</button>
          <button class:active={warningsOnly} onclick={() => (warningsOnly = true)}>Warnings</button>
        </div>
        <span class="tools-hint">Newest first. Kubernetes keeps events for about an hour.</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>Type</th><th>Reason</th><th>Object</th><th>Message</th><th>Count</th><th>Last seen</th></tr></thead>
          <tbody>
            {#each paged(events) as e, i (i)}
              <tr class="row">
                <td><span class="pill {e.type === 'Warning' ? 'pill-red' : 'pill-mist'}">{e.type}</span></td>
                <td class="fw">{e.reason}</td>
                <td>
                  {#if e.object_kind === "Pod" && kubernetesStore.pods.some((p) => p.name === e.object_name && p.namespace === e.namespace)}
                    <button class="link" onclick={() => (inspecting = { key: `${e.namespace}/${e.object_name}` })}>{e.object_name}</button>
                  {:else}
                    <span class="cell-mono nowrap">{e.object_name}</span>
                  {/if}
                  <div class="ns-sub">{e.object_kind} · {e.namespace}</div>
                </td>
                <td class="msg">{e.message}</td>
                <td class="cell-mono cell-mist">{e.count}</td>
                <td class="cell-mist nowrap">{fmtAge(e.last_seen)} ago</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if events.length === 0 && !kubernetesStore.sectionErrors.events}{@render empty(warningsOnly ? "No warnings" : "No events match")}{/if}
      <Pagination {page} perPage={PER_PAGE} total={events.length} onpage={(p) => (page = p)} />
    {/if}
  </div>
{/if}

{#if webhookFor && hostsStore.activeHostId}
  <K8sWebhookModal hostId={hostsStore.activeHostId} workload={webhookFor} onclose={() => (webhookFor = null)} />
{/if}

{#if inspectedPod}
  <K8sPodDrawer
    pod={inspectedPod}
    focus={inspecting?.focus}
    events={inspectedEvents}
    onclose={() => (inspecting = null)}
    onlogs={(c) => (logsFor = { pod: inspectedPod!, container: c })}
    ondelete={() => replacePod(inspectedPod!)}
  />
{/if}

{#if logsFor}
  <LogsModal
    containerName={`${logsFor.pod.namespace}/${logsFor.pod.name}${logsFor.container ? ` · ${logsFor.container}` : ""}`}
    fetchLogs={fetchPodLogs(logsFor.pod, logsFor.container)}
    onclose={() => (logsFor = null)}
  />
{/if}

<style>
  .state-msg { color: var(--mist); font-size: 14px; padding: 60px 0; text-align: center; }
  .empty-state {
    display: flex; flex-direction: column; align-items: center; gap: 10px;
    padding: 80px 0; color: var(--mist);
  }
  .empty-state :global(svg) { width: 38px; height: 38px; color: var(--mist-2); opacity: .6; }
  .empty-state p { font-size: 14px; margin: 0; }
  .empty-hint { font-size: 12.5px; color: var(--mist-2); max-width: 560px; text-align: center; }
  .empty-row { padding: 36px 0; text-align: center; color: var(--mist); font-size: 13px; }
  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px; padding: 7px 12px;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600;
  }

  /* Cluster header */
  .cluster {
    display: flex; align-items: center; gap: 14px; padding: 16px 18px; margin-bottom: 16px;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); box-shadow: var(--shadow-sm);
  }
  .cluster-icon {
    width: 40px; height: 40px; border-radius: 10px; flex-shrink: 0;
    background: var(--accent-tint); color: var(--accent); display: flex; align-items: center; justify-content: center;
  }
  .cluster-icon svg { width: 21px; height: 21px; }
  .cluster-main { flex: 1; min-width: 0; }
  .cluster-name { display: flex; align-items: center; gap: 10px; font-weight: 750; font-size: 16px; color: var(--ink); flex-wrap: wrap; }
  .cluster-meta { display: flex; flex-wrap: wrap; gap: 4px 14px; margin-top: 4px; font-size: 12.5px; color: var(--mist); }
  .cluster-meta .mono { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; overflow-wrap: anywhere; }
  .conn {
    display: flex; align-items: center; gap: 10px; padding: 8px 12px; flex-shrink: 0;
    border-radius: var(--radius-sm); border: 1px solid var(--border); background: var(--surface-2);
  }
  .conn svg { width: 16px; height: 16px; flex-shrink: 0; }
  .conn-push svg { color: var(--green); }
  .conn-pull svg { color: var(--violet); }
  .conn-direct svg { color: var(--mist); }
  .conn-label { font-size: 12.5px; font-weight: 700; color: var(--ink); }
  .conn-sub { font-size: 11.5px; color: var(--mist); }

  /* KPIs */
  .kpi-row { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 14px; margin-bottom: 20px; }
  .kpi-card {
    text-align: left; font: inherit; color: inherit;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 14px 16px; box-shadow: var(--shadow-sm); transition: border-color .12s, box-shadow .12s;
  }
  .kpi-card:hover { border-color: var(--border-strong); box-shadow: var(--shadow-md); }
  .kpi-lbl { font-size: 11.5px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .04em; }
  .kpi-val { font-size: 24px; font-weight: 800; color: var(--ink); line-height: 1; margin-top: 10px; }
  .kpi-val.bad-val { color: var(--red); }
  .kpi-of { font-size: 14px; font-weight: 600; color: var(--mist); }
  .kpi-trend { font-size: 12px; margin-top: 6px; color: var(--mist); }
  .kpi-trend.warn { color: var(--amber); }
  .kpi-trend.bad { color: var(--red); }

  /* Tabs + filters */
  .bar { display: flex; align-items: flex-end; justify-content: space-between; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
  /* Tabs wrap instead of scrolling; the underline is an inset shadow so
     nothing overflows the row (an overflowing row grew a scrollbar). */
  .tabs { display: flex; flex-wrap: wrap; gap: 2px; box-shadow: inset 0 -1px 0 var(--border); min-width: 0; }
  .tab {
    display: inline-flex; align-items: center; gap: 6px; padding: 9px 12px;
    border: none; background: none;
    font-size: 13px; font-weight: 650; color: var(--mist); white-space: nowrap;
  }
  .tab:hover { color: var(--ink); }
  .tab.active { color: var(--accent-ink); box-shadow: inset 0 -2px 0 var(--accent); }
  .tab-badge {
    font-size: 10.5px; font-weight: 700; padding: 1px 6px; border-radius: 20px;
    background: var(--surface-2); border: 1px solid var(--border); color: var(--mist);
  }
  .tab.active .tab-badge { background: var(--accent-tint); border-color: var(--accent-tint); color: var(--accent-ink); }
  .tab-badge.tab-badge-red { background: var(--red-tint); border-color: var(--red-tint); color: var(--red); }
  .filters { display: flex; gap: 8px; align-items: center; }
  .filter-input, .ns-select {
    padding: 7px 11px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface); color: var(--ink); font-size: 13px;
  }
  .filter-input { width: 200px; }
  .icon-btn {
    width: 34px; height: 34px; border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface); color: var(--mist); display: inline-flex; align-items: center; justify-content: center;
  }
  .icon-btn:hover { color: var(--ink); border-color: var(--border-strong); }
  .icon-btn svg { width: 14px; height: 14px; }
  .chip-row { margin: -2px 0 12px; }
  .filter-chip {
    display: inline-flex; align-items: center; gap: 6px; padding: 4px 6px 4px 11px; border-radius: 20px;
    background: var(--accent-tint); color: var(--accent-ink); font-size: 12.5px;
  }
  .filter-chip button {
    border: none; background: none; color: inherit; font-size: 15px; line-height: 1; padding: 0 4px; border-radius: 50%;
  }

  /* Panel + tables */
  .panel {
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    box-shadow: var(--shadow-sm); overflow: hidden;
  }
  .panel-tools { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; padding: 12px 16px; border-bottom: 1px solid var(--border); }
  .tools-hint { font-size: 12px; color: var(--mist-2); }
  .seg { display: inline-flex; border: 1px solid var(--border); border-radius: var(--radius-sm); overflow: hidden; }
  .seg button { padding: 5px 11px; border: none; background: var(--surface); color: var(--mist); font-size: 12px; font-weight: 650; }
  .seg button:not(:last-child) { border-right: 1px solid var(--border); }
  .seg button.active { background: var(--accent); color: #fff; }
  .table-wrap { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  thead th {
    text-align: left; padding: 10px 16px; font-size: 11.5px; text-transform: uppercase;
    letter-spacing: .05em; color: var(--mist-2); font-weight: 700;
    border-bottom: 1px solid var(--border); background: var(--surface-2); white-space: nowrap;
  }
  .panel :global(tbody td) { padding: 11px 16px; border-bottom: 1px solid var(--border); font-size: 13.3px; vertical-align: middle; }
  .panel :global(tbody tr:last-child td) { border-bottom: none; }
  .panel :global(tbody tr:hover > td) { background: var(--surface-2); }
  .row-name { font-weight: 650; font-size: 13.3px; color: var(--ink); }
  .row-name.block { display: block; }
  .row-sub { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--mist-2); margin-top: 2px; max-width: 320px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .ns-sub { display: block; font-size: 11px; color: var(--mist-2); margin-top: 2px; }
  .warn-txt { color: var(--amber); }
  .cell-mono { font-family: 'JetBrains Mono', monospace; font-size: 12px; }
  .cell-mist { color: var(--mist-2); }
  .small { font-size: 12px; }
  .fw { font-weight: 650; }
  .nowrap { white-space: nowrap; }
  .msg { font-size: 12.5px; color: var(--ink); min-width: 260px; }
  .min0 { min-width: 0; text-align: left; }
  .kind-tag {
    margin-left: 6px; font-size: 10px; font-weight: 700; letter-spacing: .03em; text-transform: uppercase;
    color: var(--mist); background: var(--surface-2); border: 1px solid var(--border); padding: 0 6px; border-radius: 20px;
    vertical-align: 1px;
  }
  td > .kind-tag:first-child { margin-left: 0; }
  .pulse-tag {
    margin-left: 6px; font-size: 10px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase;
    color: var(--accent-ink); background: var(--accent-tint); padding: 1px 6px; border-radius: 20px; vertical-align: 1px;
  }
  .restarts { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--mist-2); }
  .restarts.hot { color: var(--amber); font-weight: 700; }
  .chips { display: flex; flex-wrap: wrap; gap: 4px; }
  .chip {
    font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--mist);
    background: var(--surface-2); border: 1px solid var(--border); padding: 0 6px; border-radius: 6px;
  }
  .link {
    border: none; background: none; padding: 0; font-family: 'JetBrains Mono', monospace; font-size: 12px;
    color: var(--accent-ink); text-align: left; white-space: nowrap;
  }
  .link:hover { text-decoration: underline; }

  .ready-cell { display: flex; align-items: center; gap: 10px; }
  .bar-track { width: 70px; height: 5px; border-radius: 4px; background: var(--border); overflow: hidden; flex-shrink: 0; }
  .bar-fill { display: block; height: 100%; border-radius: 4px; }
  .bar-fill.pill-green { background: var(--green); }
  .bar-fill.pill-amber { background: var(--amber); }
  .bar-fill.pill-red { background: var(--red); }
  .bar-fill.pill-mist { background: var(--mist-2); }

  .stepper { display: inline-flex; align-items: center; border: 1px solid var(--border); border-radius: var(--radius-sm); overflow: hidden; }
  .stepper button { width: 28px; height: 28px; border: none; background: var(--surface-2); color: var(--ink); font-size: 14px; font-weight: 700; }
  .stepper button:hover:not(:disabled) { background: var(--accent-tint); color: var(--accent); }
  .stepper button:disabled { opacity: .35; cursor: not-allowed; }
  .stepper span { min-width: 32px; text-align: center; font-size: 12.5px; font-weight: 700; font-family: 'JetBrains Mono', monospace; }

  .row-actions { display: flex; gap: 5px; justify-content: flex-end; opacity: 0; transition: opacity .12s; }
  .row:hover .row-actions { opacity: 1; }
  .act-btn {
    width: 30px; height: 30px; border-radius: 8px; border: 1px solid var(--border); background: var(--surface);
    display: flex; align-items: center; justify-content: center; color: var(--mist); flex-shrink: 0;
    transition: background .12s, color .12s, border-color .12s;
  }
  .act-btn svg { width: 14px; height: 14px; }
  .act-btn:hover:not(:disabled) { background: var(--surface-2); color: var(--ink); border-color: var(--border-strong); }
  .act-btn:disabled { opacity: .3; cursor: not-allowed; }
  .act-btn.danger:hover:not(:disabled) { background: var(--red-tint); color: var(--red); border-color: var(--red); }

  .pod-toggle { display: inline-flex; align-items: center; gap: 8px; background: none; border: none; padding: 0; color: inherit; }
  .pod-toggle:hover .row-name { color: var(--accent); }
  .chev { width: 13px; height: 13px; color: var(--mist-2); transition: transform .15s ease; flex-shrink: 0; }
  .chev.open { transform: rotate(90deg); }
  .panel :global(tr.row-open > td) { background: var(--surface-2); }
  .sub-tr > td { padding: 0 16px 14px 36px !important; background: var(--surface-2); }
  .sub-table { border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--surface); border-collapse: separate; border-spacing: 0; overflow: hidden; }
  .sub-table thead th { background: var(--surface); padding: 8px 14px; font-size: 10.5px; }
  .sub-table :global(tbody td) { padding: 9px 14px; }
  .sub-msg { font-size: 12.5px; color: var(--mist); padding: 8px 2px; }

  .notice {
    display: flex; gap: 10px; margin: 14px 16px; padding: 11px 14px; border-radius: var(--radius-sm);
    background: var(--amber-tint); color: var(--ink); font-size: 12.5px; line-height: 1.5;
  }
  .notice svg { width: 16px; height: 16px; color: var(--amber); flex-shrink: 0; margin-top: 1px; }
  .notice-hint { color: var(--mist); margin-top: 4px; }
  .notice code { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; }

  @media (hover: none) { .row-actions { opacity: 1; } }
  @media (max-width: 1100px) {
    .kpi-row { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  }
  @media (max-width: 720px) {
    .cluster { flex-wrap: wrap; }
    .conn { width: 100%; }
    .kpi-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    .filters { width: 100%; }
    .filter-input { flex: 1; width: auto; min-width: 0; }
    .ns-select { max-width: 45%; }
  }
</style>
