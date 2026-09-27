// Package agent is the Go port of the Rust `pulse-core` agent crate: a stateless
// executor that owns Docker, Docker Swarm and Kubernetes access and answers
// protocol.Requests from the gateway.
//
// This file ports agent/src/config.rs. All environment reads live here (the
// consolidation the Rust side settled on) — other files receive config values.
package agent

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

// defaultHeartbeat is how often a dial-in agent calls home so the gateway knows
// it is alive; minHeartbeat floors any override.
const (
	defaultHeartbeat = 30 * time.Second
	minHeartbeat     = 5 * time.Second
)

// defaultDockerHost is the local Docker daemon an auto-registered environment
// represents when PULSE_DOCKER_HOST is unset.
const defaultDockerHost = "unix:///var/run/docker.sock"

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
	// From PULSE_TRANSPORT (legacy BEACON_TRANSPORT).
	ListenTransport string
	// InsecureSkipTLSVerify disables server-certificate verification on the
	// wss:// uplink (PULSE_INSECURE_SKIP_TLS_VERIFY). Trusted networks only.
	InsecureSkipTLSVerify bool

	// ── Auto-registration (ModeDial) ──────────────────────────────────────────

	// AutoRegister makes a dial-in agent self-register as an environment on the
	// gateway (default on). Turn off (PULSE_AUTO_REGISTER=off) for the legacy
	// static path where an admin pre-creates the environment.
	AutoRegister bool
	// InstanceID uniquely identifies this running agent process. Generated fresh
	// at boot (no config): stable for the process lifetime so in-process
	// reconnects dedup, and gone on exit so a replacement is legitimately new.
	InstanceID string
	// EnvName is the human-readable name to register under (PULSE_ENV_NAME);
	// defaults to the hostname.
	EnvName string
	// DockerHost is the local Docker endpoint the registered environment points
	// at (PULSE_DOCKER_HOST); defaults to the standard socket.
	DockerHost string
	// HeartbeatInterval is how often the agent calls home while connected
	// (PULSE_HEARTBEAT_INTERVAL); defaults to 30s, floored at 5s.
	HeartbeatInterval time.Duration
}

// LoadConfig reads the agent configuration from the environment.
func LoadConfig() Config {
	return Config{
		Mode:                  parseMode(envAlias("PULSE_MODE", "BEACON_MODE")),
		Port:                  parsePort(envAlias("PULSE_PORT", "BEACON_PORT"), 9000),
		GatewayURL:            nonEmpty(os.Getenv("PULSE_GATEWAY_WS_URL")),
		GatewayToken:          nonEmpty(envAlias("PULSE_TOKEN", "BEACON_TOKEN")),
		KubeconfigYAML:        loadKubeconfig(),
		HostRoot:              os.Getenv("HOST_ROOT"),
		InsecureSkipTLSVerify: truthy(os.Getenv("PULSE_INSECURE_SKIP_TLS_VERIFY")),
		ListenTransport:       parseListenTransport(envAlias("PULSE_TRANSPORT", "BEACON_TRANSPORT")),
		AutoRegister:          boolOr(os.Getenv("PULSE_AUTO_REGISTER"), true),
		InstanceID:            uuid.NewString(),
		EnvName:               envName(),
		DockerHost:            orDefault(os.Getenv("PULSE_DOCKER_HOST"), defaultDockerHost),
		HeartbeatInterval:     parseInterval(os.Getenv("PULSE_HEARTBEAT_INTERVAL"), defaultHeartbeat),
	}
}

// envAlias reads pulseKey, falling back to the deprecated beaconKey (logging a
// one-time notice when only the legacy name is set). Returns "" when neither set.
func envAlias(pulseKey, beaconKey string) string {
	if v := os.Getenv(pulseKey); strings.TrimSpace(v) != "" {
		return v
	}
	if v := os.Getenv(beaconKey); strings.TrimSpace(v) != "" {
		log.Printf("warning: %s is deprecated; rename it to %s", beaconKey, pulseKey)
		return v
	}
	return ""
}

// envName is the display name to register under: PULSE_ENV_NAME, else the
// hostname, else a generic fallback.
func envName() string {
	if v := strings.TrimSpace(os.Getenv("PULSE_ENV_NAME")); v != "" {
		return v
	}
	if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}
	return "pulse-agent"
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

// parseInterval accepts a Go duration ("45s", "2m") or a bare integer count of
// seconds ("30"); anything unparseable falls back to def. The result is floored
// at minHeartbeat.
func parseInterval(v string, def time.Duration) time.Duration {
	v = strings.TrimSpace(v)
	d := def
	if v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			d = parsed
		} else if n, err := strconv.Atoi(v); err == nil {
			d = time.Duration(n) * time.Second
		}
	}
	if d < minHeartbeat {
		d = minHeartbeat
	}
	return d
}

// loadKubeconfig sources explicit kubeconfig YAML from PULSE_KUBECONFIG_PATH (a
// file to read; legacy BEACON_KUBECONFIG_PATH) or, failing that, the inline
// PULSE_KUBECONFIG (legacy BEACON_KUBECONFIG). Empty ("") means auto-discover.
func loadKubeconfig() string {
	if path := envAlias("PULSE_KUBECONFIG_PATH", "BEACON_KUBECONFIG_PATH"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			return string(data)
		}
	}
	return envAlias("PULSE_KUBECONFIG", "BEACON_KUBECONFIG")
}

func nonEmpty(v string) string {
	if strings.TrimSpace(v) == "" {
		return ""
	}
	return v
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// boolOr parses an env flag, returning def when unset. Set values use truthy().
func boolOr(v string, def bool) bool {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return truthy(v)
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
