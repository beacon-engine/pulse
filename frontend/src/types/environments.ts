export interface Environment {
  id: string;
  name: string;
  docker_host: string;
  is_local: boolean;
  created_at: string;
  agent_id: string | null;
  agent_addr: string | null;
  source: "manual" | "auto";
  status: "online" | "offline";
  last_seen: string | null;
}

/** A runtime an environment's agent can drive. Docker and Podman share the
 *  Docker-compatible API; containerd will be its own kind. */
export type RuntimeKind = "docker" | "podman" | "kubernetes" | "containerd";

export interface RuntimeInfo {
  kind: RuntimeKind;
  version: string;
  endpoint?: string;
  features: string[]; // e.g. "swarm", "compose"
}

export interface HostCapabilities {
  /** false when the agent itself couldn't be reached. */
  reachable: boolean;
  error?: string;
  runtimes: RuntimeInfo[];
  /** Runtimes the agent looked for but couldn't reach, with the reason. */
  unavailable: { kind: string; error: string }[];
}

/** What a page needs from the environment to be useful. */
export type Requirement = "containers" | "swarm" | "compose" | "kubernetes";

export function satisfies(caps: HostCapabilities, req: Requirement): boolean {
  const container = caps.runtimes.find((r) => r.kind === "docker" || r.kind === "podman");
  switch (req) {
    case "containers":
      return !!container;
    case "swarm":
      return !!container?.features.includes("swarm");
    case "compose":
      return !!container?.features.includes("compose");
    case "kubernetes":
      return caps.runtimes.some((r) => r.kind === "kubernetes");
  }
}

export const RUNTIME_LABEL: Record<string, string> = {
  docker: "Docker",
  podman: "Podman",
  kubernetes: "Kubernetes",
  containerd: "containerd",
  container: "Container runtime",
};
