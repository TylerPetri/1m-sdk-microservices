#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

net="sdkms_smoke_${$}_net"
pg_name="sdkms_smoke_${$}_postgres"
created_net=false
created_pg=false

cleanup() {
  if "$created_pg"; then docker rm -f "$pg_name" >/dev/null 2>&1 || true; fi
  if "$created_net"; then docker network rm "$net" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

docker network create "$net" >/dev/null
created_net=true

echo "Starting fresh Postgres..."
docker run -d --name "$pg_name" \
  --network "$net" \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -v "$root_dir/deployments/initdb":/docker-entrypoint-initdb.d:ro \
  postgres:16 >/dev/null
created_pg=true

echo "Waiting for Postgres to become ready..."
for i in {1..60}; do
  if docker exec "$pg_name" pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker exec "$pg_name" pg_isready -U postgres >/dev/null

echo "Running auth migrations on fresh DB..."
docker run --rm --network "$net" \
  -v "$root_dir/migrations/auth":/migrations:ro \
  migrate/migrate:v4.17.1 \
  -path /migrations \
  -database "postgres://postgres:postgres@$pg_name:5432/auth?sslmode=disable" \
  up

echo "Smoke query (schema exists + basic query works)..."
docker run --rm --network "$net" postgres:16 \
  psql "postgres://postgres:postgres@$pg_name:5432/auth?sslmode=disable" \
  -v ON_ERROR_STOP=1 \
  -c "SELECT 1;" \
  -c "SELECT id, email, password_hash FROM public.users LIMIT 0;" \
  -c "SELECT * FROM public.sessions LIMIT 0;"

echo "OK: migrations applied and smoke query passed"
