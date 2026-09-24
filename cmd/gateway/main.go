// Command gateway is the Pulse gateway (Go port). It owns the database and the
// public HTTP API, serves the built frontend, and forwards Docker/Swarm/k8s
// operations to the agent(s) over the wire protocol.
package main

import (
	"context"
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
)

func main() {
	log.Println("Starting Pulse gateway v0.1.0 (Go version)")

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

	st := gateway.NewState(cfg, db)

	// Consume the pull agent's Docker event stream in the background.
	go gateway.RunWatcher(ctx, st)

	srv := &http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", cfg.HTTPPort), Handler: st.Routes()}
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
