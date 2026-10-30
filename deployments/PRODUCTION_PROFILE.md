# Local full-stack demo (historical `prod` profile)

This profile is experimental local infrastructure, not a production deployment.
All published ports bind to 127.0.0.1. Postgres uses development credentials and
Grafana permits anonymous viewing; never expose this stack externally.

This is a Docker Compose profile that runs:

- gatewayd (HTTP + metrics/admin)
- authd (gRPC + metrics/admin)
- hellod (gRPC + metrics/admin)
- Postgres
- Prometheus + recording rules + alert rules
- Grafana (pre-provisioned dashboards)
- Optional: Loki (+ Promtail) for logs
- Optional: OTEL Collector (for OTLP -> Prometheus bridge)

## Quick start

From repo root:

```bash
cp deployments/.env.example deployments/.env
printf 'AUTH_JWT_SECRET=%s\n' "$(openssl rand -hex 32)" > deployments/.env
make up-prod
```

Open:
- Gateway: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000

## Optional profiles

Logs stack:

```bash
cd deployments
docker compose --profile prod --profile logs up -d --build
```

OTEL Collector:

```bash
cd deployments
docker compose --profile prod --profile otel up -d --build
```

## Alerts and recording rules

Prometheus loads:

- `deployments/observability/prometheus/rules/recording.yml` (p95/p99 recording rules)
- `deployments/observability/prometheus/rules/alerts.yml` (basic SLO + saturation alerts)

These are intentionally minimal and safe to evolve.

Auth waits for the one-shot migration service to complete. `make down-prod` removes
local volumes and all their data. Existing database volumes retain their initial
credentials; changing environment variables does not rotate existing users.

## Reproducible smoke verification

From the repository root, `make smoke-compose` builds all three actual Dockerfile
runtime targets and creates a uniquely named disposable stack with random
loopback ports. It exercises auth/Hello, readiness, Prometheus scraping and rule
tests, Grafana provisioning, OTLP metric delivery, and Promtail-to-Loki logs.
It removes only its own containers, network, and volumes on exit. Logs are saved
under a temporary directory (or `SDK_SMOKE_LOG_DIR`). It does not reuse an
existing database or deployment.

Grafana's overview uses the emitted application metric names; HTTP grouping is
by method rather than route. Recording/error-rate rules have synthetic promtool
checks. Alert thresholds remain illustrative and need workload-specific tuning.
Promtail is scoped by the Compose project label, but its Docker socket access is
still suitable only for a trusted local development host.
