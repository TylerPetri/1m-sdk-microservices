# Production profile (local)

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
