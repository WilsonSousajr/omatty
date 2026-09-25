# Release pipeline — implementation plan (#327)

> Executed inline (executing-plans). Spec: `docs/superpowers/specs/2026-09-24-release-pipeline-design.md`.

**Goal:** a `v*` tag push re-runs the gate, then GoReleaser publishes four binaries, checksums, notes from CHANGELOG and a Homebrew cask.

**Global constraints:** GoReleaser `v2.18.2` and goreleaser-action `v7.2.3` pinned; `CGO_ENABLED=0`; `-X main.version={{.Tag}}`; no tag without the maintainer's approval (unchanged); nothing outward-facing (tap repo, token) is created by this branch.

## Review Focus

1. A tag whose version has no CHANGELOG section → the release job fails before publishing.
2. A tag pushed without the gate passing → nothing publishes (`needs: gate`).
3. A broken `.goreleaser.yaml` → a PR fails (`release-config` job), not a tag.
4. `--version` of a released binary prints exactly `omatty vX.Y.Z`.
5. `HOMEBREW_TAP_TOKEN` absent → the snapshot job (no publish) still passes; only the tag run needs it.

## Tasks

1. **`scripts/release-notes.sh` + `scripts/release_notes_test.go`** (TDD): v0.2.0 and v0.1.0 sections are exact, never `[Unreleased]`, stop at the next `## [`; unknown version exits non-zero with nothing on stdout. Commit `ci(#327): release notes come from the changelog`.
2. **`.goreleaser.yaml`**, verified locally with `go run github.com/goreleaser/goreleaser/v2@v2.18.2 check` and a `--snapshot --skip=publish` build; the darwin_arm64 binary's `--version` and a real-PTY launch. Commit `ci(#327): goreleaser builds four binaries, checksums and a cask`.
3. **Workflows**: `ci.yml` gains `workflow_call`, `GORELEASER_VERSION`, and the `release-config` job; new `release.yml` (`gate` via `uses`, then `release`). `TestCI_PinsGoReleaser` in `scripts/hygiene_test.go` (TDD). Commit `ci(#327): a v* tag re-runs the gate, then releases`.
4. **Docs**: README Install, AGENTS.md "Branches and releases", ROADMAP "Releases", CHANGELOG. Full gate; push; PR (`Closes #327` only once the first tag run passes? No — the issue's "done when" is the brew install, so the PR says `Part of #327`); CI green → merge under the normal gate; then hand the maintainer the tap/token steps.
