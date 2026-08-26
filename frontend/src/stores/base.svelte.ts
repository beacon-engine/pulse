export abstract class PollingStore {
  protected pollTimer: ReturnType<typeof setInterval> | null = null;
  intervalMs = $state(20000);

  abstract refresh(): Promise<void>;

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
