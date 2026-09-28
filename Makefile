# Shortcuts for the repetitive commands. Everything runs in containers; run "make" to list targets.

COMPOSE := docker compose -f docker/compose.yaml
GO      := $(COMPOSE) run --rm api go

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
.PHONY: help run down test vet fmt check sqlc image plan deploy status logs clean-tree

help: ## List the targets
	@awk -F ':.*## ' '/^## / {print "\n" substr($$0, 4)} /^[a-z-]+:.*## / {printf "  %-8s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Development

run: ## Run the API with go run on 127.0.0.1:8080 (stop the Nomad job first: same port)
	$(COMPOSE) up

down: ## Stop the development containers
	$(COMPOSE) down

test: ## Run the tests
	$(GO) test ./...

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format the code with gofmt
	$(COMPOSE) run --rm api gofmt -l -w .

check: vet test ## vet + tests + fail if any file is not gofmt-formatted
	@unformatted=$$($(COMPOSE) run --rm api gofmt -l . 2>/dev/null); \
	if [ -n "$$unformatted" ]; then echo "not formatted (run make fmt):"; echo "$$unformatted"; exit 1; fi

sqlc: ## Regenerate Go code from the SQL queries
	$(COMPOSE) run --rm sqlc generate

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
