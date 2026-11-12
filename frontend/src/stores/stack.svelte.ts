import { PollingStore } from "./base.svelte";
import type { Stack, SavedStack, EnvVar, Container } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

class StacksStore extends PollingStore {
  stacks = $state<Stack[]>([]);
  savedStacks = $state<SavedStack[]>([]);
  containers = $state<Container[]>([]);
  loading = $state(false);
  error = $state("");

  protected reset() {
    this.stacks = [];
    this.savedStacks = [];
    this.containers = [];
    this.error = "";
  }

  async refresh() {
    const hostId = this.beginLoad();
    if (!hostId) return;

    if (this.savedStacks.length === 0 && this.stacks.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      const [d, s, c] = await Promise.all([
        api.getDetectedStacks(hostId),
        api.getSavedStacks(hostId),
        api.getContainers(hostId),
      ]);
      if (this.stale(hostId)) return;
      this.stacks = d;
      this.savedStacks = s;
      this.containers = c;
    } catch (err: any) {
      if (this.stale(hostId)) return;
      this.error = err?.response?.data ?? String(err);
      toast.error(`Failed to load stacks: ${this.error}`);
    } finally {
      if (!this.stale(hostId)) this.loading = false;
    }
  }

  // ── Stack Operations ────────────────────────────────────────────────────────
  async createStack(name: string, content: string) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    await api.createSavedStack(hostId, name, content);
    toast.success("Stack created");
    await this.refresh();
  }

  async updateStack(id: string, content: string, envVars: EnvVar[]) {
    await api.updateSavedStack(id, content, envVars);
    toast.success("Stack updated");
    await this.refresh();
  }

  async runStackUp(id: string) {
    await api.stackUp(id);
    toast.success("Stack started");
    await this.refresh();
  }

  async runStackDown(id: string) {
    await api.stackDown(id);
    toast.info("Stack stopped");
    await this.refresh();
  }

  async deleteStack(id: string) {
    await api.deleteSavedStack(id);
    toast.info("Stack deleted");
    await this.refresh();
  }

  async generateToken(id: string) {
    await api.generateStackToken(id);
    await this.refresh();
  }

  async deleteToken(id: string) {
    await api.deleteStackToken(id);
    await this.refresh();
  }
}

export const stacksStore = new StacksStore();
