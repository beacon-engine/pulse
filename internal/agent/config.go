// Package agent is the Go port of the Rust `pulse-core` agent crate: a stateless
// executor that owns Docker, Docker Swarm and Kubernetes access and answers
// protocol.Requests from the gateway.
//
// This file ports agent/src/config.rs. All environment reads live here (the
// consolidation the Rust side settled on) — other files receive config values.
package agent

import (
	"os"
	"strconv"
	"strings"
)

// Mode is how the agent reaches the gateway.
type Mode int

const (
	// ModeListen (pull): bind a TCP socket and wait for the gateway to connect.
	ModeListen Mode = iota
	// ModeDial (push): dial out to the gateway over WSS and serve requests over
	// that session. For firewalled agents that can only make outbound connections.
	ModeDial
)

// Config is the agent's runtime configuration, sourced entirely from the
// environment.
type Config struct {
	// Mode is how the agent reaches the gateway (pull ModeListen or push ModeDial).
	Mode Mode
	// Port is the TCP port the agent listens on in ModeListen.
	Port uint16
	// GatewayURL is the gateway WebSocket URL to dial in ModeDial, e.g.
	// wss://gateway.example.com/agent/ws. Empty when unset.
	GatewayURL string
	// GatewayToken is the bearer token presented on the WS upgrade in ModeDial.
	GatewayToken string
	// KubeconfigYAML is an explicit kubeconfig; empty means auto-discover
	// (in-pod ServiceAccount or the agent host's ~/.kube/config).
	KubeconfigYAML string
	// HostRoot is the host filesystem root as seen from the container (HOST_ROOT,
	// e.g. /host for `-v /:/host:ro`). Empty means bare-metal / no bind-mount.
	HostRoot string
	// ListenTransport selects how the agent serves in Listen (pull) mode:
	// "tcp" (framed TCP, default) or "ws" (WebSocket over HTTP — rides 80/443,
	// so it traverses the firewalls/proxies that block arbitrary TCP ports).
	// From BEACON_TRANSPORT.
	ListenTransport string
	// InsecureSkipTLSVerify disables server-certificate verification on the
	// wss:// uplink (PULSE_INSECURE_SKIP_TLS_VERIFY). Trusted networks only.
	InsecureSkipTLSVerify bool
}

// LoadConfig reads the agent configuration from the environment.
func LoadConfig() Config {
	return Config{
		Mode:                  parseMode(os.Getenv("BEACON_MODE")),
		Port:                  parsePort(os.Getenv("BEACON_PORT"), 9000),
		GatewayURL:            nonEmpty(os.Getenv("PULSE_GATEWAY_WS_URL")),
		GatewayToken:          nonEmpty(os.Getenv("BEACON_TOKEN")),
		KubeconfigYAML:        loadKubeconfig(),
		HostRoot:              os.Getenv("HOST_ROOT"),
		InsecureSkipTLSVerify: truthy(os.Getenv("PULSE_INSECURE_SKIP_TLS_VERIFY")),
		ListenTransport:       parseListenTransport(os.Getenv("BEACON_TRANSPORT")),
	}
}

func parseListenTransport(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ws", "wss", "websocket", "http":
		return "ws"
	default:
		return "tcp"
	}
}

func parseMode(v string) Mode {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "dial", "push":
		return ModeDial
	default:
		return ModeListen
	}
}

func parsePort(v string, def uint16) uint16 {
	if n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 16); err == nil {
		return uint16(n)
	}
	return def
}

// loadKubeconfig sources explicit kubeconfig YAML from BEACON_KUBECONFIG_PATH (a
// file to read) or, failing that, the inline BEACON_KUBECONFIG. Empty ("") means
// auto-discover.
func loadKubeconfig() string {
	if path := os.Getenv("BEACON_KUBECONFIG_PATH"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return os.Getenv("BEACON_KUBECONFIG")
}

func nonEmpty(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return v
}

// truthy reports whether an env flag is set: 1/true/yes/on (case-insensitive).
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
