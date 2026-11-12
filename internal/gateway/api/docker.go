package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// Ports gateway/src/api/docker.rs — per-host Docker/Swarm operations. Each
// resolves the host to a docker_host and forwards a Request to the agent,
// translating the Response back into an HTTP result.

// req is a small builder to keep handlers to one line.
func req(kind, dockerHost string) protocol.Request {
	return protocol.Request{Kind: kind, DockerHost: dockerHost}
}

// ── Containers ───────────────────────────────────────────────────────────────

func (st *State) handleListContainers(w http.ResponseWriter, r *http.Request) {
	var v []protocol.ContainerInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return req(protocol.ReqListContainers, dh) },
		protocol.RespKindContainers, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	var v protocol.ContainerDetail
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqInspectContainer, DockerHost: dh, ID: cid}
		},
		protocol.RespKindContainerDetail, &v) {
		writeJSON(w, http.StatusOK, v)
	}
}

func (st *State) handleInspectRawContainer(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	var raw json.RawMessage
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqInspectRawContainer, DockerHost: dh, ID: cid}
		},
		protocol.RespKindRaw, &raw) {
		writeJSON(w, http.StatusOK, raw)
	}
}

func (st *State) handleRecreateContainer(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	var body protocol.RecreateBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	var id string
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqRecreateContainer, DockerHost: dh, ID: cid, Ports: body.Ports}
		},
		protocol.RespKindID, &id) {
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
	}
}

func (st *State) handleUpdateContainerEnv(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	var body protocol.EnvBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	var id string
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqUpdateContainerEnv, DockerHost: dh, ID: cid, Env: body.Env}
		},
		protocol.RespKindID, &id) {
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
	}
}

func (st *State) handleExecContainer(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	var body protocol.ExecBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	var out string
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqExecContainer, DockerHost: dh, ID: cid, Cmd: body.Cmd}
		},
		protocol.RespKindText, &out) {
		writeText(w, out)
	}
}

func (st *State) handleStartContainer(w http.ResponseWriter, r *http.Request) {
	st.containerAction(w, r, protocol.ReqStartContainer, "start")
}
func (st *State) handleStopContainer(w http.ResponseWriter, r *http.Request) {
	st.containerAction(w, r, protocol.ReqStopContainer, "stop")
}
func (st *State) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
	st.containerAction(w, r, protocol.ReqRestartContainer, "restart")
}

// containerAction handles start/stop/restart: forward, expect Ok, fire the Slack
// alert, return 204.
func (st *State) containerAction(w http.ResponseWriter, r *http.Request, kind, alert string) {
	cid := r.PathValue("cid")
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return protocol.Request{Kind: kind, DockerHost: dh, ID: cid} },
		protocol.RespKindOk, nil) {
		st.Notifier.AlertContainer(alert, cid)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (st *State) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	tail := uint32(clampInt(atoiDefault(r.URL.Query().Get("tail"), 200), 1, 2000))
	var out string
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqContainerLogs, DockerHost: dh, ID: cid, Tail: tail}
		},
		protocol.RespKindText, &out) {
		writeText(w, out)
	}
}

func (st *State) handleContainerNetworks(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	var v []protocol.ContainerNetworkInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqContainerNetworks, DockerHost: dh, ID: cid}
		},
		protocol.RespKindContainerNetworks, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleNetworkConnect(w http.ResponseWriter, r *http.Request) {
	st.networkAction(w, r, protocol.ReqNetworkConnect)
}
func (st *State) handleNetworkDisconnect(w http.ResponseWriter, r *http.Request) {
	st.networkAction(w, r, protocol.ReqNetworkDisconnect)
}

func (st *State) networkAction(w http.ResponseWriter, r *http.Request, kind string) {
	cid := r.PathValue("cid")
	var body protocol.NetworkBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: kind, DockerHost: dh, NetworkID: body.NetworkID, ContainerID: cid}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── System / hardware ────────────────────────────────────────────────────────

func (st *State) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	var v protocol.SystemInfo
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return req(protocol.ReqSystemInfo, dh) },
		protocol.RespKindSystemInfo, &v) {
		writeJSON(w, http.StatusOK, v)
	}
}

func (st *State) handleHardwareMetrics(w http.ResponseWriter, r *http.Request) {
	var v protocol.HardwareMetrics
	if st.proxy(w, r, r.PathValue("id"),
		func(string) protocol.Request { return protocol.Request{Kind: protocol.ReqHardwareMetrics} },
		protocol.RespKindHardwareMetrics, &v) {
		writeJSON(w, http.StatusOK, v)
	}
}

// ── Images ───────────────────────────────────────────────────────────────────

