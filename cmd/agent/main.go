// Command agent is the Pulse agent (Go port). A stateless executor that owns
// Docker, Docker Swarm and Kubernetes access and answers protocol.Requests from
// the gateway, over a TCP listener (pull) or an outbound WSS uplink (push).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/beacon-engine/pulse/internal/agent"
)

func main() {
	log.Println("Starting Pulse agent v0.1.0 (Go)")

	cfg := agent.LoadConfig()
	st := agent.NewState(cfg)
	// Cancel on SIGINT/SIGTERM so the listener / uplink shut down cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Best-effort initial Kubernetes connect; if it fails we retry lazily per request.
	if _, err := st.K8s.Client(); err != nil {
		log.Printf("Kubernetes not connected yet (%v); will retry on demand", err)
	} else {
		log.Println("Kubernetes connected")
	}

	switch cfg.Mode {
	case agent.ModeDial:
		if cfg.GatewayURL == "" {
			log.Fatal("PULSE_MODE=dial requires PULSE_GATEWAY_WS_URL")
		}
		if cfg.GatewayToken == "" {
			log.Println("warning: PULSE_TOKEN is unset — the gateway will reject the session unless it allows unauthenticated agents")
		}
		if cfg.AutoRegister {
			log.Printf("Pulse agent dialing gateway at %s (push mode; auto-register as %q, instance %s)",
				cfg.GatewayURL, cfg.EnvName, cfg.InstanceID)
		} else {
			log.Printf("Pulse agent dialing gateway at %s (push mode; auto-register disabled)", cfg.GatewayURL)
		}
		agent.RunUplink(ctx, st, cfg)

	case agent.ModeListen:
		serve := agent.ServeTCP
		if cfg.ListenTransport == "ws" {
			serve = agent.ServeWS
		}
		if err := serve(ctx, st, cfg.Port); err != nil {
			log.Fatalf("agent server failed: %v", err)
		}
	}
}
