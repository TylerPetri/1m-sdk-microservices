#!/usr/bin/env bash
set -euo pipefail

BASE_URL="http://localhost:8080"
EMAIL="tyler@example.com"
PASSWORD="supersecurepassword"

request() {
  local method="$1"; shift
  local url="$1"; shift
  local data="${1:-}"

  echo
  echo ">>> $method $url"
  if [ -n "$data" ]; then
    echo ">>> payload: $data"
  fi

  if [ -n "$data" ]; then
    curl -sS -i --show-error \
      -X "$method" "$url" \
      -H "Content-Type: application/json" \
      -d "$data"
  else
    curl -sS -i --show-error -X "$method" "$url"
  fi
  echo
}

payload_login=$(jq -cn --arg email "$EMAIL" --arg pass "$PASSWORD" \
  '{email:$email,password:$pass}')

request POST "$BASE_URL/v1/auth/login" "$payload_login"