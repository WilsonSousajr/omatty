#!/bin/sh
# release-notes.sh vX.Y.Z - print CHANGELOG.md's section for that version, the
# notes a release publishes (#327). The changelog stays the one place a release
# is described (#134). The section runs from its "## [vX.Y.Z]" heading to the
# next "## [" heading or the link references at the bottom, with blank lines
# trimmed at both ends. A version with no section exits 1 and prints nothing:
# a tag the changelog does not describe must not publish.
set -eu

version=${1:?usage: release-notes.sh vX.Y.Z}
changelog=${CHANGELOG:-CHANGELOG.md}

notes=$(awk -v heading="## [$version]" '
	index($0, heading) == 1 { inside = 1; found = 1; next }
	inside && (/^## \[/ || /^\[[^]]+\]: /) { exit }
	inside { print }
	END { if (!found) exit 1 }
' "$changelog") || {
	echo "release-notes.sh: $changelog has no section for $version" >&2
	exit 1
}

# Trim the blank lines around the section; the body keeps its own.
printf '%s\n' "$notes" | awk 'NF { started = 1 } started' | awk '
	{ lines[NR] = $0 } NF { last = NR }
	END { for (i = 1; i <= last; i++) print lines[i] }
'
