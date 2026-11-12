import { PollingStore } from "./base.svelte";
import type { Container, Stack, SavedStack, ContainerEvent } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

class ContainersStore extends PollingStore {
  containers = $state<Container[]>([]);
  stacks = $state<Stack[]>([]);
  savedStacks = $state<SavedStack[]>([]);
  events = $state<ContainerEvent[]>([]);
  selectedContainer = $state<Container | null>(null);
  loading = $state(false);
  error = $state("");
  runningCount = $derived(this.containers.filter((c) => c.state === "running").length);

  protected reset() {
    this.containers = [];
    this.stacks = [];
    this.savedStacks = [];
    this.selectedContainer = null;
    this.error = "";
  }

  async refresh() {
    const hostId = this.beginLoad();
    if (!hostId) return;

    if (this.containers.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      const [c, d, s, e] = await Promise.all([
        api.getContainers(hostId),
        api.getDetectedStacks(hostId),
        api.getSavedStacks(hostId),
        api.getEvents(),
      ]);
      if (this.stale(hostId)) return;
      this.containers = c;
      this.stacks = d;
      this.savedStacks = s;
      this.events = e;

      // Keep drawer in sync if open
      if (this.selectedContainer) {
        const updated = c.find((item) => item.id === this.selectedContainer?.id);
        if (updated) this.selectedContainer = updated;
      }
    } catch (err: any) {
      if (this.stale(hostId)) return;
      this.error = err?.response?.data ?? String(err);
      toast.error(this.error);
    } finally {
      if (!this.stale(hostId)) this.loading = false;
    }
  }

  // ── Container Actions ───────────────────────────────────────────────────────
  startContainer = async (id: string) => {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.startContainer(hostId, id);
      toast.success("Container started");
      await this.refresh();
    } catch (e: any) {
      toast.error(e?.response?.data ?? String(e));
    }
  };

  stopContainer = async (id: string) => {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.stopContainer(hostId, id);
      toast.info("Container stopped");
      await this.refresh();
    } catch (e: any) {
      toast.error(e?.response?.data ?? String(e));
    }
  };

  restartContainer = async (id: string) => {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.restartContainer(hostId, id);
      toast.success("Container restarted");
      await this.refresh();
    } catch (e: any) {
      toast.error(e?.response?.data ?? String(e));
    }
  };

  selectContainer(c: Container | null) {
    this.selectedContainer = c;
  }
}

export const containersStore = new ContainersStore();
