#!/usr/bin/env bash
# Enforces AGENTS.md's "files under 500 lines" (#609).
# Usage: check-file-length.sh [limit] [file...]
#
# The rule was written down and never checked, so #136 brought model.go and
# main.go under it before M7 and six files were back over it by v0.9.0. A rule
# nothing runs is a suggestion. Longer means the package boundary is wrong,
# so the answer to a failure is a split by responsibility, never a higher
# limit: the number is a ratchet that only moves down.
#
# Every tracked .go file counts, tests included - a test file is read and
# maintained exactly as often as the code it covers.
#
# Files may be named on the command line, which is how its tests measure a
# fixture without a repository around it. With none, it measures the checkout;
# a list that comes back empty fails rather than passing, because a gate that
# measured nothing has proved nothing.
#
# The longest file is printed on every run, clean or not, the way
# check-deps.sh prints its tightest margin: it is the warning before the break.
set -euo pipefail
limit="${1:-500}"
[ "$#" -gt 0 ] && shift

if [ "$#" -gt 0 ]; then
  files=$(printf '%s\n' "$@")
else
  files=$(git ls-files '*.go' 2>/dev/null || true)
fi
if [ -z "$files" ]; then
  echo "check-file-length: no .go files to measure" >&2
  exit 1
fi

status=0
longest=0
longest_path=""
while IFS= read -r f; do
  n=$(wc -l < "$f" | tr -d ' ')
  if [ "$n" -gt "$longest" ]; then
    longest=$n
    longest_path=$f
  fi
  if [ "$n" -gt "$limit" ]; then
    echo "$n $f"
    status=1
  fi
done <<< "$files"

echo "longest: $longest $longest_path (limit $limit)"
exit "$status"
