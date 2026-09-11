import { PollingStore } from "./base.svelte";
import type { K8sPod, K8sNamespace } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

class KubernetesStore extends PollingStore {
  pods = $state<K8sPod[]>([]);
  namespaces = $state<K8sNamespace[]>([]);
  loading = $state(false);
  error = $state("");

  async refresh() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) {
      // No host selected yet — nothing to query. Clear state and bail so we
      // don't hit an unscoped URL that 404s into the SPA fallback.
      this.pods = [];
      this.namespaces = [];
      this.loading = false;
      return;
    }

    if (this.pods.length === 0 && this.namespaces.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      const [p, n] = await Promise.all([
        api.getK8sPods(hostId),
        api.getK8sNamespaces(hostId),
      ]);
      this.pods = p;
      this.namespaces = n;
    } catch (err: any) {
      this.error = err?.response?.data ?? String(err);
      toast.error(`Kubernetes error: ${this.error}`);
    } finally {
      this.loading = false;
    }
  }

  async deletePod(namespace: string, name: string) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    await api.deleteK8sPod(hostId, namespace, name);
    toast.info(`Pod "${name}" deleted`);
    await this.refresh();
  }
}

export const kubernetesStore = new KubernetesStore();
