#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
# Capture the exit status separately: command substitution inside test masks errors.
files=$(git ls-files -z '*.go' | xargs -0 -r gofmt -l)
if [[ -n "$files" ]]; then
  printf 'gofmt changes required:\n%s\n' "$files" >&2
  exit 1
fi
