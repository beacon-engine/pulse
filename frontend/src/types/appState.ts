import type { Container, ContainerEvent, K8sNamespace, K8sPod, SavedStack, Stack, SwarmNode, SwarmService } from './containers';
import type { Environment } from './environments';

export interface AppState {
  hosts: Environment[];
  activeHostId: string | null;
  activeHost: Environment | null;
  selectedContainer: Container | null;
  containers: Container[];
  stacks: Stack[];
  savedStacks: SavedStack[];
  events: ContainerEvent[];
  pods: K8sPod[];
  namespaces: K8sNamespace[];
  swarmServices: SwarmService[];
  swarmNodes: SwarmNode[];
  loading: boolean;
  k8sLoading: boolean;
  swarmLoading: boolean;
  error: string;
  k8sError: string;
  swarmError: string;
  lastUpdated: string;
  refresh: () => Promise<void>;
  refreshK8s: () => Promise<void>;
  refreshSwarm: () => Promise<void>;
  switchHost: (host: Environment) => Promise<void>;
  startContainer: (id: string) => Promise<void>;
  stopContainer: (id: string) => Promise<void>;
  restartContainer: (id: string) => Promise<void>;
  toast: (msg: string, kind?: 'ok' | 'warn' | 'err') => void;
}
