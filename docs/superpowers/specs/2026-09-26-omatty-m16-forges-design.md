# M16 - The Forges — design (#448)

Approved 2026-09-26 in a brainstorming session. Seventeen implementation
issues, #449-#465, one PR each; this document and the ROADMAP section are #448.
Every issue is in Backlog: M16 is designed and captured, not scheduled.

## Context

omatty reads its forge only through `gh`. Three features depend on it: a
card's pull request and CI (#310), the `ctrl+o i` tracker (M14), and #331's
push / open / merge. A project on GitLab, Azure DevOps, Gitea, Forgejo,
Codeberg or Bitbucket, in the cloud or self-hosted, gets "this project is not
on GitHub" and nothing more. Those teams are a large share of the people a
terminal ADE is for.

What the code looks like today:

- `internal/forge` is a concrete `CLI{bin: "gh"}`. Its methods are
  `ListPRs`, `ListIssues`, `ViewIssue`, `ViewPR` and `Browse`, and the folds
  (`Fold`, `FoldIssues`, `FoldDetail`) are pure functions over gh's JSON.
- The UI depends only on four func types: `PRListFunc`, `IssueListFunc`,
  `ItemFunc` and `BrowseFunc`. `withForgeDeps` in `cmd/omatty/wiring.go`
  wires them, and that is already the seam a router plugs into.
- Nothing reads the git remote. "Not GitHub" is inferred from gh's stderr
  (`forge.classify`), and the UI stops polling that project (`loseGitHub`).
- The vocabulary is GitHub's. The errors are `ErrNoGH` and `ErrNotGitHub`,
  and the copy is "gh is not installed" and "not on GitHub".

Decisions taken with the user:

| Decision | Choice |
|---|---|
| Forges | **GitLab** (gitlab.com and self-managed), **Azure DevOps** (Services and Server), **Gitea / Forgejo / Codeberg**, **Bitbucket** (Cloud and Data Center). |
| Transport | **CLI first, REST fallback.** omatty uses the forge's own CLI on the operator's own auth when it is installed. Otherwise it calls the forge's REST API with a token borrowed from the environment. |
| Boards | **Out.** Issues and PRs only, which is parity with what GitHub has today. No forge's board is read, GitHub Projects included. |
| Commitment | **A milestone, M16, every issue in Backlog.** |

## What parity means

Parity means that every forge gets exactly what GitHub has today, and nothing
more:

1. Open PRs (MRs on GitLab) with a CI rollup: failing > running > passing.
   Each carries conflict, draft and fork, and a card is matched to its PR by
   branch, as #310 does it.
