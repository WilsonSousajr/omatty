#!/bin/sh
# cask-steps.sh CASK.rb - write the cask's install hook as `postflight_steps`,
# in place (#369). GoReleaser v2.18.2 can only render the hook as a raw
# `postflight do ... end` block, which Homebrew deprecates: every `brew
# install` printed a warning asking the installer to report a bug in our tap.
# So .goreleaser.yaml renders the cask without the hook and does not upload
# it, this script writes the hook, and release.yml publishes the result.
#
# Delete this script, and give the hook back to .goreleaser.yaml, once a
# GoReleaser release can emit install steps itself (goreleaser#6873).
#
# The hook exists because the binary is unsigned: without it Gatekeeper
# refuses a freshly installed copy. `{{staged_path}}` is a token rather than a
# path because a step's args are plain strings, never resolved against the
# staged directory; `must_succeed: false` keeps the tolerance system_command
# had. The stanza goes straight after `binary`, where Homebrew's stanza order
# puts postflight_steps: after the artifacts, before uninstall and zap.
set -eu

cask=${1:?usage: cask-steps.sh CASK.rb}

refuse() {
	echo "cask-steps.sh: $cask $1" >&2
	exit 1
}

grep -q '^  postflight do$' "$cask" &&
	refuse "has a deprecated postflight block; take the hook out of .goreleaser.yaml"
grep -q '^  postflight_steps do$' "$cask" &&
	refuse "already has postflight_steps; if GoReleaser writes them now, delete this script"
grep -q '^  binary "omatty"$' "$cask" ||
	refuse 'has no binary "omatty" stanza to put the hook after'

patched="$cask.steps"
awk '
	{ print }
	$0 == "  binary \"omatty\"" {
		print ""
		print "  postflight_steps do"
		print "    on_macos do"
		print "      run \"/usr/bin/xattr\", args: [\"-dr\", \"com.apple.quarantine\", \"{{staged_path}}/omatty\"], must_succeed: false"
		print "    end"
		print "  end"
	}
' "$cask" >"$patched"
mv "$patched" "$cask"
