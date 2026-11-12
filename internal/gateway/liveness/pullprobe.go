package liveness

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/beacon-engine/pulse/internal/protocol"
)

// A pull agent never calls home, so without probing, an environment added by
// address would stay "online" forever. The prober pings every pull agent on an
// interval and records the result; the agent sweeper then alerts on the ones
// that stay down past the grace window, as it does for push agents.

// pullProbeInterval is how often each pull agent is pinged.
const pullProbeInterval = 30 * time.Second

// pullProbeTimeout bounds one ping, so a dead agent can't stall a round.
const pullProbeTimeout = 10 * time.Second

// RunPullProber probes every pull agent once at startup and then on an interval.
func (m *Monitor) RunPullProber(ctx context.Context) {
	t := time.NewTicker(pullProbeInterval)
	defer t.Stop()
	for {
		m.probePullAgents(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (m *Monitor) probePullAgents(ctx context.Context) {
	envs, err := m.DB.ListPullEnvironments(ctx)
	if err != nil {
		log.Printf("pull prober: list: %v", err)
		return
	}
	var wg sync.WaitGroup
	for _, env := range envs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.probePullAgent(ctx, env)
		}()
	}
	wg.Wait()
}

// probePullAgent pings one environment's agent and records whether it answered.
// Status changes are logged once, not on every round.
func (m *Monitor) probePullAgent(ctx context.Context, env protocol.Environment) {
	pctx, cancel := context.WithTimeout(ctx, pullProbeTimeout)
	defer cancel()
	resp := m.Router.CallOn(pctx, nil, env.AgentAddr, protocol.Request{Kind: protocol.ReqPing})
	online := resp.Kind == protocol.RespKindPong
	if ctx.Err() != nil {
		return // shutting down; the failed ping says nothing about the agent
	}
	if err := m.DB.SetPullStatus(ctx, env.ID, online); err != nil {
		log.Printf("pull prober: record %s: %v", env.Name, err)
		return
	}
	switch {
	case online && env.Status == "offline":
		log.Printf("Pull agent for %q is reachable again", env.Name)
	case !online && env.Status != "offline":
		_, msg, _ := resp.AsError()
		log.Printf("Pull agent for %q is unreachable: %s", env.Name, msg)
	}
}
