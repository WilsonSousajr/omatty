#!/usr/bin/env bash
# Enforces the C.R.A.P. gate. Usage: check-crap.sh [threshold] [profile]
#
#   CRAP(f) = CC(f)^2 * (1 - coverage(f))^3 + CC(f)
#
# 15, not the canonical 30. 30 cannot fire in this repository: .golangci.yml
# caps gocyclo at 10, and a CC=10 function sitting exactly on the 90% coverage
# floor scores 10.1 - a third of the threshold. Measured over all 854 functions
# the worst score in the tree is 12.0, shared by internal/watcher's PromptText
# and typedText, both CC=3 with no test reaching them at all. 15 is the lowest
# value that is green today and still constrains anything.
#
# To re-pick it: run with a threshold of 999, which always passes, read the top
# rows, and set the gate one step above the worst score worth keeping. Same
# measure-then-set discipline that chose 90 for coverage.
#
# Reuses the profile check-coverage.sh already wrote, so the two steps cost one
# test run between them - which is why CI orders this one straight after it.
# crapcheck refuses a profile older than the source rather than scoring it:
# coverage blocks carry line and column positions, so an old profile against an
# edited tree attributes them to whatever now sits at those lines and the wrong
# answer looks exactly as trustworthy as the right one.
set -euo pipefail
threshold="${1:-15}"
profile="${2:-cover.out}"

if [ ! -f "$profile" ]; then
  go test ./internal/... -race -coverprofile="$profile"
fi

go run ./tools/crapcheck -threshold "$threshold" -profile "$profile"