2. Open issues in the tracker (work items on Azure DevOps).
3. One item's body and comments, cut at `DetailMax`.
4. Browse, which opens the item in the operator's browser.
5. Later, once #331 exists on GitHub: #331's three ship actions and its
   refusals (#464).

## Shared rules every slice follows

1. **One package owns every forge.** `internal/forge` is the only package that
   runs a forge CLI or makes a forge HTTP call. The exec allowlist does not
   grow. `TestNoGhOutsideForge` becomes a fence over every forge CLI name, and
   `net/http` gets its own depguard fence (#453). The blast radius of a
   changing forge API stays in one package, which is invariant 4 in spirit.
2. **The interface lives in the consumer.** The UI's func types keep their
   shape, and `forge.Router` provides them (#452). No cross-package forge
   interface is added. This follows the ARCHITECTURE.md rule the agent seam
   follows.
3. **Detect, never guess.** A forge's kind comes from the remote's host,
   looked up first in `[forge.hosts]` and then in a built-in table. An unknown
   host is `ErrNoForge`: quiet, and no polling, exactly like today's "not
   GitHub". omatty never probes a host to find out what it is.
4. **One set of values.** Every backend and every transport folds into the
   same `PR`, `Issue` and `Detail`. A UI test cannot tell GitLab-over-REST
   from GitHub-over-gh, and none of the UI changes per forge except the copy.
5. **Unknown is never drawn as zero** (M14). A forge that answers nothing
   leaves the header as it was.
6. **Tokens are borrowed, never kept.** A token is read from the environment
   per call. It is never written to `config.toml`, `state.json`, a log line or
   an error message. A test asserts the redaction (#453).
7. **Read-only until #331.** M16 adds no forge write. #464 carries #331's
   actions, and only those, to the other forges, and only after #331 ships.
8. **Nothing persisted.** A project's forge kind is derived from its remote at
   runtime, so `state.json` does not change (invariant 9).

## The slices

In landing order. A slice's dependencies land before it.

| # | Slice | Depends on |
|---|---|---|
| **Foundation** | | |
| #449 | forge-neutral vocabulary: `MissingToolError`, `ErrNoForge`, the PR/MR noun in the copy | - |
| #450 | read the remote and name its forge: `vcs.RemoteURL`, `ParseRemote`, `KindOf` | - |
| #451 | `[forge.hosts]` for self-hosted forges | #450 |
| #452 | `forge.Router` with GitHub as its first backend; every forge CLI fenced | #449, #450 |
| #453 | REST fallback transport: env-borrowed tokens, bounded bodies, `net/http` fence | #452 |
| **GitLab** | | |
| #454 | backend via `glab` | #452 |
| #455 | REST fallback, `/api/v4` | #453, #454 |
| **Azure DevOps** | | |
| #456 | backend via `az` (`azure-devops` extension) | #452 |
| #457 | REST fallback, `_apis/git` + `_apis/wit` | #453, #456 |
| **Gitea / Forgejo / Codeberg** | | |
| #458 | backend via `tea`, after a half-day check that its JSON output is enough | #452 |
| #459 | REST fallback, `/api/v1` | #453, #458 |
| **Bitbucket** | | |
| #460 | Cloud, REST only (`api.bitbucket.org/2.0`) | #453 |
| #461 | Data Center, REST only (`/rest/api/1.0`) | #460 |
| **GitHub** | | |
| #462 | REST fallback when `gh` is absent (`GH_TOKEN`) | #453 |
| **Close-out** | | |
| #463 | forgeprobe and sanitized fixtures for every backend | #454-#462 |
| #464 | #331's ship actions on every forge | #331, #454, #456, #458, #460, #461 |
| #465 | forge support matrix in README and `comparison.md` | #463 |

Each issue carries its own "Done when" and its forge's field mapping. This
table is the order, not the spec of each slice.

### The mapping, in one table

| | PRs and CI | Issues | CLI | REST token |
|---|---|---|---|---|
| GitHub | `pr list` + `statusCheckRollup` | `issue list` | `gh` | `GH_TOKEN` / `GITHUB_TOKEN` |
| GitLab | MRs + `head_pipeline.status` | issues | `glab` | `GITLAB_TOKEN` |
| Azure DevOps | active PRs + build-policy evaluations | work items by WIQL | `az` + `azure-devops` | `AZURE_DEVOPS_EXT_PAT` |
| Gitea / Forgejo | pulls + combined commit status | issues | `tea` | `GITEA_TOKEN` (optional on public repos) |
| Bitbucket Cloud | pullrequests + commit statuses | built-in tracker, when enabled | none | `BITBUCKET_USER` + `BITBUCKET_TOKEN` |
| Bitbucket DC | pull-requests + build-status | none (Jira is out) | none | `BITBUCKET_TOKEN` |

Where the forge's CLI already reads a token variable (`gh`, `glab`, `az`),
omatty uses the same name, so an operator who has used the CLI has probably set
it already. For Gitea/Forgejo and Bitbucket, #459 and #460 confirm the name
against the tools in use before they fix it.

### Five decisions worth the space

- **Why CLI first.** The CLI is the operator's existing, audited credential,
  used on a keypress, and #310 won that argument for `gh`. A CLI also absorbs
  the forge's auth quirks: SSO, device flow, token refresh. omatty
  reimplementing those would be a login system, and that is refused.
- **Why a REST fallback at all.** Bitbucket has no official CLI. `tea` may
  not expose enough fields (#458's check). Many CI images and fresh machines
  have a token in the environment but no CLI. Without REST, three forges of
  five would be partial at best.
- **Why a router in one package, not a package per forge.** A package per
  forge means five more `os/exec` importers. The allowlist says adding even
  one "is a decision". It also means five packages that `ui` would have to
  choose between. One package with an unexported backend per forge keeps
  every fence where it is. A second forge is a new file, which is how the
  agent seam grows too (#46).
- **Why detection by host and config, not probing.** Probing
  (`/api/v4/version`, `/api/v1/version`) means a network call to find out
  where to make network calls, it is ambiguous behind proxies, and it guesses.
  A self-hosted forge is named once in `[forge.hosts]`, and GitHub Enterprise
  comes free with that line.
- **Why Azure's work items are in and boards are out.** On Azure DevOps a work
  item *is* the issue, and there is no other issue list to show. #456 queries
  open work items with WIQL, which is a list with no columns. Showing which
  board column an item sits in is the board, and boards are out for every
  forge.

## Amended cuts

- **M14: "a `[forge]` config section" was deliberately out**, because "the
  poll is zero-config". M16 adds `[forge.hosts]` (#451). The poll stays
  zero-config for every host in the built-in table. The section exists only
  to name a host omatty cannot recognise.
- **"omatty ... holds no token of its own"** (#310, and #331's "there is no
  account, no token and no sync"). The REST fallback (#453) reads a token the
  operator already put in the environment. omatty stores nothing, logs
  nothing, has no login flow and no account, and syncs nothing. The refusal
  "cloud, accounts, sync" still holds, because nothing hidden is created. The
  sentence changes from "holds no token" to "**stores no token**", and #453
  carries the `invariant` label for it.

## Deliberately out

- **Boards on every forge**, GitHub Projects included. That was the user's
  call, and the "planning board inside the TUI" refusal is still argued from
  two sources of truth. Reading a board's columns is a later idea with its own
  brainstorm.
- **Jira**, and every tracker that is not the forge itself. Bitbucket Data
  Center shows PRs only.
- **Forge writes beyond #331's three.** M14's refusal list is unchanged: no
  create, no comment, no close, no label, no assign.
- **OAuth, device flow, or any login inside omatty**, and any token store.
- **Several remotes per project.** omatty reads `origin`. A fork workflow
  whose PRs live on `upstream` is a later question.
- **Per-check CI detail**, which is the same cut M14 made.
- **Polling periods per forge.** The #310 and M14 timings apply everywhere.
- **SourceHut, Gerrit, Phabricator, AWS CodeCommit.** Their review models are
  not PRs, or they are end-of-life. Adding one later is a new backend file.

## Verification

Each slice's PR clears the full local gate (AGENTS.md) and CI on both runners.
Every bug fix follows the regression-test procedure.

The gate is necessary, not sufficient. Every backend's unit tests substitute
a fake for the CLI (a shell script, the `NewCLIWithBin` pattern) or for the
API (a named `httptest` fake over sanitized fixtures in
`testdata/forge/<kind>/`). #463 is the milestone's smoke test: one real
`forgeprobe` run per forge and transport, read by a person, against public
repositories on gitlab.com, codeberg.org and bitbucket.org, and against Azure
DevOps or a self-hosted instance wherever one is available. The PR names every
forge that was *not* probed, and #465 claims nothing a probe did not show.
