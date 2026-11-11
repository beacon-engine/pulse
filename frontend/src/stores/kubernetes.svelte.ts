import { PollingStore } from "./base.svelte";
import type { K8sPod, K8sNamespace, K8sWorkload } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

class KubernetesStore extends PollingStore {
  pods = $state<K8sPod[]>([]);
  namespaces = $state<K8sNamespace[]>([]);
  workloads = $state<K8sWorkload[]>([]);
  loading = $state(false);
  error = $state("");

  async refresh() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) {
      // No host selected yet — nothing to query. Clear state and bail so we
      // don't hit an unscoped URL that 404s into the SPA fallback.
      this.pods = [];
      this.namespaces = [];
      this.workloads = [];
      this.loading = false;
      return;
    }

    if (this.pods.length === 0 && this.namespaces.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      const [p, n, w] = await Promise.all([
        api.getK8sPods(hostId),
        api.getK8sNamespaces(hostId),
        // An agent from before workloads existed answers with an error; keep
        // showing its pods rather than failing the whole page.
        api.getK8sWorkloads(hostId).catch(() => [] as K8sWorkload[]),
      ]);
      this.pods = p;
      this.namespaces = n;
      this.workloads = w;
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

  async scale(w: K8sWorkload, replicas: number) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.scaleK8sWorkload(hostId, w, replicas);
      toast.info(`${w.name} scaled to ${replicas}`);
    } catch (err: any) {
      toast.error(`Scale failed: ${err?.response?.data ?? err}`);
    }
    await this.refresh();
  }

  async restart(w: K8sWorkload) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.restartK8sWorkload(hostId, w);
      toast.info(`Rolling restart of ${w.name} started`);
    } catch (err: any) {
      toast.error(`Restart failed: ${err?.response?.data ?? err}`);
    }
    await this.refresh();
  }
}

export const kubernetesStore = new KubernetesStore();
