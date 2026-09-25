# Release pipeline — design (#327)

Approved 2026-09-24. A tag push builds binaries and a Homebrew tap.

## Context

v0.2.0 shipped only as `go install` (needs Go 1.26.8 on the user's machine) and
a GitHub release made by hand; nothing turns a tag into binaries. #327 asks for
a `v*` tag push to build darwin/linux × amd64/arm64 binaries, attach archives and
checksums to the GitHub release, and publish a Homebrew install — so a stranger
needs no Go toolchain, and so a release is cheap enough to do per milestone
(#329's question). Decisions taken with the user:

| Decision | Choice |
|---|---|
| Tooling | **GoReleaser v2**, pinned (latest v2.18.2, goreleaser-action v7.2.3) |
| Homebrew | **Separate public tap `WilsonSousajr/homebrew-tap`**, pushed with a fine-grained token (Contents: write on that repo only) in the secret `HOMEBREW_TAP_TOKEN`; `homebrew_casks` (GoReleaser's current path since v2.10 — `brews` is deprecated; casks are macOS-only, so Linux uses the tarball) |

Verified: all four targets build with `CGO_ENABLED=0` (one Ubuntu runner builds
everything); `cmd/omatty/version.go` already reads `-X main.version`.

## The pipeline

1. **`scripts/release-notes.sh vX.Y.Z`** — prints `CHANGELOG.md`'s `## [vX.Y.Z]`
   section up to the next `## [` heading, exits non-zero when the version has no
   section (a tag without notes must not publish). Tested from Go in `scripts/`
   (the package's existing pattern): v0.2.0 and v0.1.0 bodies start/end at the
   right headings and never include `[Unreleased]`; an unknown version fails.
2. **`.goreleaser.yaml`** (v2): `builds` `./cmd/omatty`, `CGO_ENABLED=0`,
   darwin/linux × amd64/arm64, `ldflags: -s -w -X main.version={{.Tag}}`;
   `archives` tar.gz `omatty_{{.Version}}_{{.Os}}_{{.Arch}}` + README.md,
   CHANGELOG.md; `checksum`; `release` on the tag (notes via
   `--release-notes`); `homebrew_casks` → `WilsonSousajr/homebrew-tap`, token
   `{{ .Env.HOMEBREW_TAP_TOKEN }}`, with the macOS quarantine-removal post-install
   hook (the binary is unsigned).
3. **`.github/workflows/ci.yml`**: add `workflow_call:` to its triggers so the
   release can reuse the gate; add `GORELEASER_VERSION` to `env`; a new job
   `release-config` (ubuntu) running `goreleaser check` and
   `goreleaser release --snapshot --clean --skip=publish` via
   `goreleaser/goreleaser-action` pinned, so a broken config fails a PR.
4. **`.github/workflows/release.yml`**: `on: push: tags: ['v*']`; job `gate`
   `uses: ./.github/workflows/ci.yml`; job `release` `needs: gate`,
   `permissions: contents: write`, checkout with `fetch-depth: 0`, setup-go from
   `GO_VERSION`, `scripts/release-notes.sh "$GITHUB_REF_NAME" > notes.md`, then
   goreleaser `release --clean --release-notes notes.md` with
   `GITHUB_TOKEN` and `HOMEBREW_TAP_TOKEN`.
5. **`scripts/hygiene_test.go`**: `TestCI_PinsGoReleaser` — `GORELEASER_VERSION`
   is an exact `v2.x.y` in `ci.yml` and `release.yml` and the two agree (same
   reason as the golangci/govulncheck pins).
6. **Docs**: README Install leads with `brew install WilsonSousajr/tap/omatty`
   (macOS) and the release tarball (Linux), `go install` kept as from-source;
   AGENTS.md "Branches and releases" + ROADMAP "Releases": tagging the merge
   commit now builds the release (no more `gh release create` by hand) —
   "no tag without explicit approval" unchanged; CHANGELOG `### Added` (#327).

**Stops before anything outward-facing:** creating `WilsonSousajr/homebrew-tap`
(`gh repo create`, needs the user's go-ahead) and the token/secret (the user
creates it — I can't mint credentials). The PR merges under the normal gate;
the first real run is v0.3.0, which needs its own tag approval.

**Flag for the user:** the repository has **no LICENSE file**. Publishing
binaries and a Homebrew cask without one leaves the terms unstated; the
archives will carry README and CHANGELOG only until the user picks a license.

## Critical files

`.goreleaser.yaml` (new), `.github/workflows/release.yml` (new),
`.github/workflows/ci.yml`, `scripts/release-notes.sh` (new),
`scripts/release_notes_test.go` (new), `scripts/hygiene_test.go`, `README.md`,
`AGENTS.md`, `docs/ROADMAP.md`, `CHANGELOG.md`. Reuses `cmd/omatty/version.go`'s
`-X main.version`, `ci.yml`'s `GO_VERSION`/pin pattern and the `scripts` test helpers
(`ciWorkflow`, `repoRoot`).

## Verification

- `go test ./scripts` (release-notes + pin tests); the full gate.
- Locally: `go run github.com/goreleaser/goreleaser/v2@v2.18.2 check` and
  `… release --snapshot --clean --skip=publish` → four archives + checksums in
  `dist/` (git-ignored); `dist/omatty_darwin_arm64*/omatty --version` prints the
  snapshot version; a real-PTY launch of that binary against a scratch HOME.
- CI: the new `release-config` job green on the PR; gate green on both runners.
- After merge, with the tap and secret in place: tag v0.3.0 (user approval) →
  release has 4 archives + checksums + notes; `brew install WilsonSousajr/tap/omatty`
  on a Mac without Go on PATH → `omatty --version` = v0.3.0.
