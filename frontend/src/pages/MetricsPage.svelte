<script lang="ts">
  import { metricsStore, type MetricSample } from "../stores/metric.svelte";
  import { hostsStore } from "../stores/hosts.svelte";
  import { fmtBytes, fmtNum, fmtUptime } from "../utils/fmt";

  $effect(() => {
    const hostId = hostsStore.activeHostId;
    if (hostId) {
      metricsStore.startPolling(15000);
    }
    return () => {
      metricsStore.stopPolling();
    };
  });

  function getSample(name: string, labels?: Record<string, string>): number | null {
    const s = metricsStore.samples.find((s) => {
      if (s.name !== name) return false;
      if (!labels) return true;
      return Object.entries(labels).every(([k, v]) => s.labels[k] === v);
    });
    return s ? s.value : null;
  }

  function getSamples(name: string): MetricSample[] {
    return metricsStore.samples.filter((s) => s.name === name);
  }

  let uptime = $derived(getSample("pulse_uptime_seconds"));
  let containersTotal = $derived(getSample("pulse_containers_total"));
  let requestSamples = $derived(getSamples("pulse_http_requests_total"));
  let eventSamples = $derived(getSamples("pulse_container_events_total"));
  let totalRequests = $derived(requestSamples.reduce((a, s) => a + s.value, 0));

  // ── External Prometheus ───────────────────────────────────────────────────
  let promUrl = $state(localStorage.getItem("pulse-prom-url") ?? "");
  let promQuery = $state("");
  let promResult = $state<unknown>(null);
  let promError = $state("");
  let promBusy = $state(false);

  const PRESET_QUERIES = [
    { label: "Container CPU %", q: 'rate(container_cpu_usage_seconds_total{name!=""}[1m]) * 100' },
    { label: "Container Memory (MB)", q: 'container_memory_usage_bytes{name!=""} / 1024 / 1024' },
    { label: "Container Net RX (kbps)", q: "rate(container_network_receive_bytes_total[1m]) / 1024" },
    { label: "Container Net TX (kbps)", q: "rate(container_network_transmit_bytes_total[1m]) / 1024" },
  ];

  $effect(() => {
    if (promUrl) localStorage.setItem("pulse-prom-url", promUrl);
  });

  async function runPromQuery() {
    if (!promUrl || !promQuery) return;
    promBusy = true;
    promError = "";
    promResult = null;
    try {
      const resp = await fetch(
        `/api/prometheus/query?url=${encodeURIComponent(promUrl)}&q=${encodeURIComponent(promQuery)}`
      );
      promResult = await resp.json();
    } catch (e) {
      promError = String(e);
    } finally {
      promBusy = false;
    }
  }

  type PromData = {
    status: string;
    data: {
      resultType: string;
      result: Array<{ metric: Record<string, string>; value: [number, string] }>;
    };
  };

  let promRows = $derived(() => {
    if (!promResult || typeof promResult !== "object") return [];
    const r = promResult as PromData;
    if (r.status !== "success") return [];
    return r.data?.result ?? [];
  });
</script>

