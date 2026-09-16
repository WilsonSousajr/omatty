#!/usr/bin/env bash
# Measures the package dependency structure of ./internal/... and enforces the
# two rules that hold across it. Usage: check-deps.sh
#
# Reported: Ca, Ce and instability I = Ce/(Ca+Ce) per package, plus the margin
# on the tightest edge. The margin is printed on every run, clean or not,
# because I is a ratio of small integers and moves in jumps - internal/config is
# Ca=1 Ce=1, and one new importer takes it from 0.50 to 0.33. A gate that only
# ever says "clean" gives no warning before it breaks.
#
# Enforced today: import cycles through the *test* graph. The compiler refuses a
# cycle in production imports, so a check there could never fire; a_test -> b ->
# a is legal, compiles, and couples two packages in a direction their production
# code does not admit to.
#
# Also enforced, since #269: the Stable Dependencies Principle, for every edge
# A -> B, I(A) >= I(B). It landed report-only behind --sdp on purpose - a gate
# that fails on a margin nobody has watched move is a gate people learn to
# --no-verify past, and one snapshot cannot show whether a ratio of small
# integers holds steady under ordinary work.
#
# It does. Across every merge from #263 to #278 the tightest edge stayed
# watcher -> registry at exactly +0.071, through a change that took the graph
# from 35 edges to 36. So the flag is gone and a violation fails the run.
#
# Distance from the main sequence is deliberately absent. Go declares interfaces
# at the consumer and usually unexported, so a stable pure leaf like
# internal/paths scores maximum distance while being exactly what AGENTS.md
# designed. Gating on it would demand the speculative interfaces AGENTS.md bans.
#
# Universe is ./internal/... alone. Adding cmd/omatty raises Ca on thirteen
# packages, dropping their I and putting two edges at a zero margin, where the
# next import anywhere near internal/watcher would fail the gate for a reason
# nobody would recognise as architectural.
set -euo pipefail
go run ./tools/depcheck "$@"
