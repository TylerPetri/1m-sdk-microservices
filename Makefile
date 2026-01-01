SHELL := /bin/bash

.PHONY: help tidy proto sqlc generate fmt fmt-check test up down logs verify lint-migrations migrate-auth-smoke

help:
	@echo "Targets:"
	@echo "  tidy        - go mod tidy"
	@echo "  proto       - buf generate (protobuf/go/grpc/gateway)"
	@echo "  sqlc        - sqlc generate"
	@echo "  generate    - proto + sqlc"
	@echo "  fmt         - gofmt -w ./..."
	@echo "  fmt-check   - fail if gofmt would change files"
	@echo "  test        - go test ./... -race"
	@echo "  verify      - CI gate: fmt-check + generate + git diff --exit-code"
	@echo "  lint-migrations     - fail if any down migrations exist (policy)"
	@echo "  migrate-auth-smoke  - run auth migrations on a fresh DB and smoke query"
	@echo "  up          - docker compose up (incl. Prometheus + Grafana)"
	@echo "  down        - docker compose down -v"
	@echo "  logs        - follow compose logs"

tidy:
	go mod tidy

proto:
	buf generate

sqlc:
	sqlc generate

generate: proto sqlc

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed. Run: make fmt" && gofmt -l . && exit 1)

test:
	go test ./... -race

verify: fmt-check generate
	git diff --exit-code

lint-migrations:
	./scripts/ci/lint-migrations.sh

migrate-auth-smoke:
	./scripts/ci/migrate-auth-smoke.sh

up:
	cd deployments && docker compose up -d --build

down:
	cd deployments && docker compose down -v

logs:
	cd deployments && docker compose logs -f --tail=200
