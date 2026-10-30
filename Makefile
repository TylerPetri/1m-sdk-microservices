SHELL := /bin/bash

.PHONY: help tidy proto sqlc generate fmt fmt-check test test-integration verify lint-migrations migrate-auth-smoke up down logs up-prod down-prod logs-prod

help:
	@echo "Targets:"
	@echo "  tidy             - go mod tidy"
	@echo "  proto            - buf generate (protobuf/go/grpc/gateway)"
	@echo "  sqlc             - sqlc generate"
	@echo "  generate         - proto + sqlc"
	@echo "  fmt              - gofmt -w ./..."
	@echo "  fmt-check        - fail if gofmt would change files"
	@echo "  test             - go test ./... -race"
	@echo "  test-integration - go test -tags=integration ./... (requires Docker)"
	@echo "  verify           - CI gate: fmt-check + generate + git diff --exit-code"
	@echo "  lint-migrations  - fail if any down migrations exist (policy)"
	@echo "  migrate-auth-smoke - CI gate: run auth migrations on fresh DB and smoke query"
	@echo ""
	@echo "Docker (dev):"
	@echo "  up               - docker compose up (postgres only)"
	@echo "  down             - docker compose down -v"
	@echo "  logs             - docker compose logs -f"
	@echo ""
	@echo "Docker (prod profile):"
	@echo "  up-prod          - docker compose --profile prod up (services + observability)"
	@echo "  down-prod        - docker compose --profile prod down -v"
	@echo "  logs-prod        - docker compose --profile prod logs -f"

tidy:
	go mod tidy

proto:
	buf generate

sqlc:
	sqlc generate

generate: proto sqlc

fmt:
	gofmt -w ./...

fmt-check:
	@test -z "$$(gofmt -l ./...)" || (echo "gofmt changes required" && gofmt -l ./... && exit 1)

test:
	go test ./... -race

test-integration:
	go test -tags=integration ./...

verify: fmt-check generate
	git diff --exit-code

lint-migrations:
	./scripts/ci/lint-migrations.sh

migrate-auth-smoke:
	./scripts/ci/migrate-auth-smoke.sh

up:
	cd deployments && docker compose up -d

down:
	cd deployments && docker compose down -v

logs:
	cd deployments && docker compose logs -f --tail=200

up-prod:
	cd deployments && docker compose --profile prod up -d --build

down-prod:
	cd deployments && docker compose --profile prod down -v

logs-prod:
	cd deployments && docker compose --profile prod logs -f --tail=200


# --- buf quality gates
proto-lint:
	buf lint

proto-breaking:
	# Compare against origin/main (CI should fetch full history).
	buf breaking --against '.git#branch=origin/main'

proto-check: proto-lint proto-breaking
