import { PollingStore } from "./base.svelte";
import type { K8sPod, K8sNamespace } from "../types/containers";
import { api } from "../lib/api";
import { toast } from "../lib/toast.svelte";

class KubernetesStore extends PollingStore {
  pods = $state<K8sPod[]>([]);
  namespaces = $state<K8sNamespace[]>([]);
  loading = $state(false);
  error = $state("");

  async refresh() {
    if (this.pods.length === 0 && this.namespaces.length === 0) {
      this.loading = true;
    }
    this.error = "";

    try {
      const [p, n] = await Promise.all([
        api.getK8sPods(),
        api.getK8sNamespaces(),
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
    await api.deleteK8sPod(namespace, name);
    toast.info(`Pod "${name}" deleted`);
    await this.refresh();
  }
}

export const kubernetesStore = new KubernetesStore();
