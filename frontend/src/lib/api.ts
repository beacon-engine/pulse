import axios from "axios";
import type { AxiosInstance, CreateAxiosDefaults } from "axios";
import type {
  Container,
  ContainerDetail,
  ContainerEvent,
  ContainerNetworkInfo,
  DockerImage,
  DockerVolume,
  EnvVar,
  HardwareMetrics,
  HostNetwork,
  K8sEvent,
  K8sNamespace,
  K8sNode,
  K8sPod,
  K8sService,
  K8sWorkload,
  Page,
  SavedStack,
  Stack,
  SwarmNode,
  SwarmService,
  SwarmTask,
  SystemInfo,
} from "../types/containers";
import type { Environment, HostCapabilities } from "../types/environments";
import { loadingStore } from "../stores/loading.svelte";

export type {
  Container,
  ContainerDetail,
  ContainerEvent,
  ContainerNetworkInfo,
  DockerImage,
  DockerVolume,
  EnvVar,
  HardwareMetrics,
  HostNetwork,
  K8sEvent,
  K8sNamespace,
  K8sNode,
  K8sPod,
  K8sService,
  K8sWorkload,
  Page,
  SavedStack,
  Stack,
  SwarmNode,
  SwarmService,
  SwarmTask,
  SystemInfo,
  Environment,
  HostCapabilities,
};

/// Options for paginated list endpoints.
export interface PageOpts {
  page?: number;
  perPage?: number;
  search?: string;
}

function pageParams(o?: PageOpts): Record<string, string | number> {
  const p: Record<string, string | number> = {};
  if (o?.page) p.page = o.page;
  if (o?.perPage) p.per_page = o.perPage;
  if (o?.search) p.search = o.search;
  return p;
}

export type Role = "admin" | "viewer";

export interface Account {
  id: string;
  username: string;
  role: Role;
  created_at?: string;
}

export class PulseClient {
  private http: AxiosInstance;

  constructor(config: CreateAxiosDefaults = {}) {
    this.http = axios.create({
      // Same-origin: the gateway serves this SPA and its API on one port, so a
      // relative base resolves back to whatever host:port the app was loaded from.
      baseURL: "/api",
      headers: { "Content-Type": "application/json" },
      withCredentials: true,
      ...config,
    });

    // Request Interceptor -> Start Loader
    this.http.interceptors.request.use(
      (config) => {
        // Optional: skip background polling if needed via config: { silent: true }
        if (!(config as any).silent) {
          loadingStore.start();
        }
        return config;
      },
      (error) => {
        if (!(error?.config as any)?.silent) {
          loadingStore.stop();
        }
        return Promise.reject(error);
      }
    );

    // Guard against the API proxy pointing at the wrong server: a JSON endpoint
    // that comes back as HTML (e.g. another container's index page) would
    // otherwise be handed to callers as a string and blow up with cryptic
    // ".slice/.find of undefined" errors. Reject it with a clear message.
    this.http.interceptors.response.use(
      (r) => {
        if (!(r.config as any).silent) {
          loadingStore.stop();
        }
        const wantsText = r.config.responseType === "text";
        const contentType = String(r.headers?.["content-type"] ?? "");
        if (!wantsText && contentType.includes("text/html")) {
          return Promise.reject(
            new Error(
              "Gateway returned HTML instead of JSON — the /api proxy is pointing at the wrong server. Is the Pulse gateway running on the proxied port?",
            ),
          );
        }
        return r;
      },
      (err) => {
        // Balance the counter: a failed request still "started" the loader, so
        // it must stop it — otherwise every error leaks a count and the bar
        // animates forever with nothing in flight.
        if (!(err?.config as any)?.silent) {
          loadingStore.stop();
        }
        const status = err?.response?.status;
        const url: string = err?.config?.url ?? "";
        // Ignore 401s from the auth probes themselves (expected when logged out).
        if (status === 401 && !url.startsWith("/auth/")) {
          window.dispatchEvent(new CustomEvent("pulse:unauthorized"));
        }
        return Promise.reject(err);
      },
    );
  }

  // ── Auth & users ────────────────────────────────────────────────────────────

  async setupStatus(): Promise<{ needs_setup: boolean }> {
    const { data } = await this.http.get<{ needs_setup: boolean }>(
      "/auth/setup",
    );
    return data;
  }

