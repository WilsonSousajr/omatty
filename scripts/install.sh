#!/bin/sh
# install.sh - install omatty on macOS or Linux in one line (#517):
#
#   curl -fsSL https://omatty.com/install.sh | sh
#
# With Homebrew on PATH it hands off to the tap, so upgrades stay in one
# place. Otherwise it downloads the release archive for this machine, checks
# it against the release's checksums.txt, and installs the binary to
# ~/.local/bin. Re-running upgrades in place. It never uses sudo, never runs a
# package manager other than that one brew command, and never touches
# ~/.claude.
#
#   OMATTY_VERSION=v0.8.1     that release instead of the latest (never brew)
#   OMATTY_INSTALL_DIR=DIR    install there instead of ~/.local/bin
#   OMATTY_NO_BREW=1          the archive even when brew is on PATH
#   OMATTY_RELEASE_URL=URL    where releases live; tests point it at file://
#
# Everything happens inside main, called on the last line, so a download cut
# short defines functions and runs nothing.
set -eu

say() {
	printf 'omatty: %s\n' "$*"
}

die() {
	printf 'omatty: %s\n' "$*" >&2
	exit 1
}

have() {
	command -v "$1" >/dev/null 2>&1
}

# platform prints the GoReleaser build this machine takes, e.g. darwin_arm64:
# the four .goreleaser.yaml publishes, and nothing else.
platform() {
	case $(uname -s) in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) os= ;;
	esac
	case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) arch= ;;
	esac
	if [ -z "$os" ] || [ -z "$arch" ]; then
		die "no release build for $(uname -s) $(uname -m); build from source instead: go install github.com/WilsonSousajr/omatty/cmd/omatty@latest"
	fi
	echo "${os}_${arch}"
}

fetch() {
	if have curl; then
		curl -fsSL -o "$2" "$1"
	elif have wget; then
		wget -q -O "$2" "$1"
	else
		die "downloading needs curl or wget"
	fi
}

sha256() {
	if have sha256sum; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	else
		die "checking the download needs sha256sum or shasum"
	fi
}

# with_brew installs or upgrades the cask, the same way a user would.
with_brew() {
	if brew list --cask omatty >/dev/null 2>&1; then
		say "upgrading omatty with Homebrew"
		brew upgrade --cask WilsonSousajr/tap/omatty
	else
		say "installing omatty with Homebrew"
		brew install WilsonSousajr/tap/omatty
	fi
	bin=$(command -v omatty) || die "Homebrew finished, but omatty is not on PATH"
}

# release_url prints where this run's checksums.txt and archive live.
release_url() {
	base=${OMATTY_RELEASE_URL:-https://github.com/WilsonSousajr/omatty/releases}
	if [ -z "${OMATTY_VERSION:-}" ]; then
		echo "$base/latest/download"
		return
	fi
	case $OMATTY_VERSION in
	v*) echo "$base/download/$OMATTY_VERSION" ;;
	*) echo "$base/download/v$OMATTY_VERSION" ;;
	esac
}

# from_release downloads the archive for target, refuses it unless its
# sha256 matches checksums.txt, and moves the binary into place.
from_release() {
	url=$(release_url)
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	fetch "$url/checksums.txt" "$tmp/checksums.txt" || die "could not download $url/checksums.txt"
	archive=$(awk -v want="_$target.tar.gz" 'substr($2, length($2) - length(want) + 1) == want { print $2; exit }' "$tmp/checksums.txt")
	case $archive in
	omatty_*_"$target".tar.gz) ;;
	*) die "$url/checksums.txt names no omatty archive for $target" ;;
	esac
	case $archive in
	*/*) die "$url/checksums.txt names $archive, which is not a file in the release" ;;
	esac
	expected=$(awk -v name="$archive" '$2 == name { print $1; exit }' "$tmp/checksums.txt")
	say "downloading $archive"
	fetch "$url/$archive" "$tmp/$archive" || die "could not download $url/$archive"
	actual=$(sha256 "$tmp/$archive")
	[ "$actual" = "$expected" ] ||
		die "$archive does not match its checksum (expected $expected, got $actual); nothing was installed"
	tar -xzf "$tmp/$archive" -C "$tmp" omatty || die "$archive holds no omatty binary"
	dir=${OMATTY_INSTALL_DIR:-$HOME/.local/bin}
	mkdir -p "$dir"
	cp "$tmp/omatty" "$dir/.omatty.tmp"
	chmod 755 "$dir/.omatty.tmp"
	mv -f "$dir/.omatty.tmp" "$dir/omatty"
	bin=$dir/omatty
	say "installed $bin"
	case ":$PATH:" in
	*":$dir:"*) ;;
	*) say "$dir is not on your PATH; add it with: export PATH=\"$dir:\$PATH\"" ;;
	esac
}

prerequisites() {
	have git || say "git is not on your PATH, and omatty needs it: https://git-scm.com/downloads"
	have claude || say "Claude Code (claude) is not on your PATH, and omatty needs it: https://claude.com/claude-code"
	have dtach || say "optional: with dtach, quitting omatty detaches from sessions instead of ending them: brew install dtach (or: apt install dtach)"
}

main() {
	target=$(platform)
	if [ -z "${OMATTY_NO_BREW:-}" ] && [ -z "${OMATTY_VERSION:-}" ] && have brew; then
		with_brew
	else
		from_release
	fi
	prerequisites
	"$bin" --version
}

main "$@"
