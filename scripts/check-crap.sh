#!/usr/bin/env bash
# Enforces the C.R.A.P. gate. Usage: check-crap.sh [threshold] [profile]
#
#   CRAP(f) = CC(f)^2 * (1 - coverage(f))^3 + CC(f)
#
# 12, not the canonical 30. 30 cannot fire in this repository: .golangci.yml
# caps gocyclo at 10, and a CC=10 function sitting exactly on the 90% coverage
# floor scores 10.1 - a third of the threshold.
#
# It shipped at 15 (#262), which was then the lowest green value: the worst two
# functions in the tree were internal/watcher's PromptText and typedText, both
# CC=3 with no test reaching them at all, scoring exactly 3^2 * 1^3 + 3 = 12.0.
# Testing those two (#267) was the whole point of the number - the gate found
# an *exported* function at 0% inside a package measuring 92.2% - and with them
# covered the worst score in the tree is 8.2 (internal/ui renderPreview, CC=8
# at 86.7%). So the gate moves to 12 with a margin of nearly four.
#
# Comparison is `fail if crap >= threshold`, deliberately, so CC=3 at zero
# coverage is caught exactly rather than by a hair either side of a float.
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
threshold="${1:-12}"
profile="${2:-cover.out}"

if [ ! -f "$profile" ]; then
  go test ./internal/... -race -coverprofile="$profile"
fi

go run ./tools/crapcheck -threshold "$threshold" -profile "$profile"