  async setup(username: string, password: string): Promise<Account> {
    const { data } = await this.http.post<Account>("/auth/setup", {
      username,
      password,
    });
    return data;
  }

  async login(username: string, password: string): Promise<Account> {
    const { data } = await this.http.post<Account>("/auth/login", {
      username,
      password,
    });
    return data;
  }

  async logout(): Promise<void> {
    await this.http.post("/auth/logout");
  }

  async me(): Promise<Account> {
    const { data } = await this.http.get<Account>("/auth/me");
    return data;
  }

  async listUsers(): Promise<Account[]> {
    const { data } = await this.http.get<Account[]>("/users");
    return data;
  }

  async createUser(
    username: string,
    password: string,
    role: Role,
  ): Promise<Account> {
    const { data } = await this.http.post<Account>("/users", {
      username,
      password,
      role,
    });
    return data;
  }

  async updateUser(
    id: string,
    changes: { role?: Role; password?: string },
  ): Promise<Account> {
    const { data } = await this.http.put<Account>(`/users/${id}`, changes);
    return data;
  }

  async deleteUser(id: string): Promise<void> {
    await this.http.delete(`/users/${id}`);
  }

  // ── Hosts ─────────────────────────────────────────────────────────────────

  async listHosts(opts?: PageOpts): Promise<Page<Environment>> {
    const { data } = await this.http.get<Page<Environment>>("/hosts", {
      params: pageParams(opts),
    });
    return data;
  }

  async getHost(id: string): Promise<Environment> {
    const { data } = await this.http.get<Environment>(`/hosts/${encodeURIComponent(id)}`);
    return data;
  }

  /** Register an environment reached through a Pulse agent (agentAddr) and/or
   *  a container API endpoint (dockerHost). At least one must be given. */
  async createHost(
    name: string,
    target: { agentAddr?: string; dockerHost?: string },
  ): Promise<Environment> {
    const { data } = await this.http.post<Environment>("/hosts", {
      name,
      docker_host: target.dockerHost ?? "",
      agent_addr: target.agentAddr || null,
    });
    return data;
  }

  async getHostCapabilities(id: string): Promise<HostCapabilities> {
    const { data } = await this.http.get<HostCapabilities>(`/hosts/${id}/capabilities`);
    return data;
  }

  async deleteHost(id: string): Promise<void> {
    await this.http.delete(`/hosts/${id}`);
  }

  async renameHost(id: string, name: string): Promise<Environment> {
    const { data } = await this.http.put<Environment>(`/hosts/${id}`, { name });
    return data;
  }

  // ── Containers ────────────────────────────────────────────────────────────

  async getContainers(hostId: string): Promise<Container[]> {
    const { data } = await this.http.get<Container[]>(
      `/hosts/${hostId}/containers`,
    );
    return data;
  }

  async inspectContainer(
    hostId: string,
    containerId: string,
  ): Promise<ContainerDetail> {
    const { data } = await this.http.get<ContainerDetail>(
      `/hosts/${hostId}/containers/${containerId}/inspect`,
    );
    // Agents before 0.1.1 send null for an empty list.
    return {
      ...data,
      ports: data.ports ?? [],
      mounts: data.mounts ?? [],
      env: data.env ?? [],
      cmd: data.cmd ?? [],
    };
  }

  async inspectRawContainer(
    hostId: string,
    containerId: string,
  ): Promise<unknown> {
    const { data } = await this.http.get<unknown>(
      `/hosts/${hostId}/containers/${containerId}/raw`,
    );
    return data;
  }

  async startContainer(hostId: string, containerId: string): Promise<void> {
    await this.http.post(`/hosts/${hostId}/containers/${containerId}/start`);
  }

  async stopContainer(hostId: string, containerId: string): Promise<void> {
    await this.http.post(`/hosts/${hostId}/containers/${containerId}/stop`);
  }

  async restartContainer(hostId: string, containerId: string): Promise<void> {
    await this.http.post(`/hosts/${hostId}/containers/${containerId}/restart`);
  }

  async recreateContainer(
    hostId: string,
    containerId: string,
    ports: Array<{
      host_port: string;
      container_port: string;
      protocol: string;
    }>,
  ): Promise<{ id: string }> {
    const { data } = await this.http.post(
      `/hosts/${hostId}/containers/${containerId}/recreate`,
      { ports },
    );
    return data;
  }

