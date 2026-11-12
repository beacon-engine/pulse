// src/lib/stores/hosts.svelte.ts
import { api } from "../lib/api";
import { envFromUrl, setEnvInUrl } from "../lib/router";
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
        // Prefer the environment in the URL (it may be beyond the first page),
        // then the local one, then the first.
        const wanted = envFromUrl();
        const initial =
          (wanted && (await this.find(wanted))) ||
          this.hosts.find((h) => h.is_local) ||
          this.hosts[0];
        this.activeHostId = initial.id;
        this.activeHostObj = initial;
        setEnvInUrl(initial.id, true);
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

  /** Re-read the active environment, so its online/offline status follows the
   *  gateway's liveness checks while a page stays open. */
  async refreshActive() {
    const id = this.activeHostId;
    if (!id) return;
    try {
      const fresh = await api.getHost(id);
      if (this.activeHostId !== id) return; // switched meanwhile
      this.activeHostObj = fresh;
      this.hosts = this.hosts.map((h) => (h.id === id ? fresh : h));
    } catch {
      // Keep the last known state; the page shows its own load errors.
    }
  }

  switchHost(host: Environment) {
    this.activeHostObj = host;
    if (!this.hosts.some((h) => h.id === host.id)) {
      this.hosts = [host, ...this.hosts].slice(0, 8);
    }
    this.activeHostId = host.id;
    setEnvInUrl(host.id);
    void this.probe(host.id);
  }

  /** Follow the URL when it names another environment (back/forward, a pasted
   *  link). An unknown id puts the active environment back in the URL. */
  async syncFromUrl(id: string | null) {
    if (!id || this.activeHostId === null || id === this.activeHostId) return;
    const host = await this.find(id);
    if (host) this.switchHost(host);
    else setEnvInUrl(this.activeHostId, true);
  }

  private async find(id: string): Promise<Environment | null> {
    const loaded = this.hosts.find((h) => h.id === id);
    if (loaded) return loaded;
    try {
      return await api.getHost(id);
    } catch {
      return null; // deleted, or a stale link
    }
  }
}

export const hostsStore = new HostsStore();
