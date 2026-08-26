import { PollingStore } from "./base.svelte";
import type { HardwareMetrics } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

export interface MetricSample {
  name: string;
  labels: Record<string, string>;
  value: number;
  help: string;
  type: string;
}

class MetricsStore extends PollingStore {
  hw = $state<HardwareMetrics | null>(null);
  hwError = $state("");
  rawMetrics = $state("");
  samples = $state<MetricSample[]>([]);
  metricsError = $state("");
  metricsLoading = $state(false);

  private parsePrometheus(text: string): MetricSample[] {
    const lines = text.split("\n");
    const helps: Record<string, string> = {};
    const types: Record<string, string> = {};
    const result: MetricSample[] = [];

    for (const line of lines) {
      const t = line.trim();
      if (!t || t.startsWith("#")) {
        const parts = t.split(" ");
        if (parts[1] === "HELP") helps[parts[2]] = parts.slice(3).join(" ");
        if (parts[1] === "TYPE") types[parts[2]] = parts[3];
        continue;
      }
      const brace = t.indexOf("{");
      const space = t.lastIndexOf(" ");
      if (space === -1) continue;
      const valueStr = t.slice(space + 1);
      const value = parseFloat(valueStr);
      if (isNaN(value)) continue;

      let name: string;
      let labels: Record<string, string> = {};
      if (brace !== -1 && brace < space) {
        name = t.slice(0, brace);
        const lStr = t.slice(brace + 1, t.lastIndexOf("}"));
        for (const pair of lStr.matchAll(/(\w+)="([^"]*)"/g)) {
          labels[pair[1]] = pair[2];
        }
      } else {
        name = t.slice(0, space);
      }
      result.push({
        name,
        labels,
        value,
        help: helps[name] ?? "",
        type: types[name] ?? "",
      });
    }
    return result;
  }

  async loadHardware() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    try {
      this.hw = await api.getHardwareMetrics(hostId);
      this.hwError = "";
    } catch (e: any) {
      this.hwError = e?.response?.data ?? String(e);
    }
  }

  async loadPulseMetrics() {
    this.metricsLoading = true;
    this.metricsError = "";
    try {
      const resp = await fetch("/metrics");
      this.rawMetrics = await resp.text();
      this.samples = this.parsePrometheus(this.rawMetrics);
    } catch (e: any) {
      this.metricsError = String(e);
      toast.error(`Metrics load error: ${this.metricsError}`);
    } finally {
      this.metricsLoading = false;
    }
  }

  async refresh() {
    await Promise.all([this.loadHardware(), this.loadPulseMetrics()]);
  }
}

export const metricsStore = new MetricsStore();