  async updateContainerEnv(
    hostId: string,
    containerId: string,
    env: string[],
  ): Promise<{ id: string }> {
    const { data } = await this.http.post(
      `/hosts/${hostId}/containers/${containerId}/env`,
      { env },
    );
    return data;
  }

  async execContainer(
    hostId: string,
    containerId: string,
    cmd: string[],
  ): Promise<string> {
    const { data } = await this.http.post<string>(
      `/hosts/${hostId}/containers/${containerId}/exec`,
      { cmd },
      { responseType: "text" },
    );
    return data;
  }

  // ── Networks ──────────────────────────────────────────────────────────────

  async listHostNetworks(hostId: string): Promise<HostNetwork[]> {
    const { data } = await this.http.get<HostNetwork[]>(
      `/hosts/${hostId}/networks`,
    );
    return data;
  }

  async getContainerNetworks(
    hostId: string,
    containerId: string,
  ): Promise<ContainerNetworkInfo[]> {
    const { data } = await this.http.get<ContainerNetworkInfo[]>(
      `/hosts/${hostId}/containers/${containerId}/networks`,
    );
    return data;
  }

  async networkConnect(
    hostId: string,
    containerId: string,
    networkId: string,
  ): Promise<void> {
    await this.http.post(
      `/hosts/${hostId}/containers/${containerId}/networks/connect`,
      { network_id: networkId },
    );
  }

  async networkDisconnect(
    hostId: string,
    containerId: string,
    networkId: string,
  ): Promise<void> {
    await this.http.post(
      `/hosts/${hostId}/containers/${containerId}/networks/disconnect`,
      { network_id: networkId },
    );
  }

  // ── Prometheus proxy ──────────────────────────────────────────────────────

  async prometheusQuery(
    prometheusUrl: string,
    query: string,
  ): Promise<unknown> {
    const { data } = await this.http.get("/prometheus/query", {
      params: { url: prometheusUrl, q: query },
    });
    return data;
  }

  // ── System ────────────────────────────────────────────────────────────────

  async getSystemInfo(hostId: string): Promise<SystemInfo> {
    const { data } = await this.http.get<SystemInfo>(`/hosts/${hostId}/system`);
    return data;
  }

  async getHardwareMetrics(hostId: string): Promise<HardwareMetrics> {
    const { data } = await this.http.get<HardwareMetrics>(
      `/hosts/${hostId}/hardware`,
    );
    return data;
  }

  async getImages(hostId: string, opts?: PageOpts): Promise<Page<DockerImage>> {
    const { data } = await this.http.get<Page<DockerImage>>(
      `/hosts/${hostId}/images`,
      { params: pageParams(opts) },
    );
    return data;
  }

  async deleteImage(hostId: string, imageId: string): Promise<void> {
    await this.http.delete(`/hosts/${hostId}/images/${imageId}`);
  }

  async pruneDanglingImages(hostId: string): Promise<{ reclaimed: number }> {
    const { data } = await this.http.post<{ reclaimed: number }>(
      `/hosts/${hostId}/images/prune`,
    );
    return data;
  }

  async getVolumes(
    hostId: string,
    opts?: PageOpts,
  ): Promise<Page<DockerVolume>> {
    const { data } = await this.http.get<Page<DockerVolume>>(
      `/hosts/${hostId}/volumes`,
      { params: pageParams(opts) },
    );
    return data;
  }

  async deleteVolume(hostId: string, name: string): Promise<void> {
    await this.http.delete(
      `/hosts/${hostId}/volumes/${encodeURIComponent(name)}`,
    );
  }

  async pruneVolumes(hostId: string): Promise<{ reclaimed: number }> {
    const { data } = await this.http.post<{ reclaimed: number }>(
      `/hosts/${hostId}/volumes/prune`,
    );
    return data;
  }

  async getContainerLogs(
    hostId: string,
    containerId: string,
    tail = 200,
  ): Promise<string> {
    const { data } = await this.http.get<string>(
      `/hosts/${hostId}/containers/${containerId}/logs`,
      { params: { tail }, responseType: "text" },
    );
    return data;
  }

  // ── Stacks ────────────────────────────────────────────────────────────────

