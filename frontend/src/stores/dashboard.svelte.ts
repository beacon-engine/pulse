import { PollingStore } from "./base.svelte";
import type {
  Container,
  HardwareMetrics,
  K8sEvent,
  K8sNode,
  K8sPod,
  K8sWorkload,
  SwarmNode,
  SwarmService,
  SystemInfo,
} from "../types/containers";
import { satisfies, type HostCapabilities } from "../types/environments";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";

/** The dashboard's summary of the active environment. Each section loads only
 *  when the environment's agent reports that runtime, and fails on its own. */
class DashboardStore extends PollingStore {
  caps = $state<HostCapabilities | null>(null);
  system = $state<SystemInfo | null>(null);
  hardware = $state<HardwareMetrics | null>(null);
  containers = $state<Container[] | null>(null);
  stacks = $state<number | null>(null);
  swarmServices = $state<SwarmService[] | null>(null);
  swarmNodes = $state<SwarmNode[] | null>(null);
  workloads = $state<K8sWorkload[] | null>(null);
  pods = $state<K8sPod[] | null>(null);
  nodes = $state<K8sNode[] | null>(null);
  k8sEvents = $state<K8sEvent[] | null>(null);
  loading = $state(false);

  protected reset() {
    this.caps = null;
    this.system = null;
    this.hardware = null;
    this.containers = null;
    this.stacks = null;
    this.swarmServices = null;
    this.swarmNodes = null;
    this.workloads = null;
    this.pods = null;
    this.nodes = null;
    this.k8sEvents = null;
  }

  async refresh() {
    const hostId = this.beginLoad();
    if (!hostId) return;
    if (!this.caps) this.loading = true;

    // Re-probing also refreshes the online/offline state the header shows.
    const [caps] = await Promise.all([hostsStore.probe(hostId), hostsStore.refreshActive()]);
    if (this.stale(hostId)) return;
    this.caps = caps;

    const optional = <T>(want: boolean, p: () => Promise<T>) =>
      want ? p().catch(() => null) : Promise.resolve(null);
    const docker = caps.reachable && satisfies(caps, "containers");
    const swarm = caps.reachable && satisfies(caps, "swarm");
    const k8s = caps.reachable && satisfies(caps, "kubernetes");

    const [system, hardware, containers, stacks, swarmServices, swarmNodes, workloads, pods, nodes, events] =
      await Promise.all([
        optional(docker, () => api.getSystemInfo(hostId)),
        optional(docker, () => api.getHardwareMetrics(hostId)),
        optional(docker, () => api.getContainers(hostId)),
        optional(docker, () => api.getDetectedStacks(hostId).then((s) => s.length)),
        optional(swarm, () => api.getSwarmServices(hostId)),
        optional(swarm, () => api.getSwarmNodes(hostId)),
        optional(k8s, () => api.getK8sWorkloads(hostId)),
        optional(k8s, () => api.getK8sPods(hostId)),
        optional(k8s, () => api.getK8sNodes(hostId)),
        optional(k8s, () => api.getK8sEvents(hostId)),
      ]);
    if (this.stale(hostId)) return;
    this.system = system;
    this.hardware = hardware;
    this.containers = containers;
    this.stacks = stacks;
    this.swarmServices = swarmServices;
    this.swarmNodes = swarmNodes;
    this.workloads = workloads;
    this.pods = pods;
    this.nodes = nodes;
    this.k8sEvents = events;
    this.loading = false;
  }
}

export const dashboardStore = new DashboardStore();
