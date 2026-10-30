#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

net="sdkms_smoke_${$}_net"
pg_name="sdkms_smoke_${$}_postgres"
created_net=false
created_pg=false
negative_dir=$(mktemp -d)
chmod 755 "$negative_dir"

cleanup() {
  rm -rf "$negative_dir"
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
  if docker exec "$pg_name" pg_isready -h 127.0.0.1 -U postgres -d auth >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker exec "$pg_name" pg_isready -h 127.0.0.1 -U postgres -d auth >/dev/null

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

echo "Checking idempotent migrate up..."
docker run --rm --network "$net" \
  -v "$root_dir/migrations/auth":/migrations:ro migrate/migrate:v4.17.1 \
  -path /migrations -database "postgres://postgres:postgres@$pg_name:5432/auth?sslmode=disable" up

expect_sql_failure() {
  local database="$1" expected="$2" sql="$3" output
  if output=$(docker exec "$pg_name" psql -U postgres -d "$database" -v ON_ERROR_STOP=1 -v VERBOSITY=sqlstate -c "$sql" 2>&1); then
    echo "ERROR: expected SQLSTATE $expected, query unexpectedly succeeded" >&2
    exit 1
  fi
  if [[ "$output" != *"$expected"* ]]; then
    echo "ERROR: expected SQLSTATE $expected, got a different failure" >&2
    exit 1
  fi
}
expect_sql_failure auth 23505 "INSERT INTO users(email,password_hash) VALUES ('duplicate@example.com','test'); INSERT INTO users(email,password_hash) VALUES ('duplicate@example.com','test');"
expect_sql_failure auth 23503 "INSERT INTO sessions(user_id,refresh_token_hash,expires_at) VALUES ('00000000-0000-0000-0000-000000000001',decode('01','hex'),now());"
docker exec "$pg_name" createdb -U postgres migration_negative
expect_sql_failure migration_negative 42P01 "SELECT id FROM public.users;"
cp "$root_dir"/migrations/auth/*.up.sql "$negative_dir/"
printf 'INVALID MIGRATION SYNTAX;\n' > "$negative_dir/003_invalid.up.sql"
chmod 644 "$negative_dir"/*.sql
negative_migrate=(docker run --rm --network "$net" -v "$negative_dir":/migrations:ro migrate/migrate:v4.17.1 -path /migrations -database "postgres://postgres:postgres@$pg_name:5432/migration_negative?sslmode=disable" up)
if "${negative_migrate[@]}" > "$negative_dir/failure.log" 2>&1; then
  echo "ERROR: invalid migration was accepted" >&2; exit 1
fi
# Prove this was a SQL migration failure and not a connectivity/tool failure.
grep -q 'syntax error' "$negative_dir/failure.log"
dirty=$(docker exec "$pg_name" psql -U postgres -d migration_negative -Atc 'SELECT version::text || dirty::text FROM schema_migrations;')
[[ "$dirty" == "3true" ]]
printf 'SELECT 1;\n' > "$negative_dir/003_invalid.up.sql"
if "${negative_migrate[@]}" > "$negative_dir/retry.log" 2>&1; then
  echo "ERROR: dirty database advanced without operator repair" >&2; exit 1
fi
grep -qi 'dirty database' "$negative_dir/retry.log"
echo "OK: migrations, idempotence, constraints, missing schema, invalid SQL, and dirty-state rejection passed"