  async getDetectedStacks(hostId: string): Promise<Stack[]> {
    const { data } = await this.http.get<Stack[]>(
      `/hosts/${hostId}/stacks/detected`,
    );
    return data;
  }

  async getSavedStacks(hostId: string): Promise<SavedStack[]> {
    const { data } = await this.http.get<SavedStack[]>(
      `/hosts/${hostId}/stacks`,
    );
    return data;
  }

  async createSavedStack(
    hostId: string,
    name: string,
    content: string,
  ): Promise<SavedStack> {
    const { data } = await this.http.post<SavedStack>(
      `/hosts/${hostId}/stacks`,
      { name, content },
    );
    return data;
  }

  async updateSavedStack(
    id: string,
    content: string,
    envVars?: EnvVar[],
  ): Promise<SavedStack> {
    const { data } = await this.http.put<SavedStack>(`/stacks/${id}`, {
      content,
      env_vars: envVars ?? null,
    });
    return data;
  }

  async generateStackToken(id: string): Promise<{ token: string }> {
    const { data } = await this.http.post<{ token: string }>(
      `/stacks/${id}/token`,
    );
    return data;
  }

  async deleteStackToken(id: string): Promise<void> {
    await this.http.delete(`/stacks/${id}/token`);
  }

  async deleteSavedStack(id: string): Promise<void> {
    await this.http.delete(`/stacks/${id}`);
  }

  async stackUp(
    id: string,
  ): Promise<{ id: string; status: string; output: string }> {
    const { data } = await this.http.post(`/stacks/${id}/up`);
    return data;
  }

  async stackDown(
    id: string,
  ): Promise<{ id: string; status: string; output: string }> {
    const { data } = await this.http.post(`/stacks/${id}/down`);
    return data;
  }

  // ── Events ────────────────────────────────────────────────────────────────

  async getEvents(): Promise<ContainerEvent[]> {
    const { data } = await this.http.get<ContainerEvent[]>("/events");
    return data;
  }

  // ── Swarm ──────────────────────────────────────────────────────────────────

  async getSwarmServices(hostId: string): Promise<SwarmService[]> {
    const { data } = await this.http.get<SwarmService[]>(
      `/hosts/${hostId}/swarm/services`,
    );
    return data;
  }

  async getSwarmService(
    hostId: string,
    serviceId: string,
  ): Promise<SwarmService> {
    const { data } = await this.http.get<SwarmService>(
      `/hosts/${hostId}/swarm/services/${serviceId}`,
    );
    return data;
  }

  async getSwarmNodes(hostId: string): Promise<SwarmNode[]> {
    const { data } = await this.http.get<SwarmNode[]>(
      `/hosts/${hostId}/swarm/nodes`,
    );
    return data;
  }

  async getSwarmServiceTasks(
    hostId: string,
    serviceId: string,
  ): Promise<SwarmTask[]> {
    const { data } = await this.http.get<SwarmTask[]>(
      `/hosts/${hostId}/swarm/services/${serviceId}/tasks`,
    );
    return data;
  }

  // ── Webhooks ─────────────────────────────────────────────────────────────────

  async webhookRecreate(
    hostId: string,
    containerId: string,
    pull = false,
  ): Promise<{ id: string; pulled: boolean }> {
    const { data } = await this.http.post(
      `/webhooks/${hostId}/${containerId}/recreate`,
      null,
      { params: { pull } },
    );
    return data;
  }

  async webhookRedeployStack(
    hostId: string,
    stackId: string,
    pull = true,
    token?: string,
  ): Promise<{ id: string; status: string; output: string }> {
    const { data } = await this.http.post(
      `/webhooks/${hostId}/stacks/${stackId}/redeploy`,
      null,
      { params: { pull, token } },
    );
    return data;
  }

  async updateSwarm(
    hostId: string,
    serviceId: string,
  ): Promise<{ service: string; updated: boolean }> {
    const { data } = await this.http.post(
      `/hosts/${hostId}/swarm/services/${serviceId}/update`,
      null
    );
    return data;
  }

  async scaleSwarm(
    hostId: string,
    serviceId: string,
    count: number,
  ): Promise<{ service: string; updated: boolean }> {
    const { data } = await this.http.post(
      `/hosts/${hostId}/swarm/services/${serviceId}/scale`,
      null,
      { params: { target_replicas: count } },
    );
    return data;
  }

