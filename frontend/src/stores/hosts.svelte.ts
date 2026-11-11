// src/lib/stores/hosts.svelte.ts
import { api } from "../lib/api";
import {
  satisfies,
  type Environment,
  type HostCapabilities,
  type Requirement,
} from "../types/environments";

class HostsStore {
  hosts = $state<Environment[]>([]);
  totalHosts = $state(0);
  activeHostId = $state<string | null>(null);
  activeHostObj = $state<Environment | null>(null);

  /** Capabilities by host id. Missing = not probed yet. */
  capabilities = $state<Record<string, HostCapabilities>>({});

  activeHost = $derived(
    this.hosts.find((h) => h.id === this.activeHostId) ?? this.activeHostObj
  );

  activeCapabilities = $derived(
    this.activeHostId ? this.capabilities[this.activeHostId] ?? null : null
  );

  /** Whether the active environment backs a page. Unknown (still probing)
   *  counts as yes, so navigation doesn't flicker while the probe runs. */
  supports(req: Requirement): boolean {
    const caps = this.activeCapabilities;
    return caps === null || satisfies(caps, req);
  }

  async loadHosts() {
    try {
      const p = await api.listHosts({ perPage: 8 });
      this.hosts = p.items;
      this.totalHosts = p.total;
      if (this.activeHostId === null && this.hosts.length > 0) {
        const initial = this.hosts.find((h) => h.is_local) ?? this.hosts[0];
        this.activeHostId = initial.id;
        this.activeHostObj = initial;
      }
      if (this.activeHostId) void this.probe(this.activeHostId);
    } catch (e) {
      console.error('Failed to load hosts', e);
    }
  }

  /** Ask the host's agent which runtimes it can reach. */
  async probe(id: string): Promise<HostCapabilities> {
    let caps: HostCapabilities;
    try {
      caps = await api.getHostCapabilities(id);
    } catch (e: any) {
      caps = {
        reachable: false,
        error: e?.response?.data ?? String(e),
        runtimes: [],
        unavailable: [],
      };
    }
    this.capabilities = { ...this.capabilities, [id]: caps };
    return caps;
  }

  switchHost(host: Environment) {
    this.activeHostObj = host;
    if (!this.hosts.some((h) => h.id === host.id)) {
      this.hosts = [host, ...this.hosts].slice(0, 8);
    }
    this.activeHostId = host.id;
    void this.probe(host.id);
  }
}

export const hostsStore = new HostsStore();
