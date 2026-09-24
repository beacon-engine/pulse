# Pulse dev tasks. Requires Go 1.26+. `make tools` installs air + dlv.
GOBIN := $(shell go env GOPATH)/bin

.PHONY: help tools build test fmt vet check \
        watch-gateway watch-agent debug-gateway debug-agent \
        frontend clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	  awk 'BEGIN {FS=":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

tools: ## Install dev tools (air for hot-reload, dlv for debugging)
	go install github.com/air-verse/air@latest
	go install github.com/go-delve/delve/cmd/dlv@latest
	@echo "Installed to $(GOBIN) — make sure it's on your PATH."

build: ## Build both binaries into ./bin
	go build -o ./bin/agent ./cmd/agent
	go build -o ./bin/gateway ./cmd/gateway

test: ## Run all Go tests
	go test ./... -count=1

fmt: ## Format all Go code
	gofmt -w cmd internal

vet: ## Run go vet
	go vet ./...

check: fmt vet test ## Format, vet, and test (what the pre-push hook runs)

# ── Local dev (binaries run on your machine) ─────────────────────────────────
watch-gateway: ## Hot-reload the gateway locally (rebuild + restart on save)
	HTTP_PORT=9080 AGENT_ADDR=127.0.0.1:9000 DATABASE_URL=sqlite://pulse-dev.db \
	  STATIC_DIR=./frontend/dist PULSE_DB_RESET_ON_MISMATCH=1 $(GOBIN)/air -c .air.gateway.toml

watch-agent: ## Hot-reload the agent locally (rebuild + restart on save)
	BEACON_MODE=listen BEACON_PORT=9000 $(GOBIN)/air -c .air.agent.toml

debug-gateway: ## Debug the gateway locally under dlv-dap (attach Zed on :2345)
	$(GOBIN)/dlv dap --listen=:2345 --log

debug-agent: ## Debug the agent locally under dlv-dap (attach Zed on :2346)
	$(GOBIN)/dlv dap --listen=:2346 --log

frontend: ## Run the Vite dev server (proxies /api and /ws to the gateway)
	cd frontend && npm install && npm run dev

# ── Containerized dev (binaries run in Docker, hot-reload + remote debug) ─────
DEV := docker compose -f docker-compose.dev.yml

dev: ## Start the dev stack in containers (air hot-reload; edit+save → rebuild)
	$(DEV) up --build

dev-down: ## Stop the dev stack
	$(DEV) down

dev-debug-gateway: ## Debug the containerized gateway: run dlv-dap in it, attach Zed on :2345
	$(DEV) exec gateway sh -c "pkill air 2>/dev/null; pkill -f tmp/gateway 2>/dev/null; dlv dap --listen=:2345 --log"

dev-debug-agent: ## Debug the containerized agent: run dlv-dap in it, attach Zed on :2346
	$(DEV) exec agent sh -c "pkill air 2>/dev/null; pkill -f tmp/agent 2>/dev/null; dlv dap --listen=:2346 --log"

clean: ## Remove build artifacts
	rm -rf ./bin ./tmp
