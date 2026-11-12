<script lang="ts">
  import { untrack } from "svelte";
  import { dashboardStore as ds } from "../stores/dashboard.svelte";
  import { hostsStore } from "../stores/hosts.svelte";
  import { navigate, type Route } from "../lib/router";
  import { RUNTIME_LABEL, satisfies } from "../types/environments";
  import type { DiskInfo } from "../types/containers";
  import { connectionOf, podStatus } from "../utils/k8s";
  import { fmtAge, fmtBytes } from "../utils/fmt";

  // The landing page: what every environment has in common, whatever it runs.
  $effect(() => {
    if (!hostsStore.activeHostId) return;
    untrack(() => ds.startPolling(20000));
    return () => ds.stopPolling();
  });

  let env = $derived(hostsStore.activeHost);
  let conn = $derived(connectionOf(env));
  let caps = $derived(ds.caps);
  let docker = $derived(!!caps?.reachable && satisfies(caps, "containers"));
  let swarm = $derived(!!caps?.reachable && satisfies(caps, "swarm"));
  let k8s = $derived(!!caps?.reachable && satisfies(caps, "kubernetes"));

  let status = $derived(
    env?.status === "offline"
      ? { label: "Offline", pill: "pill-red" }
      : caps && !caps.reachable
        ? { label: "Unreachable", pill: "pill-red" }
        : { label: "Connected", pill: "pill-green" },
  );

  // ── Docker ───────────────────────────────────────────────────────────────
  // Pod containers (Kubernetes on the Docker engine) are counted under Kubernetes.
  let ownContainers = $derived((ds.containers ?? []).filter((c) => !c.kubernetes));
  let running = $derived(ownContainers.filter((c) => c.state === "running").length);
  let stopped = $derived(ownContainers.length - running);

  // Exit codes of a deliberate stop (`docker stop` sends SIGTERM → 143, Ctrl-C
  // → 130): stopped, not crashed.
  const STOP_CODES = new Set([0, 130, 143]);

  function exitCode(status: string): number | null {
    const m = /Exited \((\d+)\)/.exec(status);
    return m ? Number(m[1]) : null;
  }

  let mainDisk = $derived.by((): DiskInfo | null => {
    const disks = ds.hardware?.disks ?? [];
    return disks.find((d) => d.mount === "/") ?? [...disks].sort((a, b) => b.total - a.total)[0] ?? null;
  });

  // ── Swarm ────────────────────────────────────────────────────────────────
  let swarmHealthy = $derived((ds.swarmServices ?? []).filter((s) => s.running >= s.replicas).length);
  let swarmReady = $derived((ds.swarmNodes ?? []).filter((n) => n.state === "ready").length);

  // ── Kubernetes ───────────────────────────────────────────────────────────
  let healthyWorkloads = $derived((ds.workloads ?? []).filter((w) => w.ready >= w.replicas).length);
  let runningPods = $derived((ds.pods ?? []).filter((p) => podStatus(p).pill === "pill-green").length);
  let failingPods = $derived((ds.pods ?? []).filter((p) => podStatus(p).pill === "pill-red").length);
  let readyNodes = $derived((ds.nodes ?? []).filter((n) => n.status === "Ready").length);
  let warnings = $derived((ds.k8sEvents ?? []).filter((e) => e.type === "Warning").length);

  // ── Needs attention ──────────────────────────────────────────────────────
  type Issue = { severity: "bad" | "warn"; title: string; detail: string; route: Route };
  let issues = $derived.by((): Issue[] => {
    const out: Issue[] = [];
    for (const c of ownContainers) {
      const code = exitCode(c.status);
      if (c.state === "restarting") out.push({ severity: "bad", title: c.name, detail: "container keeps restarting", route: "containers" });
      else if (c.state === "dead") out.push({ severity: "bad", title: c.name, detail: "container is dead", route: "containers" });
      else if (c.state === "exited" && code === 137)
        out.push({ severity: "warn", title: c.name, detail: "container was killed (137): out of memory, or a forced stop", route: "containers" });
      else if (c.state === "exited" && code !== null && !STOP_CODES.has(code))
        out.push({ severity: "bad", title: c.name, detail: `container exited with code ${code}`, route: "containers" });
      else if (/unhealthy/i.test(c.status)) out.push({ severity: "warn", title: c.name, detail: "health check failing", route: "containers" });
    }
    for (const s of ds.swarmServices ?? []) {
      if (s.running < s.replicas)
        out.push({ severity: "warn", title: s.name, detail: `Swarm service ${s.running}/${s.replicas} running`, route: "swarm" });
    }
    for (const n of ds.nodes ?? []) {
      if (n.status !== "Ready") out.push({ severity: "bad", title: n.name, detail: `node ${n.status}`, route: "kubernetes" });
    }
    for (const w of ds.workloads ?? []) {
      if (w.ready < w.replicas)
        out.push({ severity: "warn", title: `${w.namespace}/${w.name}`, detail: `${w.kind} ${w.ready}/${w.replicas} ready`, route: "kubernetes" });
    }
    for (const p of ds.pods ?? []) {
      const st = podStatus(p);
      if (st.pill === "pill-red") out.push({ severity: "bad", title: `${p.namespace}/${p.name}`, detail: `pod ${st.label}`, route: "kubernetes" });
    }
    return out.sort((a, b) => (a.severity === b.severity ? 0 : a.severity === "bad" ? -1 : 1));
  });
  const SHOWN = 8;

  function since(ts?: string | null) {
    if (!ts) return "";
    // SQLite timestamps ("2026-11-11 19:25:20") are UTC without a zone.
    const iso = ts.includes("T") ? ts : ts.replace(" ", "T") + "Z";
    return fmtAge(iso);
  }
