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

export interface EnvVar {
  key: string;
  value: string;
}

export interface SavedStack {
  id: string;
  host_id: string;
  name: string;
  content: string;
  env_vars: string | null;
  webhook_token: string | null;
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

export interface HostNetwork {
  id: string;
  name: string;
  driver: string;
  scope: string;
  internal: boolean;
}

export interface ContainerNetworkInfo {
  id: string;
  name: string;
  ip_address: string;
  gateway: string;
  mac_address: string;
}

// Images
export interface DockerImage {
  id: string;
  tags: string[];
  size: number;
  created: number;
  dangling: boolean;
}

// Volumes
export interface DockerVolume {
  name: string;
  driver: string;
  mountpoint: string;
  created: string;
  in_use: boolean;
}

// System info
export interface SystemInfo {
  docker_version: string;
  api_version: string;
  os: string;
  arch: string;
  kernel: string;
  total_memory: number;
  cpus: number;
  containers_running: number;
  containers_stopped: number;
  images: number;
}

// Hardware metrics
export interface DiskInfo {
  name: string;
  mount: string;
  total: number;
  used: number;
  pct: number;
}

export interface HardwareMetrics {
  mem_used: number;
  mem_total: number;
  mem_pct: number;
  disks: DiskInfo[];
}