  async webhookRolloutK8s(
    hostId: string,
    namespace: string,
    deployment: string,
    token?: string,
  ): Promise<{ namespace: string; deployment: string; restarted: boolean }> {
    const { data } = await this.http.post(
      `/webhooks/${hostId}/k8s/${namespace}/${deployment}/rollout`,
      null,
      { params: { token } },
    );
    return data;
  }

  // ── Webhook token management ──────────────────────────────────────────────

  async getWebhookToken(
    hostId: string,
    entityType: string,
    entityKey: string,
  ): Promise<{ enabled: boolean; token: string | null }> {
    const { data } = await this.http.get(
      `/webhooks/${hostId}/${entityType}/${encodeURIComponent(entityKey)}/token`,
    );
    return data;
  }

  async generateWebhookToken(
    hostId: string,
    entityType: string,
    entityKey: string,
  ): Promise<{ token: string }> {
    const { data } = await this.http.post(
      `/webhooks/${hostId}/${entityType}/${encodeURIComponent(entityKey)}/token`,
    );
    return data;
  }

  async deleteWebhookToken(
    hostId: string,
    entityType: string,
    entityKey: string,
  ): Promise<void> {
    await this.http.delete(
      `/webhooks/${hostId}/${entityType}/${encodeURIComponent(entityKey)}/token`,
    );
  }

  // ── Kubernetes ─────────────────────────────────────────────────────────────

  async getK8sPods(hostId: string): Promise<K8sPod[]> {
    const { data } = await this.http.get<K8sPod[]>(
      `/hosts/${hostId}/k8s/pods`,
    );
    return data;
  }

  async getK8sNamespaces(hostId: string): Promise<K8sNamespace[]> {
    const { data } = await this.http.get<K8sNamespace[]>(
      `/hosts/${hostId}/k8s/namespaces`,
    );
    return data;
  }

  async getK8sWorkloads(hostId: string): Promise<K8sWorkload[]> {
    const { data } = await this.http.get<K8sWorkload[]>(
      `/hosts/${hostId}/k8s/workloads`,
    );
    return data;
  }

  async getK8sNodes(hostId: string): Promise<K8sNode[]> {
    const { data } = await this.http.get<K8sNode[]>(`/hosts/${hostId}/k8s/nodes`);
    return data;
  }

  async getK8sServices(hostId: string): Promise<K8sService[]> {
    const { data } = await this.http.get<K8sService[]>(`/hosts/${hostId}/k8s/services`);
    return data;
  }

  /** The cluster's most recent events, newest first. */
  async getK8sEvents(hostId: string): Promise<K8sEvent[]> {
    const { data } = await this.http.get<K8sEvent[]>(`/hosts/${hostId}/k8s/events`);
    return data;
  }

  /** container may be omitted for a single-container pod. */
  async getK8sPodLogs(
    hostId: string,
    namespace: string,
    pod: string,
    container?: string,
    tail = 200,
  ): Promise<string> {
    const { data } = await this.http.get<string>(
      `/hosts/${hostId}/k8s/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(pod)}/logs`,
      { params: { container, tail }, responseType: "text" },
    );
    return data;
  }

  async scaleK8sWorkload(
    hostId: string,
    w: { namespace: string; kind: string; name: string },
    replicas: number,
  ): Promise<void> {
    await this.http.post(`${k8sWorkloadPath(hostId, w)}/scale`, null, {
      params: { replicas },
    });
  }

  async restartK8sWorkload(
    hostId: string,
    w: { namespace: string; kind: string; name: string },
  ): Promise<void> {
    await this.http.post(`${k8sWorkloadPath(hostId, w)}/restart`);
  }

  async deleteK8sPod(
    hostId: string,
    namespace: string,
    name: string,
  ): Promise<void> {
    await this.http.delete(
      `/hosts/${hostId}/k8s/pods/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`,
    );
  }
}

function k8sWorkloadPath(
  hostId: string,
  w: { namespace: string; kind: string; name: string },
): string {
  return `/hosts/${hostId}/k8s/workloads/${encodeURIComponent(w.namespace)}/${encodeURIComponent(w.kind)}/${encodeURIComponent(w.name)}`;
}

export const api = new PulseClient();
