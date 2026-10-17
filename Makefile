# Common development tasks. Run "make help" for the list.
#
# make dev starts PostgreSQL, the API and the worker in Docker, then runs the
# web app with hot reload on http://127.0.0.1:5173 (Ctrl+C stops the web app;
# "make down" stops the containers).

COMPOSE ?= docker compose
WEB_DEV_PORT ?= 5173

.DEFAULT_GOAL := help
.PHONY: help env dev backend web-deps up down logs ps test test-api test-web lint smoke

help: ## List the available tasks
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

env: ## Create .env with a random database password if it is missing
	@scripts/init-env.sh

dev: backend web-deps ## Backend in Docker, web app with hot reload on :5173
	@port=$$(. ./.env; echo "$${API_HOST_PORT:-8080}"); \
	echo "RoleScout: http://127.0.0.1:$(WEB_DEV_PORT)  (API on :$$port)"; \
	cd web && API_PROXY_TARGET="http://127.0.0.1:$$port" npm run dev -- --host 127.0.0.1 --port $(WEB_DEV_PORT) --strictPort

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

test-api:
	cd api && go test -race ./...

test-web: web-deps
	cd web && npm test

lint: web-deps ## Format and lint checks
	@cd api && test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt: files above need formatting"; exit 1; }
	cd api && go vet ./...
	cd web && npm run lint

smoke: ## End-to-end check against the stack started with "make up"
	scripts/smoke.sh