<!-- Hardware metrics -->
{#if metricsStore.hw}
  <div class="hw-section">
    <div class="hw-card">
      <div class="hw-label">RAM</div>
      <div class="hw-bar-row">
        <div class="hw-bar-track">
          <div
            class="hw-bar-fill"
            style="width:{metricsStore.hw.mem_pct.toFixed(1)}%; background: {metricsStore.hw.mem_pct > 85 ? 'var(--red)' : metricsStore.hw.mem_pct > 65 ? 'var(--amber)' : 'var(--accent)'}"
          ></div>
        </div>
        <span class="hw-pct">{metricsStore.hw.mem_pct.toFixed(1)}%</span>
      </div>
      <div class="hw-sub">{fmtBytes(metricsStore.hw.mem_used)} / {fmtBytes(metricsStore.hw.mem_total)}</div>
    </div>
    {#each metricsStore.hw.disks.slice(0, 4) as disk}
      <div class="hw-card">
        <div class="hw-label">{disk.mount === "/" ? "Root disk" : disk.mount}</div>
        <div class="hw-bar-row">
          <div class="hw-bar-track">
            <div
              class="hw-bar-fill"
              style="width:{disk.pct.toFixed(1)}%; background: {disk.pct > 85 ? 'var(--red)' : disk.pct > 65 ? 'var(--amber)' : 'var(--green)'}"
            ></div>
          </div>
          <span class="hw-pct">{disk.pct.toFixed(1)}%</span>
        </div>
        <div class="hw-sub">{fmtBytes(disk.used)} / {fmtBytes(disk.total)}</div>
      </div>
    {/each}
  </div>
{:else if metricsStore.hwError}
  <div class="err-box" style="margin-bottom:18px">{metricsStore.hwError}</div>
{/if}

<!-- Pulse own metrics -->
<div class="metrics-grid">
  <div class="m-card accent">
    <div class="m-label">Uptime</div>
    <div class="m-val">{fmtUptime(uptime)}</div>
    <div class="m-sub">since last restart</div>
  </div>
  <div class="m-card">
    <div class="m-label">Containers</div>
    <div class="m-val">{fmtNum(containersTotal)}</div>
    <div class="m-sub">known to Docker</div>
  </div>
  <div class="m-card">
    <div class="m-label">API requests</div>
    <div class="m-val">{fmtNum(totalRequests)}</div>
    <div class="m-sub">all time</div>
  </div>
  <div class="m-card">
    <div class="m-label">Events recorded</div>
    <div class="m-val">{fmtNum(eventSamples.reduce((a, s) => a + s.value, 0))}</div>
    <div class="m-sub">container lifecycle</div>
  </div>
</div>

<div class="section-row">
  <div class="section-block">
    <div class="section-hdr">
      <h2>API activity</h2>
      <button class="btn-ghost btn-sm" onclick={() => metricsStore.loadPulseMetrics()} disabled={metricsStore.metricsLoading}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M3 12a9 9 0 1 0 2.6-6.4L3 8" />
          <path d="M3 3v5h5" />
        </svg>
        Refresh
      </button>
    </div>
    {#if metricsStore.metricsError}
      <div class="err-box">{metricsStore.metricsError}</div>
    {:else if requestSamples.length === 0 && !metricsStore.metricsLoading}
      <div class="empty-hint">No requests recorded yet</div>
    {:else}
      <table class="m-table">
        <thead>
          <tr>
            <th>Endpoint</th>
            <th>Method</th>
            <th>Status</th>
            <th class="num">Count</th>
          </tr>
        </thead>
        <tbody>
          {#each requestSamples.sort((a, b) => b.value - a.value) as s (s.name + JSON.stringify(s.labels))}
            <tr>
              <td class="mono small">{s.labels.endpoint ?? "—"}</td>
              <td><span class="method-tag">{s.labels.method ?? "—"}</span></td>
              <td>
                <span
                  class="status-tag"
                  class:ok={s.labels.status?.startsWith("2")}
                  class:err={s.labels.status?.startsWith("4") || s.labels.status?.startsWith("5")}
                >
                  {s.labels.status ?? "—"}
                </span>
              </td>
              <td class="num mono">{fmtNum(s.value)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>

  <div class="section-block">
    <div class="section-hdr"><h2>Container events</h2></div>
    {#if eventSamples.length === 0}
      <div class="empty-hint">No events recorded yet</div>
    {:else}
      <div class="event-bars">
        {#each eventSamples as s}
          {@const max = Math.max(...eventSamples.map((e) => e.value), 1)}
          <div class="ebar-row">
            <span class="ebar-label">{s.labels.event ?? s.name}</span>
            <div class="ebar-track">
              <div
                class="ebar-fill"
                style="width:{(s.value / max * 100).toFixed(1)}%; background:{s.labels.event === 'start' ? 'var(--green)' : s.labels.event === 'crash' || s.labels.event === 'oom' ? 'var(--red)' : 'var(--amber)'}"
              ></div>
            </div>
            <span class="ebar-val">{fmtNum(s.value)}</span>
          </div>
        {/each}
      </div>
    {/if}

    <div class="prom-endpoint">
      <span class="prom-label">Scrape endpoint</span>
      <code class="prom-url">/metrics</code>
    </div>
  </div>
</div>

<!-- Webhooks reference -->
{#if hostsStore.activeHostId}
  <div class="ext-section" style="margin-bottom: 20px;">
    <div class="ext-hdr">
      <h2>Deployment webhooks</h2>
      <span class="ext-sub">POST these URLs from CI/CD to auto-pull and redeploy</span>
    </div>
    <div class="webhook-list">
      <div class="wh-row">
        <span class="wh-badge container">container</span>
        <code class="wh-url">/api/webhooks/{hostsStore.activeHostId}/&lt;container-id&gt;/recreate?pull=true</code>
        <span class="wh-hint">Pull latest image and recreate container</span>
      </div>
      <div class="wh-row">
        <span class="wh-badge stack">stack</span>
        <code class="wh-url">/api/webhooks/{hostsStore.activeHostId}/stacks/&lt;stack-id&gt;/redeploy?pull=true</code>
        <span class="wh-hint">Pull latest images and docker compose up -d</span>
      </div>
      <div class="wh-row">
        <span class="wh-badge swarm">swarm</span>
        <code class="wh-url">/api/webhooks/{hostsStore.activeHostId}/swarm/&lt;service-id&gt;/update</code>
        <span class="wh-hint">Force rolling update of a Swarm service</span>
      </div>
      <div class="wh-row">
        <span class="wh-badge k8s">k8s</span>
        <code class="wh-url">/api/webhooks/{hostsStore.activeHostId}/k8s/&lt;namespace&gt;/&lt;deployment&gt;/rollout</code>
        <span class="wh-hint">Rollout restart a Kubernetes deployment</span>
      </div>
    </div>
  </div>
{/if}

<!-- External Prometheus -->
<div class="ext-section">
  <div class="ext-hdr">
    <h2>External Prometheus</h2>
    <span class="ext-sub">Query your Prometheus instance for container resource metrics</span>
  </div>
  <div class="prom-config">
    <div class="prom-url-row">
      <label class="field-label">Prometheus URL</label>
      <div class="prom-input-row">
        <input
          class="text-input"
          type="text"
          placeholder="http://prometheus:9090"
          bind:value={promUrl}
        />
        <span class="prom-status" class:connected={!!promUrl}>{promUrl ? "configured" : "not set"}</span>
      </div>
    </div>
    <div class="preset-row">
      {#each PRESET_QUERIES as pq}
        <button
          class="preset-btn"
          onclick={() => {
            promQuery = pq.q;
            runPromQuery();
          }}
        >
          {pq.label}
        </button>
      {/each}
    </div>
    <div class="query-row">
      <input
        class="text-input mono-input"
        type="text"
        placeholder="PromQL query, e.g. up"
        bind:value={promQuery}
        onkeydown={(e) => e.key === "Enter" && runPromQuery()}
      />
      <button class="btn-primary" onclick={runPromQuery} disabled={promBusy || !promUrl || !promQuery}>
        {#if promBusy}Running…{:else}Run query{/if}
      </button>
    </div>
    {#if promError}
      <div class="err-box">{promError}</div>
    {:else if promRows().length > 0}
      <table class="m-table">
        <thead>
          <tr>
            {#each Object.keys(promRows()[0].metric) as k}<th>{k}</th>{/each}
            <th class="num">Value</th>
          </tr>
        </thead>
        <tbody>
          {#each promRows() as row}
            <tr>
              {#each Object.values(row.metric) as v}<td class="small">{v}</td>{/each}
              <td class="num mono">{parseFloat(row.value[1]).toFixed(4)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if promResult && !promBusy}
      <pre class="raw-result">{JSON.stringify(promResult, null, 2)}</pre>
    {/if}
  </div>
</div>

<style>
  /* Hardware */
  .hw-section {
    display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 12px; margin-bottom: 20px;
  }
  .hw-card {
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 14px 16px; box-shadow: var(--shadow-sm);
  }
  .hw-label { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; color: var(--mist-2); margin-bottom: 10px; }
  .hw-bar-row { display: flex; align-items: center; gap: 10px; margin-bottom: 6px; }
  .hw-bar-track { flex: 1; height: 8px; background: var(--surface-2); border-radius: 20px; overflow: hidden; border: 1px solid var(--border); }
  .hw-bar-fill { height: 100%; border-radius: 20px; transition: width .4s ease; }
  .hw-pct { font-family: 'JetBrains Mono', monospace; font-size: 13px; font-weight: 700; color: var(--ink); min-width: 44px; text-align: right; }
  .hw-sub { font-size: 11.5px; color: var(--mist); }

  .metrics-grid {
    display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin-bottom: 24px;
  }
  .m-card {
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 18px 20px; box-shadow: var(--shadow-sm);
  }
  .m-card.accent { border-color: var(--accent); background: var(--accent-tint); }
  .m-label { font-size: 11.5px; font-weight: 700; text-transform: uppercase; letter-spacing: .05em; color: var(--mist-2); margin-bottom: 8px; }
  .m-val { font-family: 'Manrope', sans-serif; font-size: 28px; font-weight: 800; color: var(--ink); line-height: 1; }
  .m-card.accent .m-val { color: var(--accent-ink); }
  .m-sub { font-size: 12px; color: var(--mist); margin-top: 6px; }

  .section-row { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; margin-bottom: 24px; }
  .section-block { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); padding: 18px 20px; }
  .section-hdr { display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; }
  .section-hdr h2 { font-size: 14px; font-weight: 700; margin: 0; }

  .m-table { width: 100%; border-collapse: collapse; }
  .m-table th {
    text-align: left; font-size: 11px; font-weight: 700; text-transform: uppercase;
    letter-spacing: .04em; color: var(--mist-2); padding: 7px 10px;
    border-bottom: 1px solid var(--border); background: var(--surface-2);
  }
  .m-table td { padding: 8px 10px; font-size: 12.5px; border-bottom: 1px solid var(--border); }
  .m-table tr:last-child td { border-bottom: none; }
  .m-table th.num, .m-table td.num { text-align: right; }
  .mono { font-family: 'JetBrains Mono', monospace; }
  .small { font-size: 11.5px; }

  .method-tag {
    display: inline-block; padding: 1px 7px; border-radius: 20px; font-size: 10.5px;
    font-weight: 700; text-transform: uppercase; background: var(--accent-tint); color: var(--accent-ink);
  }
  .status-tag {
    display: inline-block; padding: 1px 7px; border-radius: 20px; font-size: 10.5px;
    font-weight: 700; background: var(--surface-2); color: var(--mist);
  }
  .status-tag.ok  { background: var(--green-tint); color: var(--green); }
  .status-tag.err { background: var(--red-tint);   color: var(--red); }

  .event-bars { display: flex; flex-direction: column; gap: 10px; margin-bottom: 18px; }
  .ebar-row { display: flex; align-items: center; gap: 10px; }
  .ebar-label { font-size: 12px; font-weight: 600; min-width: 64px; color: var(--mist); }
  .ebar-track { flex: 1; height: 8px; background: var(--surface-2); border-radius: 20px; overflow: hidden; }
  .ebar-fill { height: 100%; border-radius: 20px; transition: width .4s ease; }
  .ebar-val { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--mist); min-width: 36px; text-align: right; }

  .prom-endpoint {
    display: flex; align-items: center; gap: 10px; margin-top: 18px;
    padding: 10px 12px; background: var(--surface-2); border-radius: var(--radius-sm); border: 1px solid var(--border);
  }
  .prom-label { font-size: 11.5px; font-weight: 700; color: var(--mist-2); }
  .prom-url { font-family: 'JetBrains Mono', monospace; font-size: 12px; color: var(--accent); background: var(--accent-tint); padding: 2px 8px; border-radius: 5px; }

  .empty-hint { font-size: 13px; color: var(--mist-2); text-align: center; padding: 24px 0; }
  .err-box { padding: 10px 12px; background: var(--red-tint); color: var(--red); border-radius: var(--radius-sm); font-size: 12.5px; }

  .ext-section { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md); padding: 20px 22px; }
  .ext-hdr { margin-bottom: 18px; }
  .ext-hdr h2 { font-size: 15px; font-weight: 700; margin: 0 0 4px; }
  .ext-sub { font-size: 13px; color: var(--mist); }
  .prom-config { display: flex; flex-direction: column; gap: 14px; }
  .prom-url-row { display: flex; flex-direction: column; gap: 6px; }
  .field-label { font-size: 12px; font-weight: 700; color: var(--mist); }
  .prom-input-row { display: flex; align-items: center; gap: 10px; }
  .prom-status { font-size: 12px; font-weight: 600; color: var(--mist-2); }
  .prom-status.connected { color: var(--green); }
  .text-input {
    flex: 1; padding: 9px 12px; border: 1px solid var(--border); border-radius: var(--radius-sm);
    background: var(--surface-2); color: var(--ink); font-size: 13px; outline: none;
    transition: border-color .15s;
  }
  .text-input:focus { border-color: var(--accent); background: var(--surface); }
  .mono-input { font-family: 'JetBrains Mono', monospace; font-size: 12.5px; }
  .preset-row { display: flex; flex-wrap: wrap; gap: 7px; }
  .preset-btn {
    padding: 5px 12px; border-radius: 20px; border: 1px solid var(--border);
    background: var(--surface-2); color: var(--mist); font-size: 12px; font-weight: 600;
    cursor: pointer; transition: all .12s;
  }
  .preset-btn:hover { background: var(--accent-tint); color: var(--accent-ink); border-color: transparent; }
  .query-row { display: flex; gap: 10px; align-items: center; }
  .btn-primary {
    display: inline-flex; align-items: center; gap: 6px; padding: 9px 16px;
    border-radius: var(--radius-sm); border: none; background: var(--accent);
    color: #fff; font-size: 13px; font-weight: 650; cursor: pointer; white-space: nowrap;
    transition: filter .15s;
  }
  .btn-primary:hover:not(:disabled) { filter: brightness(1.07); }
  .btn-primary:disabled { opacity: .45; cursor: not-allowed; }
  .btn-ghost {
    display: inline-flex; align-items: center; gap: 6px; padding: 7px 12px;
    border-radius: var(--radius-sm); border: 1px solid var(--border);
    background: var(--surface-2); color: var(--ink); font-size: 13px; font-weight: 600;
    cursor: pointer; transition: background .12s, border-color .12s;
  }
  .btn-ghost:hover:not(:disabled) { background: var(--surface); border-color: var(--border-strong); }
  .btn-ghost:disabled { opacity: .4; cursor: not-allowed; }
  .btn-ghost svg { width: 13px; height: 13px; }
  .btn-sm { padding: 5px 10px; font-size: 12px; }
  .raw-result {
    background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm);
    padding: 12px 14px; font-family: 'JetBrains Mono', monospace; font-size: 11.5px;
    max-height: 320px; overflow-y: auto; color: var(--mist); white-space: pre;
  }

  /* Webhooks */
  .webhook-list { display: flex; flex-direction: column; gap: 10px; }
  .wh-row { display: flex; align-items: center; gap: 10px; padding: 10px 12px; background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--radius-sm); flex-wrap: wrap; }
  .wh-badge {
    font-size: 10px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em;
    padding: 2px 8px; border-radius: 20px; flex-shrink: 0;
  }
  .wh-badge.container { background: var(--accent-tint); color: var(--accent-ink); }
  .wh-badge.stack     { background: var(--violet-tint); color: var(--violet); }
  .wh-badge.swarm     { background: var(--amber-tint);  color: var(--amber); }
  .wh-badge.k8s       { background: var(--green-tint);  color: var(--green); }
  .wh-url { font-family: 'JetBrains Mono', monospace; font-size: 11.5px; color: var(--ink); flex: 1; word-break: break-all; }
  .wh-hint { font-size: 11.5px; color: var(--mist); white-space: nowrap; }
</style>
