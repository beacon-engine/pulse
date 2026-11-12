<script lang="ts">
  import { untrack } from 'svelte';
  import type { Account } from '../lib/api';
  import UsersPage from './UsersPage.svelte';
  import ComingSoon from '../components/ComingSoon.svelte';

  let { me, initial }: { me: Account; initial?: Section } = $props();

  type Section =
    | 'users' | 'roles' | 'tokens' | 'audit'
    | 'alerts' | 'notifications'
    | 'scans' | 'vulnerabilities'
    | 'agents' | 'enrollment';

  const GROUPS: { label: string; items: { id: Section; label: string; soon?: boolean }[] }[] = [
    {
      label: 'Access',
      items: [
        { id: 'users', label: 'Users' },
        { id: 'roles', label: 'Roles & permissions', soon: true },
        { id: 'tokens', label: 'API tokens', soon: true },
        { id: 'audit', label: 'Audit log', soon: true },
      ],
    },
    {
      label: 'Monitoring',
      items: [
        { id: 'alerts', label: 'Metric alerts', soon: true },
        { id: 'notifications', label: 'Notifications', soon: true },
      ],
    },
    {
      label: 'Security',
      items: [
        { id: 'scans', label: 'Security scans', soon: true },
        { id: 'vulnerabilities', label: 'Vulnerabilities', soon: true },
      ],
    },
    {
      label: 'Fleet',
      items: [
        { id: 'agents', label: 'Agent liveness', soon: true },
        { id: 'enrollment', label: 'Agent enrollment', soon: true },
      ],
    },
  ];
  const ALL = GROUPS.flatMap((g) => g.items);
  const KEY = 'pulse.settings.section';

  // Only the starting section; the parent remounts this page to change it.
  let section = $state<Section>(untrack(() => initial) ?? readSection());

  function readSection(): Section {
    try {
      const s = localStorage.getItem(KEY) as Section | null;
      if (s && ALL.some((i) => i.id === s)) return s;
    } catch {}
    return 'users';
  }

  function select(s: Section) {
    section = s;
    try {
      localStorage.setItem(KEY, s);
    } catch {}
  }
</script>

