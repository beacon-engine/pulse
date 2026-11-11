# Pulse dev tasks. Requires Go 1.26+. `make tools` installs air + dlv.
GOBIN := $(shell go env GOPATH)/bin

.PHONY: help tools build test fmt vet check \
        watch-gateway watch-agent debug-gateway debug-agent \
        frontend clean security security-iac security-deps security-images

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
	PULSE_MODE=listen PULSE_PORT=9000 $(GOBIN)/air -c .air.agent.toml

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

# ── Security scans (same pinned scanners + config as .github/workflows/security.yml)
# Scanners run from their official images, pinned by digest: nothing to install.
CHECKOV_IMAGE := bridgecrew/checkov:3.4.3@sha256:bfe9d70bb7f044aba49bd99675813445f106dee6072da583f51008cb667f3a63
TRIVY_IMAGE   := aquasec/trivy:0.76.2@sha256:8f6bb947dbc528902e11f2f5be52f7b519f6a39d5d148f41166f2040499768fb
TRIVY := docker run --rm -v pulse-trivy-cache:/root/.cache/trivy -v "$(CURDIR)":/src:ro -w /src \
	-v /var/run/docker.sock:/var/run/docker.sock $(TRIVY_IMAGE) --config trivy.yaml

security: security-iac security-deps security-images ## Run all security scans (Checkov + Trivy)

security-iac: ## Checkov: Dockerfile, k8s manifests, workflows, secrets
	docker run --rm -v "$(CURDIR)":/src:ro -w /src $(CHECKOV_IMAGE) --config-file .checkov.yaml

security-deps: ## Trivy: Go/npm dependencies and committed secrets
	$(TRIVY) fs .

security-images: ## Trivy: build the agent + gateway images and scan them
	docker build -q --target agent -t pulse-agent:scan . >/dev/null
	docker build -q --target gateway -t pulse-gateway:scan . >/dev/null
	$(TRIVY) image pulse-agent:scan
	$(TRIVY) image pulse-gateway:scan

clean: ## Remove build artifacts
	rm -rf ./bin ./tmp
