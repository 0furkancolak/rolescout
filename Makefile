# Common development tasks. Run "make help" for the list.
#
# make dev starts PostgreSQL, the API and the worker in Docker, then runs the
# web app with hot reload on http://127.0.0.1:5173 (Ctrl+C stops the web app;
# "make down" stops the containers). The API and worker (Go) logs are
# streamed into the same terminal.

COMPOSE ?= docker compose
WEB_DEV_PORT ?= 5173

.DEFAULT_GOAL := help
.PHONY: help doctor env dev backend rebuild web-deps up down logs ps test test-api test-web lint smoke

help: ## List the available tasks
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

doctor: ## Check Docker, Node, ports and .env before starting
	@scripts/doctor.sh

env: ## Create .env with a random database password if it is missing
	@scripts/init-env.sh

dev: preflight backend web-deps ## Backend in Docker, web app with hot reload on :5173, Go logs in this terminal
	@port=$$(. ./.env; echo "$${API_HOST_PORT:-8080}"); \
	echo "RoleScout: http://127.0.0.1:$(WEB_DEV_PORT)  (API on :$$port)"; \
	$(COMPOSE) logs -f --tail=20 api worker & logs=$$!; \
	trap 'kill $$logs 2>/dev/null' EXIT INT TERM; \
	cd web && API_PROXY_TARGET="http://127.0.0.1:$$port" npm run dev -- --host 127.0.0.1 --port $(WEB_DEV_PORT) --strictPort

# preflight stops early with a clear message instead of a Docker stack trace.
.PHONY: preflight
preflight:
	@docker info >/dev/null 2>&1 || { echo "Docker is not running. Start Docker Desktop or OrbStack, then run make dev again."; exit 1; }
	@scripts/doctor.sh --check

rebuild: env ## Rebuild and restart the API and worker after Go changes
	$(COMPOSE) up -d --build --wait api worker

backend: env ## Start PostgreSQL, the API and the worker in Docker
	$(COMPOSE) up -d --build --wait postgres api worker

web-deps: web/node_modules/.package-lock.json

web/node_modules/.package-lock.json: web/package-lock.json
	cd web && npm ci

up: env ## Start the full stack in Docker on http://127.0.0.1:4173
	$(COMPOSE) up -d --build --wait
	@port=$$(. ./.env; echo "$${WEB_PORT:-4173}"); echo "RoleScout: http://127.0.0.1:$$port"

down: ## Stop the containers (data is kept)
	$(COMPOSE) down

logs: ## Follow container logs
	$(COMPOSE) logs -f

ps: ## Show container status
	$(COMPOSE) ps

test: test-api test-web ## Run all unit tests

# Database tests need PostgreSQL. When the dev database is up they run against
# it (in throwaway databases); otherwise they are skipped, as in "go test".
test-api: ## Go tests; uses the dev PostgreSQL when it is running
	@if [ -z "$$TEST_DATABASE_URL" ] && [ -f .env ] && $(COMPOSE) ps --status running --services 2>/dev/null | grep -qx postgres; then \
	  set -a; . ./.env; set +a; \
	  echo "Using the dev PostgreSQL for database tests."; \
	  cd api && TEST_DATABASE_URL="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@127.0.0.1:$${POSTGRES_PORT:-5432}/$$POSTGRES_DB?sslmode=disable" go test -race ./...; \
	else \
	  cd api && go test -race ./...; \
	fi

test-web: web-deps
	cd web && npm test

lint: web-deps ## Format and lint checks
	@cd api && test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files above need formatting"; exit 1; }
	cd api && go vet ./...
	cd web && npm run lint

smoke: ## End-to-end check against the stack started with "make up"
	scripts/smoke.sh
