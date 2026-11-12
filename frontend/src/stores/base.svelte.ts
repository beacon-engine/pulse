import { hostsStore } from "./hosts.svelte";

export abstract class PollingStore {
  protected pollTimer: ReturnType<typeof setInterval> | null = null;
  intervalMs = $state(20000);
  /** The environment the store's data was loaded for. */
  private loadedFor: string | null = null;

  abstract refresh(): Promise<void>;

  /** Clear the data that belongs to one environment. */
  protected reset(): void {}

  /** Start a load for the active environment and return its id. When that's
   *  not the environment the store's data came from, the data is cleared first,
   *  so a page never shows the previous environment while the new one loads. */
  protected beginLoad(): string | null {
    const id = hostsStore.activeHostId;
    if (id !== this.loadedFor) {
      this.loadedFor = id;
      this.reset();
    }
    return id;
  }

  /** Whether a load for hostId finished after the user moved to another
   *  environment; its result must then be dropped. */
  protected stale(hostId: string): boolean {
    return hostsStore.activeHostId !== hostId;
  }

  startPolling(intervalMs = 20000) {
    this.intervalMs = intervalMs;
    this.stopPolling(); // Clear existing if any

    // Initial fetch
    this.refresh();

    // Start background interval
    this.pollTimer = setInterval(() => {
      // Don't hammer the network if tab is hidden in background
      if (document.visibilityState === 'visible') {
        this.refresh();
      }
    }, this.intervalMs);
  }

  stopPolling() {
    if (this.pollTimer) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  setInterval(ms: number) {
    this.startPolling(ms);
  }
}
