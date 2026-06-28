export interface Container {
  id: string;
  name: string;
  image: string;
  status: string;
  state: string;
  stack?: string;
}

export enum ContainerFilterStatus {
  All = 'all',
  Running = 'running',
  Stopped = 'stopped',
}

export interface ContainerPort {
  container_port: string;
  protocol: string;
  host_ip: string;
  host_port: string;
}

export interface ContainerMount {
  source: string;
  destination: string;
  mode: string;
  rw: boolean;
}

export interface ContainerDetail {
  id: string;
  name: string;
  image: string;
  created: string;
  state: string;
  started_at: string;
  finished_at: string;
  restart_count: number;
  ports: ContainerPort[];
  mounts: ContainerMount[];
  env: string[];
  cmd: string[];
  network_mode: string;
  restart_policy: string;
}

export interface ContainerEvent {
  id: string;
  container_id: string;
  container_name: string;
  event: string;
  occurred_at: string;
}

export interface WsEvent {
  event: 'start' | 'stop' | 'restart';
  container: string;
}

export interface Stack {
  name: string;
  services: Container[];
}

export interface SavedStack {
  id: string;
  host_id: string;
  name: string;
  content: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface K8sPod {
  name: string;
  namespace: string;
  status: string;
  ready: boolean;
}

export interface K8sNamespace {
  name: string;
  status: string;
}

export interface SwarmService {
  id: string;
  name: string;
  image: string;
  replicas: number;
  running: number;
  desired_state: string;
}

export interface SwarmNode {
  id: string;
  hostname: string;
  role: string;
  state: string;
  availability: string;
  addr: string;
}
