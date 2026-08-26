// src/lib/stores/hosts.svelte.ts
import { api } from "../lib/api";
import { Environment } from "../types/environments";

class HostsStore {
  hosts = $state<Environment[]>([]);
  activeHostId = $state<string | null>(null);
  activeHostObj = $state<Environment | null>(null);

  activeHost = $derived(
    this.hosts.find((h) => h.id === this.activeHostId) ?? this.activeHostObj
  );

  async loadHosts() {
    try {
      const p = await api.listHosts({ perPage: 8 });
      this.hosts = p.items;
      if (this.activeHostId === null && this.hosts.length > 0) {
        const initial = this.hosts.find((h) => h.is_local) ?? this.hosts[0];
        this.activeHostId = initial.id;
        this.activeHostObj = initial;
      }
    } catch (e) {
      console.error('Failed to load hosts', e);
    }
  }

  switchHost(host: Environment) {
    this.activeHostObj = host;
    if (!this.hosts.some((h) => h.id === host.id)) {
      this.hosts = [host, ...this.hosts].slice(0, 8);
    }
    this.activeHostId = host.id;
  }
}

export const hostsStore = new HostsStore();
