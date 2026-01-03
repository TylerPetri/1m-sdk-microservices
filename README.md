# sdk-microservices (Go)

A production-style Go microservices starter: gRPC services behind an HTTP gateway (grpc-gateway), Postgres-backed auth, and a shared “platform” layer for boot/health/metrics/logging/OTel.

## Why this repo exists

This is a **clean, team-ready** baseline that demonstrates how I build services that scale operationally:
- **Consistent service boot pattern** (logging, config, shutdown, health/readiness, admin server)
- **API-first boundaries** with **Protobuf + Buf** and generated gRPC + HTTP (grpc-gateway)
- **Typed data access** with **pgxpool + sqlc**
- **Operational maturity**: `/livez`, `/readyz`, `/metrics`, tracing hooks, and Docker Compose for local prod-like runs
- **Deterministic generation + CI gates** (no “it works on my machine” drift)

## Architecture (at a glance)

- **gatewayd** (HTTP) → routes `/v1/*` → calls gRPC services
- **authd** (gRPC) → register/login/validate backed by Postgres
- **hellod** (gRPC) → simple example service
- **admin ports** per service expose health + metrics

## Quick run (local)

Prereqs: Go + Docker

```bash
make generate   # buf + sqlc
make test       # unit tests (race)
make up-prod    # starts postgres + services + observability stack (profile-based)
```

> This repository represents an evolving production-grade system and is not intended as a reusable template.

---

## License

© 2026 Tyler Petri. All rights reserved.

This repository and its contents are proprietary and confidential.  
No permission is granted to use, copy, modify, merge, publish, distribute, sublicense, or sell any part of this software without explicit written permission from the author.

This project is shared publicly for evaluation, discussion, and demonstration purposes only.
