import type { DockerImage, DockerVolume, HostNetwork, SystemInfo } from "../types/containers";
import { api } from "../lib/api";
import { hostsStore } from "./hosts.svelte";
import { toast } from "../lib/toast.svelte";

export type ResourceTab = "images" | "volumes" | "networks" | "system";

class ResourcesStore {
  tab = $state<ResourceTab>("images");

  // System Info
  sysInfo = $state<SystemInfo | null>(null);
  sysError = $state("");
  sysLoading = $state(false);

  // Images
  images = $state<DockerImage[]>([]);
  imgTotal = $state(0);
  imgPage = $state(1);
  imgSearch = $state("");
  imgError = $state("");
  imgBusy = $state(false);

  // Volumes
  volumes = $state<DockerVolume[]>([]);
  volTotal = $state(0);
  volPage = $state(1);
  volSearch = $state("");
  volError = $state("");
  volBusy = $state(false);

  // Networks
  networks = $state<HostNetwork[]>([]);
  netError = $state("");
  netBusy = $state(false);

  // Derived image stats
  danglingCount = $derived(this.images.filter((i) => i.dangling).length);
  danglingSize = $derived(
    this.images.filter((i) => i.dangling).reduce((a, i) => a + i.size, 0)
  );

  async loadImages(perPage = 50) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.imgBusy = true;
    this.imgError = "";
    try {
      const p = await api.getImages(hostId, {
        page: this.imgPage,
        perPage,
        search: this.imgSearch,
      });
      this.images = p.items;
      this.imgTotal = p.total;
    } catch (e: any) {
      this.imgError = e?.response?.data ?? String(e);
      toast.error(`Image error: ${this.imgError}`);
    } finally {
      this.imgBusy = false;
    }
  }

  async deleteImage(id: string) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.imgBusy = true;
    try {
      await api.deleteImage(hostId, id);
      toast.info("Image deleted");
      await this.loadImages();
    } catch (e: any) {
      this.imgError = e?.response?.data ?? String(e);
      toast.error(this.imgError);
      this.imgBusy = false;
    }
  }

  async pruneImages() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.imgBusy = true;
    try {
      const r = await api.pruneDanglingImages(hostId);
      await this.loadImages();
      toast.success("Pruned images");
    } catch (e: any) {
      this.imgError = e?.response?.data ?? String(e);
      toast.error(this.imgError);
      this.imgBusy = false;
    }
  }

  async loadVolumes(perPage = 50) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.volBusy = true;
    this.volError = "";
    try {
      const p = await api.getVolumes(hostId, {
        page: this.volPage,
        perPage,
        search: this.volSearch,
      });
      this.volumes = p.items;
      this.volTotal = p.total;
    } catch (e: any) {
      this.volError = e?.response?.data ?? String(e);
      toast.error(`Volume error: ${this.volError}`);
    } finally {
      this.volBusy = false;
    }
  }

  async deleteVolume(name: string) {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.volBusy = true;
    try {
      await api.deleteVolume(hostId, name);
      toast.info("Volume deleted");
      await this.loadVolumes();
    } catch (e: any) {
      this.volError = e?.response?.data ?? String(e);
      toast.error(this.volError);
      this.volBusy = false;
    }
  }

  async pruneVolumes() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.volBusy = true;
    try {
      await api.pruneVolumes(hostId);
      await this.loadVolumes();
      toast.success("Pruned unused volumes");
    } catch (e: any) {
      this.volError = e?.response?.data ?? String(e);
      toast.error(this.volError);
      this.volBusy = false;
    }
  }

  async loadNetworks() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.netBusy = true;
    this.netError = "";
    try {
      this.networks = await api.listHostNetworks(hostId);
    } catch (e: any) {
      this.netError = e?.response?.data ?? String(e);
      toast.error(`Network error: ${this.netError}`);
    } finally {
      this.netBusy = false;
    }
  }

  async loadSystem() {
    const hostId = hostsStore.activeHostId;
    if (!hostId) return;
    this.sysLoading = true;
    this.sysError = "";
    try {
      this.sysInfo = await api.getSystemInfo(hostId);
    } catch (e: any) {
      this.sysError = e?.response?.data ?? String(e);
      toast.error(`System info error: ${this.sysError}`);
    } finally {
      this.sysLoading = false;
    }
  }

  /** The environment the lists were loaded for. */
  private loadedFor: string | null = null;

  async loadActiveTab() {
    // Another environment's lists would show until the new ones arrive.
    if (hostsStore.activeHostId !== this.loadedFor) {
      this.loadedFor = hostsStore.activeHostId;
      this.sysInfo = null;
      this.images = [];
      this.imgTotal = 0;
      this.volumes = [];
      this.volTotal = 0;
      this.networks = [];
    }
    if (this.tab === "images") await this.loadImages();
    else if (this.tab === "volumes") await this.loadVolumes();
    else if (this.tab === "networks") await this.loadNetworks();
    else if (this.tab === "system") await this.loadSystem();
  }
}

export const resourcesStore = new ResourcesStore();
