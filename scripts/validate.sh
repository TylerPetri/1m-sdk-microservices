#!/usr/bin/env bash
set -euo pipefail

BASE_URL="http://localhost:8080"
EMAIL="tyler@example.com"
PASSWORD="supersecurepassword"
ACCESS_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6InR5bGVyQGV4YW1wbGUuY29tIiwiaXNzIjoic2RrLW1pY3Jvc2VydmljZXMiLCJzdWIiOiJhZmE3M2MxZi1lM2IxLTQ5YWEtYTY4MC1jZmMzZWZhNjBmYTAiLCJleHAiOjE3NjcwNTg3NjksImlhdCI6MTc2NzA1Nzg2OX0.5H5kNdJVX-fmcoM-WJV6oN9VI-8yjZQPm_oKWISdJIs"

curl -sS -X POST "$BASE_URL/v1/auth/validate" \
  -H "Content-Type: application/json" \
  -d "{
    \"access_token\":\"$ACCESS_TOKEN\"
  }"
