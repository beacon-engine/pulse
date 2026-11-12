import { PollingStore } from "./base.svelte";
import type {
  K8sEvent,
  K8sNamespace,
  K8sNode,
  K8sPod,
  K8sService,
  K8sWorkload,
} from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

class KubernetesStore extends PollingStore {
  pods = $state<K8sPod[]>([]);
  namespaces = $state<K8sNamespace[]>([]);
  workloads = $state<K8sWorkload[]>([]);
  nodes = $state<K8sNode[]>([]);
  services = $state<K8sService[]>([]);
  events = $state<K8sEvent[]>([]);
  /** Why an optional section couldn't load (an older agent, or a ClusterRole
   *  that predates it). The rest of the page still works. */
  sectionErrors = $state<Partial<Record<K8sSection, string>>>({});
  loading = $state(false);
  error = $state("");

  protected reset() {
    this.pods = [];
    this.namespaces = [];
    this.workloads = [];
    this.nodes = [];
    this.services = [];
    this.events = [];
    this.sectionErrors = {};
    this.error = "";
  }

  async refresh() {
    const hostId = this.beginLoad();
    if (!hostId) {
      // No host selected yet: nothing to query (an unscoped URL would 404 into
      // the SPA fallback).
      this.loading = false;
      return;
    }

    if (this.pods.length === 0 && this.namespaces.length === 0) {
      this.loading = true;
    }
    this.error = "";

    const errors: Partial<Record<K8sSection, string>> = {};
    // Optional sections fail on their own: an agent or ClusterRole from before
    // a section existed answers with an error, and the pods should still show.
    const optional = <T>(section: K8sSection, p: Promise<T[]>) =>
      p.catch((err: any) => {
        errors[section] = errorText(err);
        return [] as T[];
      });

    try {
      const [p, n, w, nodes, svcs, events] = await Promise.all([
        api.getK8sPods(hostId),
        api.getK8sNamespaces(hostId),
        optional("workloads", api.getK8sWorkloads(hostId)),
        optional("nodes", api.getK8sNodes(hostId)),
        optional("services", api.getK8sServices(hostId)),
        optional("events", api.getK8sEvents(hostId)),
      ]);
      if (this.stale(hostId)) return;
      this.pods = p;
      this.namespaces = n;
      this.workloads = w;
      this.nodes = nodes;
      this.services = svcs;
      this.events = events;
      this.sectionErrors = errors;
    } catch (err: any) {
      if (this.stale(hostId)) return;
      this.error = errorText(err);
      toast.error(`Kubernetes error: ${this.error}`);
    } finally {
      if (!this.stale(hostId)) this.loading = false;
    }
  }

  async deletePod(namespace: string, name: string) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.deleteK8sPod(hostId, namespace, name);
      toast.info(`Pod "${name}" deleted`);
    } catch (err: any) {
      toast.error(`Delete failed: ${errorText(err)}`);
      throw err;
    }
    await this.refresh();
  }

  async scale(w: K8sWorkload, replicas: number) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      await api.scaleK8sWorkload(hostId, w, replicas);
      toast.info(`${w.name} scaled to ${replicas}`);
    } catch (err: any) {
      toast.error(`Scale failed: ${errorText(err)}`);
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
      toast.error(`Restart failed: ${errorText(err)}`);
    }
    await this.refresh();
  }
}

export type K8sSection = "workloads" | "nodes" | "services" | "events";

function errorText(err: any): string {
  const data = err?.response?.data;
  if (typeof data === "string" && data) return data;
  if (data?.error) return String(data.error);
  return err?.message ?? String(err);
}

export const kubernetesStore = new KubernetesStore();
