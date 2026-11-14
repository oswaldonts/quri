# Shortcuts for the repetitive commands. Everything runs in containers; run "make" to list targets.

COMPOSE := docker compose -f docker/compose.yaml
GO      := $(COMPOSE) run --rm api go
# --no-deps: npm commands don't need the API running (web-types is the exception).
NPM     := $(COMPOSE) run --rm --no-deps web npm

# Deployment: the image tag is the commit, so a tag always maps to the exact code it was built from.
ENV   ?= dev
TAG   ?= $(shell git rev-parse --short HEAD)
IMAGE ?= go-income-mgr:$(TAG)
# AUTH_PASSWORD_HASH and AUTH_SESSION_SECRET must be exported in the shell (never in a
# vars.hcl file); see deploy/nomad/go-income-mgr.nomad.hcl for how to generate each.
JOB   := -var-file=$(ENV).vars.hcl -var image=$(IMAGE) \
         -var auth_password_hash=$(AUTH_PASSWORD_HASH) -var auth_session_secret=$(AUTH_SESSION_SECRET) \
         go-income-mgr.nomad.hcl

export NOMAD_NAMESPACE := $(ENV)

.DEFAULT_GOAL := help
.PHONY: help run down test vet fmt check sqlc web-dev web-build web-check web-types desktop image plan deploy status logs clean-tree

help: ## List the targets
	@awk -F ':.*## ' '/^## / {print "\n" substr($$0, 4)} /^[a-z-]+:.*## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Development

run: ## Run the API with go run on 127.0.0.1:8081 (8080 is the Nomad job)
	$(COMPOSE) up

down: ## Stop the development containers
	$(COMPOSE) down

test: ## Run the tests
	$(GO) test ./...

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format the code with gofmt
	$(COMPOSE) run --rm api gofmt -l -w .

check: vet test web-check ## vet + tests + frontend type-check + fail if any file is not gofmt-formatted
	@unformatted=$$($(COMPOSE) run --rm api gofmt -l . 2>/dev/null); \
	if [ -n "$$unformatted" ]; then echo "not formatted (run make fmt):"; echo "$$unformatted"; exit 1; fi

sqlc: ## Regenerate Go code from the SQL queries
	$(COMPOSE) run --rm sqlc generate

## Frontend (web/, Svelte)

web-dev: ## Run the frontend with hot reload on 127.0.0.1:5173 (starts the API too)
	$(COMPOSE) up web

web-build: ## Build the frontend into web/dist, which the Go binary embeds
	$(NPM) run build

web-check: ## Type-check the frontend
	$(NPM) run check

web-types: ## Regenerate the frontend's API types from the API's OpenAPI spec (starts the API)
	$(COMPOSE) run --rm web npm run types

## Desktop app (cmd/desktop, Wails; own Go module)

DESKTOP_IMAGE := quri-desktop-build
# Runs a go command inside cmd/desktop in the image that has the GTK/WebKit headers Wails needs.
DESKTOP_GO := docker run --rm --user $$(id -u):$$(id -g) -e HOME=/tmp -e GOCACHE=/go-cache/build \
              -e GOMODCACHE=/go-cache/mod -v $(CURDIR):/app -v $(HOME)/.cache/go-container:/go-cache \
              -w /app/cmd/desktop $(DESKTOP_IMAGE) go

desktop: web-build ## Build the Linux desktop app into bin/quri (runs on a host with libwebkit2gtk-4.1)
	docker build -q -f docker/desktop.Dockerfile -t $(DESKTOP_IMAGE) docker
	$(DESKTOP_GO) build -tags "desktop,production,webkit2_41" -trimpath -ldflags="-s -w" -o /app/bin/quri .

## Deployment (ENV=dev by default)

image: clean-tree ## Build the image tagged with the current commit
	docker build -f docker/Dockerfile -t $(IMAGE) .

plan: image ## Show what a deploy would change
	cd deploy/nomad && nomad job plan $(JOB); test $$? -le 1 # exit 1 only means "changes found"

deploy: check image ## Check, build and deploy to Nomad
	cd deploy/nomad && nomad job run $(JOB)

status: ## Show the job status
	nomad job status go-income-mgr

logs: ## Follow the logs of the running allocation
	nomad alloc logs -f -job go-income-mgr

clean-tree:
	@test -z "$$(git status --porcelain)" || { echo "uncommitted changes: commit first, the image tag is the commit"; exit 1; }
