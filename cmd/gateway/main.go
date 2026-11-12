// Command gateway is the Pulse gateway (Go port). It owns the database and the
// public HTTP API, serves the built frontend, and forwards Docker/Swarm/k8s
// operations to the agent(s) over the wire protocol.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/beacon-engine/pulse/internal/gateway"
	"github.com/beacon-engine/pulse/internal/gateway/storage"
	"github.com/beacon-engine/pulse/internal/version"
)

func main() {
	// `gateway healthcheck` is the image's HEALTHCHECK: the runtime image has no
	// shell or curl, so the binary probes its own /health.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}

	log.Printf("Starting Pulse gateway %s (Go version)", version.Version)

	cfg := gateway.LoadConfig()
	// Cancel on SIGINT/SIGTERM so the server and background watcher shut down
	// cleanly on `docker stop` / a k8s rolling update.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := storage.Init(ctx, cfg.DatabaseURL, cfg.DBResetOnMismatch)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize database: %v\n", err)
		var mm *storage.MismatchError
		if errors.As(err, &mm) {
			fmt.Fprintf(os.Stderr,
				"\n  This database has migration %d applied, but this binary doesn't include it.\n"+
					"  You're running a stale build or switched branches against a persistent DB.\n"+
					"  Rebuild from current source, or reset a THROWAWAY dev DB:\n"+
					"    • set PULSE_DB_RESET_ON_MISMATCH=1 to auto-recreate on startup, or\n"+
					"    • delete the sqlite file (rm pulse.db*) / drop the Docker volume.\n", mm.Version)
		}
		os.Exit(1)
	}
	log.Printf("Database ready at %s", cfg.DatabaseURL)
	log.Printf("Forwarding Docker operations to agent at %s", cfg.AgentAddr)

	gw := gateway.New(cfg, db)
	// The default agent's Docker events, push-agent liveness and pull-agent probes.
	gw.Start(ctx)

	srv := &http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", cfg.HTTPPort), Handler: gw.Handler()}
	tls := cfg.TLSCertPath != "" && cfg.TLSKeyPath != ""

	go func() {
		var err error
		if tls {
			log.Printf("Pulse gateway listening on https://localhost:%d", cfg.HTTPPort)
			err = srv.ListenAndServeTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
		} else {
			log.Printf("Pulse gateway listening on http://localhost:%d", cfg.HTTPPort)
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
}

// healthcheck GETs this gateway's /health on loopback and returns the process
// exit code: 0 healthy, 1 not. With native TLS the cert is issued for the public
// name, not 127.0.0.1, so verification is skipped for this loopback-only probe.
func healthcheck() int {
	cfg := gateway.LoadConfig()
	scheme := "http"
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		scheme = "https"
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // loopback self-probe
		},
	}
	resp, err := client.Get(fmt.Sprintf("%s://127.0.0.1:%d/health", scheme, cfg.HTTPPort))
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: status", resp.StatusCode)
		return 1
	}
	return 0
}
