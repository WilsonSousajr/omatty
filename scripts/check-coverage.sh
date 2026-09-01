#!/usr/bin/env bash
# Enforces the coverage gate from AGENTS.md. Usage: check-coverage.sh [threshold]
set -euo pipefail
threshold="${1:-90}"
go test ./... -race -coverpkg=./internal/... -coverprofile=cover.out >/dev/null
total="$(go tool cover -func=cover.out | awk '/^total:/ {gsub(/%/,"",$3); print $3}')"
awk -v t="$total" -v th="$threshold" 'BEGIN {
  if (t + 0 < th + 0) { printf "coverage %.1f%% is below the %s%% gate\n", t, th; exit 1 }
  printf "coverage %.1f%% meets the %s%% gate\n", t, th
}'
