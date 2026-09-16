// Package gateway is the Go port of the Rust `gateway` crate: it owns the
// database, the public HTTP API, and the transport to the agent(s). This file
// ports gateway/src/config.rs — all environment reads live here.
package gateway

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	SlackWebhookURL string
	// HTTPPort is the port the gateway serves the frontend + API on.
	HTTPPort uint16
	// AgentAddr is the TCP address of the default (pull) agent, host:port.
	AgentAddr   string
	DatabaseURL string
	StaticDir   string
	// AgentTokens maps token -> agent_id for authenticating push agents on
	// /agent/ws. Empty disables push (only the pull agent is used).
	AgentTokens map[string]string
	// TLSCertPath/TLSKeyPath: when both are set the gateway serves HTTPS/WSS
	// directly (native rustls-equivalent); otherwise plain HTTP behind an
	// upstream terminator.
	TLSCertPath string
	TLSKeyPath  string
	// DBResetOnMismatch (dev-only): recreate the database on a migration
	// mismatch instead of aborting.
	DBResetOnMismatch bool
}

func LoadConfig() Config {
	return Config{
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		HTTPPort:          parsePort(os.Getenv("HTTP_PORT"), 8080),
		AgentAddr:         envOr("AGENT_ADDR", "127.0.0.1:9000"),
		DatabaseURL:       envOr("DATABASE_URL", "sqlite://pulse.db"),
		StaticDir:         envOr("STATIC_DIR", "./frontend/dist"),
		AgentTokens:       parseAgentTokens(os.Getenv("PULSE_AGENT_TOKENS")),
		TLSCertPath:       os.Getenv("PULSE_TLS_CERT"),
		TLSKeyPath:        os.Getenv("PULSE_TLS_KEY"),
		DBResetOnMismatch: envFlag("PULSE_DB_RESET_ON_MISMATCH"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parsePort(v string, def uint16) uint16 {
	if n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 16); err == nil {
		return uint16(n)
	}
	return def
}

func envFlag(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// parseAgentTokens parses a comma-separated list of agent_id:token pairs into a
// token -> agent_id lookup. Blank/malformed entries are skipped.
// Example: "prod:s3cret,staging:h0nk".
func parseAgentTokens(raw string) map[string]string {
	out := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		id, token, ok := strings.Cut(entry, ":")
		if !ok {
			continue
		}
		id, token = strings.TrimSpace(id), strings.TrimSpace(token)
		if id != "" && token != "" {
			out[token] = id
		}
	}
	return out
}
