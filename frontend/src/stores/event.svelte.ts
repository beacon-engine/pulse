import { PollingStore } from "./base.svelte";
import type { ContainerEvent } from "../types/containers";
import { api } from "../lib/api";
import { toast } from "../lib/toast.svelte";

class EventsStore extends PollingStore {
  events = $state<ContainerEvent[]>([]);
  loading = $state(false);
  error = $state("");

  async refresh() {
    if (this.events.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      this.events = await api.getEvents();
    } catch (err: any) {
      this.error = err?.response?.data ?? String(err);
      toast.error(`Failed to load events: ${this.error}`);
    } finally {
      this.loading = false;
    }
  }
}

export const eventsStore = new EventsStore();
