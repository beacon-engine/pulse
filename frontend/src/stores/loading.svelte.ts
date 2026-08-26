class LoadingStore {
  // Count concurrent in-flight requests
  activeRequests = $state(0);

  isLoading = $derived(this.activeRequests > 0);

  start() {
    this.activeRequests += 1;
  }

  stop() {
    this.activeRequests = Math.max(0, this.activeRequests - 1);
  }

  reset() {
    this.activeRequests = 0;
  }
}

export const loadingStore = new LoadingStore();
