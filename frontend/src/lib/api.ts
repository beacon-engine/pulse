import axios from 'axios';
import type { AxiosInstance, CreateAxiosDefaults } from 'axios';
import type { Container, ContainerDetail, ContainerEvent, K8sNamespace, K8sPod, SavedStack, Stack, SwarmNode, SwarmService } from '../types/containers';
import type { Environment } from '../types/environments';

export type { Container, ContainerDetail, ContainerEvent, K8sNamespace, K8sPod, SavedStack, Stack, SwarmNode, SwarmService, Environment };

export class BeaconClient {
  private http: AxiosInstance;

  constructor(config: CreateAxiosDefaults = {}) {
    this.http = axios.create({
      baseURL: '/api',
      headers: { 'Content-Type': 'application/json' },
      ...config,
    });
  }

  // ── Hosts ─────────────────────────────────────────────────────────────────

  async listHosts(): Promise<Environment[]> {
    const { data } = await this.http.get<Environment[]>('/hosts');
    return data;
  }

  async createHost(name: string, dockerHost: string): Promise<Environment> {
    const { data } = await this.http.post<Environment>('/hosts', { name, docker_host: dockerHost });
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
    const { data } = await this.http.get<Container[]>(`/hosts/${hostId}/containers`);
    return data;
  }

  async inspectContainer(hostId: string, containerId: string): Promise<ContainerDetail> {
    const { data } = await this.http.get<ContainerDetail>(
      `/hosts/${hostId}/containers/${containerId}/inspect`,
    );
    return data;
  }

  async inspectRawContainer(hostId: string, containerId: string): Promise<unknown> {
    const { data } = await this.http.get<unknown>(`/hosts/${hostId}/containers/${containerId}/raw`);
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
    ports: Array<{ host_port: string; container_port: string; protocol: string }>,
  ): Promise<{ id: string }> {
    const { data } = await this.http.post(`/hosts/${hostId}/containers/${containerId}/recreate`, { ports });
    return data;
  }

  async updateContainerEnv(hostId: string, containerId: string, env: string[]): Promise<{ id: string }> {
    const { data } = await this.http.post(`/hosts/${hostId}/containers/${containerId}/env`, { env });
    return data;
  }

  async getContainerLogs(hostId: string, containerId: string, tail = 200): Promise<string> {
    const { data } = await this.http.get<string>(
      `/hosts/${hostId}/containers/${containerId}/logs`,
      { params: { tail }, responseType: 'text' },
    );
    return data;
  }

  // ── Stacks ────────────────────────────────────────────────────────────────

  async getDetectedStacks(hostId: string): Promise<Stack[]> {
    const { data } = await this.http.get<Stack[]>(`/hosts/${hostId}/stacks/detected`);
    return data;
  }

  async getSavedStacks(hostId: string): Promise<SavedStack[]> {
    const { data } = await this.http.get<SavedStack[]>(`/hosts/${hostId}/stacks`);
    return data;
  }

  async createSavedStack(hostId: string, name: string, content: string): Promise<SavedStack> {
    const { data } = await this.http.post<SavedStack>(`/hosts/${hostId}/stacks`, { name, content });
    return data;
  }

  async updateSavedStack(id: string, content: string): Promise<SavedStack> {
    const { data } = await this.http.put<SavedStack>(`/stacks/${id}`, { content });
    return data;
  }

  async deleteSavedStack(id: string): Promise<void> {
    await this.http.delete(`/stacks/${id}`);
  }

  async stackUp(id: string): Promise<{ id: string; status: string; output: string }> {
    const { data } = await this.http.post(`/stacks/${id}/up`);
    return data;
  }

  async stackDown(id: string): Promise<{ id: string; status: string; output: string }> {
    const { data } = await this.http.post(`/stacks/${id}/down`);
    return data;
  }

  // ── Events ────────────────────────────────────────────────────────────────

  async getEvents(): Promise<ContainerEvent[]> {
    const { data } = await this.http.get<ContainerEvent[]>('/events');
    return data;
  }

  // ── Swarm ──────────────────────────────────────────────────────────────────

  async getSwarmServices(hostId: string): Promise<SwarmService[]> {
    const { data } = await this.http.get<SwarmService[]>(`/hosts/${hostId}/swarm/services`);
    return data;
  }

  async getSwarmNodes(hostId: string): Promise<SwarmNode[]> {
    const { data } = await this.http.get<SwarmNode[]>(`/hosts/${hostId}/swarm/nodes`);
    return data;
  }

  // ── Webhooks ─────────────────────────────────────────────────────────────────

  async webhookRecreate(hostId: string, containerId: string, pull = false): Promise<{ id: string; pulled: boolean }> {
    const { data } = await this.http.post(`/webhooks/${hostId}/${containerId}/recreate`, null, {
      params: { pull },
    });
    return data;
  }

  // ── Kubernetes ─────────────────────────────────────────────────────────────

  async getK8sPods(): Promise<K8sPod[]> {
    const { data } = await this.http.get<K8sPod[]>('/k8s/pods');
    return data;
  }

  async getK8sNamespaces(): Promise<K8sNamespace[]> {
    const { data } = await this.http.get<K8sNamespace[]>('/k8s/namespaces');
    return data;
  }
}

export const api = new BeaconClient();
