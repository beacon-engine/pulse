import type { K8sPod, PodContainer } from "../types/containers";
import type { Environment } from "../types/environments";

/** A status pill: label, colour class, and whether to show the live dot. */
export interface StatusPill {
  label: string;
  pill: "pill-green" | "pill-red" | "pill-amber" | "pill-mist";
  live: boolean;
}

/** A pod container's status, worded like the Containers page:
 *  Running, Waiting (CrashLoopBackOff), Exited (1), Completed. */
export function containerStatus(c: PodContainer): StatusPill {
  switch (c.state) {
    case "running":
      return c.ready
        ? { label: "Running", pill: "pill-green", live: true }
        : { label: "Running · not ready", pill: "pill-amber", live: false };
    case "waiting": {
      const bad = /BackOff|Err|Invalid/i.test(c.reason ?? "");
      return { label: c.reason || "Waiting", pill: bad ? "pill-red" : "pill-amber", live: false };
    }
    case "terminated":
      if (c.exit_code === 0) return { label: c.reason || "Completed", pill: "pill-mist", live: false };
      return {
        label: `${c.reason || "Exited"} (${c.exit_code ?? "?"})`,
        pill: "pill-red",
        live: false,
      };
  }
  return { label: "Unknown", pill: "pill-mist", live: false };
}

/** A pod's status as `kubectl get pods` shows it. */
export function podStatus(p: K8sPod): StatusPill {
  const label = p.reason || p.status;
  if (p.reason === "Completed" || p.status === "Succeeded") return { label, pill: "pill-mist", live: false };
  if (p.reason === "Terminating") return { label, pill: "pill-amber", live: false };
  if (p.status === "Failed" || /BackOff|Err|Evicted|OOM/i.test(p.reason ?? "")) {
    return { label, pill: "pill-red", live: false };
  }
  if (p.status === "Running" && p.ready && !p.reason) return { label, pill: "pill-green", live: true };
  return { label, pill: "pill-amber", live: false };
}

export function podRestarts(p: K8sPod): number {
  return p.containers.reduce((n, c) => n + c.restart_count, 0);
}

export function podReadyCount(p: K8sPod): string {
  return `${p.containers.filter((c) => c.ready).length}/${p.containers.length}`;
}

/** "8080/TCP, 9090/TCP" — the ports a container declares. */
export function containerPorts(c: PodContainer): string {
  if (!c.ports?.length) return "—";
  return c.ports.map((p) => `${p.container_port}/${p.protocol}`).join(", ");
}

/** How the gateway reaches the agent for an environment. */
export interface Connection {
  mode: "push" | "pull" | "direct";
  label: string;
  detail: string;
}

export function connectionOf(env: Environment | null | undefined): Connection {
  if (env?.agent_id) {
    return {
      mode: "push",
      label: "Push agent",
      detail: `The agent in the cluster dials the gateway (outbound only). Agent ${env.agent_id}`,
    };
  }
  if (env?.agent_addr) {
    return {
      mode: "pull",
      label: "Pull agent",
      detail: `The gateway dials the agent at ${env.agent_addr}`,
    };
  }
  return { mode: "direct", label: "Default agent", detail: "Reached through the gateway's default agent" };
}

/** Kubernetes memory quantities ("16310588Ki", "64Mi") as GiB/MiB. */
export function fmtMemory(q?: string): string {
  if (!q) return "—";
  const m = /^(\d+(?:\.\d+)?)([KMGTE]i?)?$/.exec(q);
  if (!m) return q;
  const unit: Record<string, number> = {
    "": 1, K: 1e3, M: 1e6, G: 1e9, T: 1e12, E: 1e18,
    Ki: 1024, Mi: 1024 ** 2, Gi: 1024 ** 3, Ti: 1024 ** 4, Ei: 1024 ** 6,
  };
  const bytes = parseFloat(m[1]) * (unit[m[2] ?? ""] ?? 1);
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GiB`;
  return `${Math.round(bytes / 1024 ** 2)} MiB`;
}