<div class="settings">
  <nav class="side" aria-label="Settings">
    {#each GROUPS as g (g.label)}
      <div class="side-group">
        <div class="side-label">{g.label}</div>
        {#each g.items as item (item.id)}
          <button class="side-item" class:active={section === item.id} aria-current={section === item.id ? 'page' : undefined}
            onclick={() => select(item.id)}>
            {item.label}
            {#if item.soon}<span class="soon-dot" title="Coming soon"></span>{/if}
          </button>
        {/each}
      </div>
    {/each}
  </nav>

  <section class="body">
    {#if section === 'users'}
      <UsersPage {me} />

    {:else if section === 'roles'}
      <ComingSoon
        title="Roles & permissions"
        summary="Replace the fixed admin / viewer roles with roles you define. Each role is a set of permissions, and the permissions decide both what the API allows and what the dashboard shows."
        planned={[
          'Create, rename and delete roles; assign one or more roles to each user',
          'A permission matrix per role, grouped by area (containers, stacks, Kubernetes, hosts, settings…)',
          'Scope a role to environments, e.g. operator on staging, read-only on production',
          'Built-in Admin and Viewer roles kept as defaults that can be copied but not deleted',
          'Pages, buttons and menu items hidden when the user lacks the permission',
        ]}
      >
        <p>
          Permissions will live in one JSON catalog that the gateway loads and serves to the dashboard
          (e.g. <code>GET /api/permissions</code>), so the API checks and the UI share one source of truth:
        </p>
        <pre>{`{
  "permissions": [
    { "id": "containers.view",   "group": "Containers", "label": "See containers and logs" },
    { "id": "containers.manage", "group": "Containers", "label": "Start, stop, restart, recreate" },
    { "id": "k8s.scale",         "group": "Kubernetes", "label": "Scale and restart workloads" },
    { "id": "settings.roles",    "group": "Settings",   "label": "Manage roles" }
  ],
  "roles": {
    "admin":  ["*"],
    "viewer": ["*.view"]
  }
}`}</pre>
      </ComingSoon>

    {:else if section === 'tokens'}
      <ComingSoon
        title="API tokens"
        summary="Personal and service tokens for scripts and CI, as an alternative to session cookies."
        planned={[
          'Create tokens with a name, an expiry and a subset of your permissions',
          'See when each token was last used; revoke at any time',
          'SSO (OIDC) sign-in for teams',
        ]}
      />

    {:else if section === 'audit'}
      <ComingSoon
        title="Audit log"
        summary="Who did what, where and when: every change made through Pulse."
        planned={[
          'Each action recorded with user, environment, target and result',
          'Filter by user, environment, action or time range',
          'Export as CSV / JSON and set how long entries are kept',
        ]}
      />

    {:else if section === 'alerts'}
      <ComingSoon
        title="Metric alerts"
        summary="Get told when a host, container or workload crosses a threshold, instead of watching the Metrics page."
        planned={[
          'Rules such as "CPU above 90% for 5 minutes" or "memory above 80%" per environment, container or workload',
          'Kubernetes rules: pods restarting, workloads not fully ready, nodes NotReady',
          'Severity, a quiet period, and auto-resolve when the value recovers',
          'Send to any configured channel (Slack today; email and webhooks next)',
          'A history of fired and resolved alerts',
        ]}
      />

    {:else if section === 'notifications'}
      <ComingSoon
        title="Notifications"
        summary="Where alerts and notices go. Slack alerting works today through environment variables; this moves it here."
        planned={[
          'Slack, email, Microsoft Teams and generic webhook channels',
          'Choose which events reach which channel (crashes, OOM kills, deploys, alerts, vulnerabilities)',
          'Send a test message',
        ]}
      />

    {:else if section === 'scans'}
      <ComingSoon
        title="Security scans"
        summary="Scan the images your containers and workloads run for known vulnerabilities, on a schedule or on demand."
        planned={[
          'Choose a scanner (Trivy first) and the minimum severity to report',
          'Scan on a schedule, when a new image is deployed, or on demand',
          'Pick environments and exclude images or CVEs you have accepted',
          'Notify a channel when new vulnerabilities appear',
        ]}
      />

    {:else if section === 'vulnerabilities'}
      <ComingSoon
        title="Vulnerabilities"
        summary="Findings from security scans: what was found, in which image, where it runs, and when it was found."
        planned={[
          'Findings by severity with CVE, package, installed and fixed versions',
          'Which containers and workloads run each affected image',
          'First seen / last seen, and a timeline of scans',
          'Acknowledge or accept a finding with a reason',
        ]}
      />

    {:else if section === 'agents'}
      <ComingSoon
        title="Agent liveness"
        summary="How long an auto-registered agent may stay silent before it is marked offline, and when offline environments are cleaned up."
        planned={[
          'Edit the heartbeat and offline thresholds (the gateway already stores these; see GET/PUT /api/settings)',
          'See each agent’s version, mode (push / pull) and last heartbeat',
        ]}
      />

    {:else if section === 'enrollment'}
      <ComingSoon
        title="Agent enrollment"
        summary="Add agents without hand-copying shared tokens: one-time enrollment codes and per-agent credentials (see docs/AGENT_ENROLLMENT.md)."
        planned={[
          'Generate a one-time code and a ready-to-run command or Kubernetes manifest',
          'Per-agent credentials that can be rotated or revoked individually',
          'Encrypted, mutually authenticated gateway↔agent connections',
        ]}
      />
    {/if}
  </section>
</div>

<style>
  .settings { display: grid; grid-template-columns: 220px minmax(0, 1fr); gap: 24px; align-items: start; }
  .side {
    position: sticky; top: 16px; display: flex; flex-direction: column; gap: 14px;
    background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 12px 8px; box-shadow: var(--shadow-sm);
  }
  .side-group { display: flex; flex-direction: column; gap: 1px; }
  .side-label {
    font-size: 11px; font-weight: 700; color: var(--mist-2); text-transform: uppercase; letter-spacing: .06em;
    padding: 2px 10px 6px;
  }
  .side-item {
    display: flex; align-items: center; justify-content: space-between; gap: 8px;
    padding: 7px 10px; border: none; border-radius: 7px; background: none;
    font-size: 13px; font-weight: 600; color: var(--mist); text-align: left;
  }
  .side-item:hover { background: var(--surface-2); color: var(--ink); }
  .side-item.active { background: var(--accent-tint); color: var(--accent-ink); }
  .soon-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--amber); opacity: .7; flex-shrink: 0; }
  .body {
    min-width: 0; background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-md);
    padding: 22px 24px; box-shadow: var(--shadow-sm);
  }

  @media (max-width: 800px) {
    .settings { grid-template-columns: 1fr; }
    .side { position: static; flex-direction: row; flex-wrap: wrap; gap: 8px 16px; }
    .side-group { flex-direction: row; flex-wrap: wrap; align-items: center; gap: 4px; }
    .side-label { width: 100%; padding-bottom: 2px; }
    .body { padding: 18px 16px; }
  }
</style>
