package agent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/swarm"
	"k8s.io/apimachinery/pkg/version"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// probeTimeout bounds each runtime probe so one unreachable runtime (a dead
// tcp:// daemon, a cluster behind a dropped VPN) can't stall the answer.
const probeTimeout = 3 * time.Second

// capabilities probes every runtime this agent knows how to drive, in parallel,
// and reports which ones answered. dockerHost is the environment's container
// endpoint ("" = the agent's local default).
func capabilities(ctx context.Context, st *State, dockerHost string) protocol.Capabilities {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		caps = protocol.Capabilities{Runtimes: []protocol.RuntimeInfo{}, Unavailable: []protocol.RuntimeProbe{}}
	)
	record := func(rt *protocol.RuntimeInfo, kind string, err error) {
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			caps.Unavailable = append(caps.Unavailable, protocol.RuntimeProbe{Kind: kind, Error: err.Error()})
			return
		}
		caps.Runtimes = append(caps.Runtimes, *rt)
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		rt, err := probeContainerRuntime(ctx, st, dockerHost)
		record(rt, "container", err)
	}()
	go func() {
		defer wg.Done()
		rt, err := probeKubernetes(ctx, st)
		record(rt, protocol.RuntimeKubernetes, err)
	}()
	wg.Wait()
	return caps
}

// probeContainerRuntime pings the Docker-API endpoint and tells Docker from
// Podman (whose compat API reports a "Podman Engine" component).
func probeContainerRuntime(ctx context.Context, st *State, dockerHost string) (*protocol.RuntimeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cli, err := st.Docker.ClientFor(dockerHost)
	if err != nil {
		return nil, err
	}
	v, err := cli.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}

	rt := &protocol.RuntimeInfo{
		Kind:     protocol.RuntimeDocker,
		Version:  v.Version,
		Endpoint: cli.DaemonHost(),
		Features: []string{},
	}
	for _, c := range v.Components {
		if strings.Contains(strings.ToLower(c.Name), "podman") {
			rt.Kind = protocol.RuntimePodman
			rt.Version = c.Version
		}
	}
	if rt.Kind == protocol.RuntimeDocker {
		if info, err := cli.Info(ctx); err == nil && info.Swarm.LocalNodeState == swarm.LocalNodeStateActive {
			rt.Features = append(rt.Features, protocol.FeatureSwarm)
		}
	}
	// Stacks shell out to `docker compose`, so they need the CLI in the agent.
	if _, err := exec.LookPath("docker"); err == nil {
		rt.Features = append(rt.Features, protocol.FeatureCompose)
	}
	return rt, nil
}

// probeKubernetes confirms the cluster actually answers (/version), not merely
// that a kubeconfig or ServiceAccount exists.
func probeKubernetes(ctx context.Context, st *State) (*protocol.RuntimeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	cs, err := st.K8s.Client()
	if err != nil {
		return nil, err
	}
	body, err := cs.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	var v version.Info
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	return &protocol.RuntimeInfo{
		Kind:     protocol.RuntimeKubernetes,
		Version:  v.GitVersion,
		Endpoint: st.K8s.Host(),
		Features: []string{},
	}, nil
}