</script>

{#snippet kpi(label: string, value: string | number, of: number | null, note: string, tone: "" | "warn" | "bad", to: Route)}
  <button class="kpi-card" onclick={() => navigate(to)}>
    <span class="kpi-lbl">{label}</span>
    <div class="kpi-val">{value}{#if of !== null}<span class="kpi-of"> / {of}</span>{/if}</div>
    <div class="kpi-trend {tone}">{note}</div>
  </button>
{/snippet}

{#snippet meter(label: string, used: number, total: number, pct: number)}
  <div class="meter">
    <div class="meter-top">
      <span class="meter-lbl">{label}</span>
      <span class="meter-val">{fmtBytes(used)} / {fmtBytes(total)}</span>
    </div>
    <div class="bar"><div class="fill" class:hot={pct >= 90} class:warm={pct >= 75 && pct < 90} style="width: {Math.min(pct, 100)}%"></div></div>
    <div class="meter-pct">{pct.toFixed(0)}% used</div>
  </div>
{/snippet}

{#if !env}
  <div class="state-msg">No environment selected.</div>
{:else}
  <!-- Environment -->
  <div class="env">
    <div class="env-icon">
      {#if k8s && !docker}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z" /><path d="M12 22V12M20 6.5L12 12M4 6.5L12 12" /></svg>
      {:else}
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="4" width="18" height="7" rx="1.5" /><rect x="3" y="13" width="18" height="7" rx="1.5" /><path d="M7 7.5h.01M7 16.5h.01" /></svg>
      {/if}
    </div>
    <div class="env-main">
      <div class="env-name">
        {env.name}
        <span class="pill {status.pill}">{#if status.pill === "pill-green"}<span class="dot"></span>{/if}{status.label}</span>
      </div>
      <div class="env-meta">
        {#each caps?.runtimes ?? [] as r (r.kind)}
          <span>{RUNTIME_LABEL[r.kind] ?? r.kind} {r.version}</span>
        {/each}
        {#if ds.system}<span>{ds.system.os} · {ds.system.arch}</span>{/if}
        {#if env.last_seen}<span>seen {since(env.last_seen)} ago</span>{/if}
        {#if env.created_at}<span>added {since(env.created_at)} ago</span>{/if}
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
          {conn.mode === "push" ? "agent dials the gateway" : conn.mode === "pull" ? (env.agent_addr ?? "gateway dials the agent") : "next to the gateway"}
        </div>
      </div>
    </div>
  </div>

  {#if ds.loading && !caps}
    <div class="state-msg">Loading {env.name}…</div>
  {:else if caps && !caps.reachable}
    <div class="notice bad">
      <strong>Pulse can't reach this environment's agent.</strong>
      <span>{caps.error ?? "No answer from the agent."}</span>
      <button class="btn-ghost" onclick={() => ds.refresh()}>Try again</button>
    </div>
  {:else if caps}
    {#if docker}
      <div class="sec-title">Containers</div>
      <div class="kpi-row">
        {@render kpi("Running", running, ownContainers.length, stopped ? `${stopped} stopped` : "all running", "", "containers")}
        {@render kpi("Images", ds.system?.images ?? "—", null, "on this host", "", "resources")}
        {@render kpi("Stacks", ds.stacks ?? "—", null, "Compose projects", "", "stacks")}
        {@render kpi("CPUs", ds.system?.cpus ?? "—", null, ds.system ? `Docker ${ds.system.docker_version}` : "", "", "metrics")}
      </div>
      {#if ds.hardware}
        <div class="meters">
          {@render meter("Memory", ds.hardware.mem_used, ds.hardware.mem_total, ds.hardware.mem_pct)}
          {#if mainDisk}
            {@render meter(`Disk ${mainDisk.mount}`, mainDisk.used, mainDisk.total, mainDisk.pct)}
          {/if}
        </div>
      {/if}
    {/if}

    {#if swarm && ds.swarmServices}
      <div class="sec-title">Swarm</div>
      <div class="kpi-row">
        {@render kpi("Services", swarmHealthy, ds.swarmServices.length, swarmHealthy < ds.swarmServices.length ? "some under-replicated" : "all running", swarmHealthy < ds.swarmServices.length ? "warn" : "", "swarm")}
        {@render kpi("Nodes", swarmReady, ds.swarmNodes?.length ?? 0, "ready", "", "swarm")}
      </div>
    {/if}

    {#if k8s}
      <div class="sec-title">Kubernetes</div>
      <div class="kpi-row">
        {@render kpi("Workloads", healthyWorkloads, ds.workloads?.length ?? 0, healthyWorkloads < (ds.workloads?.length ?? 0) ? `${(ds.workloads?.length ?? 0) - healthyWorkloads} not fully ready` : "all healthy", healthyWorkloads < (ds.workloads?.length ?? 0) ? "warn" : "", "kubernetes")}
        {@render kpi("Pods", runningPods, ds.pods?.length ?? 0, failingPods ? `${failingPods} failing` : "running and ready", failingPods ? "bad" : "", "kubernetes")}
        {@render kpi("Nodes", ds.nodes ? readyNodes : "—", ds.nodes ? ds.nodes.length : null, "ready", "", "kubernetes")}
        {@render kpi("Warnings", ds.k8sEvents ? warnings : "—", null, "recent events", warnings ? "bad" : "", "kubernetes")}
      </div>
    {/if}

    {#if !docker && !k8s}
      <div class="notice">
        <strong>No container runtime or cluster found.</strong>
        <span>The agent answers, but reaches neither Docker/Podman nor Kubernetes.</span>
      </div>
    {/if}

    <div class="sec-title">Needs attention <span class="count" class:count-bad={issues.length > 0}>{issues.length}</span></div>
    {#if issues.length === 0}
      <div class="all-good">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6L9 17l-5-5" /></svg>
        Nothing is crashing, restarting or short of replicas.
      </div>
    {:else}
      <div class="issues">
        {#each issues.slice(0, SHOWN) as i, n (n)}
          <button class="issue" onclick={() => navigate(i.route)}>
            <span class="sev sev-{i.severity}"></span>
            <span class="issue-title">{i.title}</span>
            <span class="issue-detail">{i.detail}</span>
            <span class="issue-go">{i.route === "kubernetes" ? "Kubernetes" : i.route === "swarm" ? "Swarm" : "Containers"} →</span>
          </button>
        {/each}
        {#if issues.length > SHOWN}
          <div class="issue-more">and {issues.length - SHOWN} more</div>
        {/if}
      </div>
    {/if}
  {/if}
{/if}

<style>
  .state-msg { padding: 48px 0; text-align: center; color: var(--mist); font-size: 13px; }

  .env {
    display: flex; align-items: center; gap: 14px; padding: 16px 18px; margin-bottom: 20px;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); box-shadow: var(--shadow-sm);
  }
  .env-icon {
    width: 40px; height: 40px; border-radius: 10px; flex-shrink: 0;
    background: var(--accent-tint); color: var(--accent); display: flex; align-items: center; justify-content: center;
  }
  .env-icon svg { width: 21px; height: 21px; }
  .env-main { flex: 1; min-width: 0; }
  .env-name { display: flex; align-items: center; gap: 10px; font-weight: 750; font-size: 16px; color: var(--ink); flex-wrap: wrap; }
  .env-meta { display: flex; flex-wrap: wrap; gap: 4px 14px; margin-top: 4px; font-size: 12.5px; color: var(--mist); }
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

  .sec-title {
    display: flex; align-items: center; gap: 8px; margin: 4px 0 10px;
    font-size: 12px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .05em;
  }
  .count { font-size: 10.5px; padding: 1px 7px; border-radius: 20px; background: var(--surface-2); border: 1px solid var(--border); color: var(--mist); }
  .count-bad { background: var(--red-tint); border-color: transparent; color: var(--red); }

  .kpi-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(170px, 1fr)); gap: 14px; margin-bottom: 20px; }
  .kpi-card {
    text-align: left; font: inherit; color: inherit;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 14px 16px; box-shadow: var(--shadow-sm); transition: border-color .12s, box-shadow .12s;
  }
  .kpi-card:hover { border-color: var(--border-strong); box-shadow: var(--shadow-md); }
  .kpi-lbl { font-size: 11.5px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .04em; }
  .kpi-val { font-size: 24px; font-weight: 800; color: var(--ink); line-height: 1; margin-top: 10px; }
  .kpi-of { font-size: 14px; font-weight: 600; color: var(--mist); }
  .kpi-trend { font-size: 12px; margin-top: 6px; color: var(--mist); }
  .kpi-trend.warn { color: var(--amber); }
  .kpi-trend.bad { color: var(--red); }

  .meters { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 14px; margin: -6px 0 20px; }
  .meter {
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 14px 16px; box-shadow: var(--shadow-sm);
  }
  .meter-top { display: flex; justify-content: space-between; gap: 10px; }
  .meter-lbl { font-size: 11.5px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .04em; }
  .meter-val { font-size: 12.5px; color: var(--ink); font-weight: 600; }
  .bar { height: 7px; border-radius: 4px; background: var(--surface-2); border: 1px solid var(--border); margin-top: 10px; overflow: hidden; }
  .fill { height: 100%; background: var(--green); border-radius: 4px; }
  .fill.warm { background: var(--amber); }
  .fill.hot { background: var(--red); }
  .meter-pct { font-size: 12px; color: var(--mist); margin-top: 6px; }

  .notice {
    display: flex; flex-wrap: wrap; align-items: center; gap: 6px 12px; padding: 14px 16px; margin-bottom: 20px;
    border-radius: var(--radius-md); border: 1px solid var(--border); background: var(--surface); font-size: 13px; color: var(--mist);
  }
  .notice strong { color: var(--ink); }
  .notice.bad { border-color: var(--red); background: var(--red-tint); }
  .notice.bad span { color: var(--red); word-break: break-word; }
  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; margin-left: auto;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface); color: var(--ink); font-size: 12.5px; font-weight: 600;
  }

  .all-good {
    display: flex; align-items: center; gap: 10px; padding: 14px 16px;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    font-size: 13px; color: var(--mist);
  }
  .all-good svg { width: 16px; height: 16px; color: var(--green); }
  .issues { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); overflow: hidden; }
  .issue {
    display: flex; align-items: center; gap: 12px; width: 100%; padding: 10px 16px; text-align: left;
    background: none; border: none; border-bottom: 1px solid var(--border); font: inherit; color: inherit;
  }
  .issue:last-child { border-bottom: none; }
  .issue:hover { background: var(--surface-2); }
  .sev { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .sev-bad { background: var(--red); }
  .sev-warn { background: var(--amber); }
  .issue-title { font-size: 13px; font-weight: 650; color: var(--ink); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 45%; }
  .issue-detail { flex: 1; font-size: 12.5px; color: var(--mist); min-width: 0; }
  .issue-go { font-size: 12px; color: var(--accent-ink); font-weight: 600; white-space: nowrap; }
  .issue-more { padding: 9px 16px; font-size: 12px; color: var(--mist); }

  @media (max-width: 700px) {
    .env { flex-wrap: wrap; }
    .conn { width: 100%; }
    .issue { flex-wrap: wrap; gap: 4px 10px; }
    .issue-title { max-width: 100%; }
  }
</style>
