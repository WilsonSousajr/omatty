#!/usr/bin/env bash
# Enforces the coverage gate from AGENTS.md. Usage: check-coverage.sh [threshold]
#
# Measures each internal package against its own tests. Deliberately NOT
# -coverpkg=./internal/...: with several test binaries each instrumenting every
# package, the merged profile zeroes out counts from the other binaries and
# reports a number far below the truth. cmd/ is excluded by listing only
# ./internal/..., which is what the gate covers.
set -euo pipefail
threshold="${1:-90}"

go test ./internal/... -race -coverprofile=cover.out
total="$(go tool cover -func=cover.out | awk '/^total:/ {gsub(/%/,"",$3); print $3}')"

awk -v t="$total" -v th="$threshold" 'BEGIN {
  if (t + 0 < th + 0) { printf "coverage %.1f%% is below the %s%% gate\n", t, th; exit 1 }
  printf "coverage %.1f%% meets the %s%% gate\n", t, th
}'
