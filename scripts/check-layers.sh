#!/usr/bin/env bash
# Reports ADR 0001's layer rule against the import graph (#620).
# Usage: check-layers.sh [-enforce]
#
# Without -enforce it prints every import the architecture forbids and exits
# 0. It landed that way because the code predated the architecture, and a gate
# that failed on it would have blocked every pull request that was not the
# migration; the count on the last line was the migration's backlog. The
# migration ended at zero, and CI and AGENTS.md's gate list now pass -enforce
# (migration step 8.2, #653), the way check-deps.sh went from report to gate
# once its margin had been watched (#263, #269).
set -euo pipefail
go run ./tools/layercheck "$@"