func (st *State) handleListImages(w http.ResponseWriter, r *http.Request) {
	var all []protocol.ImageInfo
	if !st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return req(protocol.ReqListImages, dh) },
		protocol.RespKindImages, &all) {
		return
	}
	page, perPage := pageBounds(r)
	if s := searchTerm(r); s != "" {
		filtered := all[:0]
		for _, img := range all {
			if strings.Contains(strings.ToLower(img.ID), s) || anyContains(img.Tags, s) {
				filtered = append(filtered, img)
			}
		}
		all = filtered
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Created > all[j].Created }) // newest first
	writeJSON(w, http.StatusOK, paginate(all, page, perPage))
}

func (st *State) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	imageID := r.PathValue("image_id")
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqDeleteImage, DockerHost: dh, ID: imageID}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (st *State) handlePruneImages(w http.ResponseWriter, r *http.Request) {
	st.pruneReclaimed(w, r, protocol.ReqPruneImages)
}
func (st *State) handlePruneVolumes(w http.ResponseWriter, r *http.Request) {
	st.pruneReclaimed(w, r, protocol.ReqPruneVolumes)
}

func (st *State) pruneReclaimed(w http.ResponseWriter, r *http.Request, kind string) {
	var n int64
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return req(kind, dh) },
		protocol.RespKindCount, &n) {
		writeJSON(w, http.StatusOK, map[string]int64{"reclaimed": n})
	}
}

// ── Volumes ──────────────────────────────────────────────────────────────────

func (st *State) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	var all []protocol.VolumeInfo
	if !st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return req(protocol.ReqListVolumes, dh) },
		protocol.RespKindVolumes, &all) {
		return
	}
	page, perPage := pageBounds(r)
	if s := searchTerm(r); s != "" {
		filtered := all[:0]
		for _, v := range all {
			if strings.Contains(strings.ToLower(v.Name), s) || strings.Contains(strings.ToLower(v.Driver), s) {
				filtered = append(filtered, v)
			}
		}
		all = filtered
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	writeJSON(w, http.StatusOK, paginate(all, page, perPage))
}

func (st *State) handleDeleteVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("vol_name")
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqDeleteVolume, DockerHost: dh, Name: name}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── Networks / detected stacks ───────────────────────────────────────────────

func (st *State) handleListHostNetworks(w http.ResponseWriter, r *http.Request) {
	var v []protocol.HostNetwork
	if st.proxy(w, r, r.PathValue("id"),
		func(dh string) protocol.Request { return req(protocol.ReqListHostNetworks, dh) },
		protocol.RespKindHostNetworks, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleListDetectedStacks(w http.ResponseWriter, r *http.Request) {
	var v []protocol.Stack
	if st.proxy(w, r, r.PathValue("host_id"),
		func(dh string) protocol.Request { return req(protocol.ReqListDetectedStacks, dh) },
		protocol.RespKindDetectedStacks, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

// ── Swarm ────────────────────────────────────────────────────────────────────

func (st *State) handleListServices(w http.ResponseWriter, r *http.Request) {
	var v []protocol.SwarmService
	if st.proxy(w, r, r.PathValue("host_id"),
		func(dh string) protocol.Request { return req(protocol.ReqListSwarmServices, dh) },
		protocol.RespKindSwarmServices, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleListNodes(w http.ResponseWriter, r *http.Request) {
	var v []protocol.SwarmNode
	if st.proxy(w, r, r.PathValue("host_id"),
		func(dh string) protocol.Request { return req(protocol.ReqListSwarmNodes, dh) },
		protocol.RespKindSwarmNodes, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleListServiceTasks(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("service_id")
	var v []protocol.SwarmTask
	if st.proxy(w, r, r.PathValue("host_id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqListSwarmTasks, DockerHost: dh, ServiceID: sid}
		},
		protocol.RespKindSwarmTasks, &v) {
		writeJSON(w, http.StatusOK, orEmpty(v))
	}
}

func (st *State) handleUpdateSwarmService(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("service_id")
	if st.proxy(w, r, r.PathValue("host_id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqUpdateSwarmService, DockerHost: dh, ServiceID: sid}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (st *State) handleScaleSwarmService(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("service_id")
	replicas := uint64(clampInt(atoiDefault(r.URL.Query().Get("target_replicas"), 1), 0, 1<<20))
	if st.proxy(w, r, r.PathValue("host_id"),
		func(dh string) protocol.Request {
			return protocol.Request{Kind: protocol.ReqScaleSwarmService, DockerHost: dh, ServiceID: sid, TargetReplicas: replicas}
		},
		protocol.RespKindOk, nil) {
		w.WriteHeader(http.StatusNoContent)
	}
}

// ── small helpers ────────────────────────────────────────────────────────────

func writeText(w http.ResponseWriter, s string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func anyContains(items []string, sub string) bool {
	for _, s := range items {
		if strings.Contains(strings.ToLower(s), sub) {
			return true
		}
	}
	return false
}
