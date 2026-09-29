#!/usr/bin/env bash
# Reports ADR 0001's layer rule against the import graph (#620).
# Usage: check-layers.sh [-enforce]
#
# Report-only: it prints every import the target architecture forbids and
# exits 0, because today's code predates that architecture and a gate that
# failed on it would block every pull request that is not the migration. The
# count on the last line is the migration's backlog. The migration's last PR
# adds -enforce here and in CI, the way check-deps.sh went from report to
# gate once its margin had been watched (#263, #269).
set -euo pipefail
go run ./tools/layercheck "$@"
