#!/usr/bin/env bash
set -euo pipefail
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
context_dir=$(mktemp -d)
image="sdkms-context-check:$$"
cleanup() { docker image rm "$image" >/dev/null 2>&1 || true; rm -rf "$context_dir"; }
trap cleanup EXIT
cp "$root_dir/.dockerignore" "$context_dir/.dockerignore"
mkdir "$context_dir/deployments"
printf 'NON_SECRET_CONTEXT_TEST\n' > "$context_dir/.env"
printf 'NON_SECRET_CONTEXT_TEST\n' > "$context_dir/deployments/.env"
printf 'EXAMPLE_CONTEXT_TEST\n' > "$context_dir/deployments/.env.example"
printf 'FROM scratch\nCOPY deployments/.env.example /example\n' > "$context_dir/Dockerfile"
docker build -t "$image" "$context_dir" > "$context_dir/build.log" 2>&1
for path in .env deployments/.env; do
  printf 'FROM scratch\nCOPY %s /should-not-exist\n' "$path" > "$context_dir/Dockerfile"
  if docker build -t "$image" "$context_dir" > "$context_dir/build.log" 2>&1; then
    echo "ERROR: local environment file entered Docker context" >&2; exit 1
  fi
  grep -q 'not found' "$context_dir/build.log"
done
echo 'PASS: root/nested .env files excluded from Docker context; example retained'
