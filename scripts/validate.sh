#!/usr/bin/env bash
set -euo pipefail
BASE_URL="${BASE_URL:-http://localhost:8080}"
: "${ACCESS_TOKEN:?Set ACCESS_TOKEN from a fresh login response}"
jq -cn --arg token "$ACCESS_TOKEN" '{access_token:$token}' |
  curl --fail-with-body -sS -X POST "$BASE_URL/v1/auth/validate" \
    -H "Content-Type: application/json" --data-binary @-
