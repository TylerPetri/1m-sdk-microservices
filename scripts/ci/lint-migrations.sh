#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Policy: Down migrations are banned (expand/contract only).
if find "$root_dir/migrations" -type f -name "*.down.sql" | grep -q .; then
  echo "ERROR: down migrations are banned. Found:" >&2
  find "$root_dir/migrations" -type f -name "*.down.sql" -print >&2
  exit 1
fi

echo "OK: no down migrations found"
