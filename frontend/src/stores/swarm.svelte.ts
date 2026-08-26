import { PollingStore } from "./base.svelte";
import type { SwarmNode, SwarmService, SwarmTask } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";

class SwarmStore extends PollingStore {
  services = $state<SwarmService[]>([]);
  nodes = $state<SwarmNode[]>([]);
  selectedService = $state<SwarmService | null>(null);
  loading = $state(false);
  error = $state("");

  async refresh() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;

    if (this.services.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      const [s, n] = await Promise.all([
        api.getSwarmServices(hostId),
        api.getSwarmNodes(hostId),
      ]);
      this.services = s;
      this.nodes = n;

      if (this.selectedService) {
        const updated = s.find((item) => item.id === this.selectedService?.id);
        if (updated) this.selectedService = updated;
      }
    } catch (err: any) {
      this.error = err?.response?.data ?? String(err);
    } finally {
      this.loading = false;
    }
  }

  async getServiceTasks(serviceId: string): Promise<SwarmTask[]> {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return [];
    return await api.getSwarmServiceTasks(hostId, serviceId);
  }

  selectService(service: SwarmService | null) {
    this.selectedService = service;
  }
}

export const swarmStore = new SwarmStore();
