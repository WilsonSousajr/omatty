# The field omatty ships into, September 2026 — inventory

> Captured **2026-09-18**, against `develop` at v0.1.0 plus M9, M10 and M11.
> Every load-bearing claim below was checked against a live primary page on
> that date, not remembered. Repository numbers come from the GitHub API
> (`gh api repos/{owner}/{repo}`, `/releases/latest`, `search/issues`), and
> the script that produces them is in "Reproducing the table" so the next pass
> re-runs rather than re-invents it.
>
> **Analysis only.** No implementation decision is made in this file, and no
> recommendation in §5 binds the roadmap. What omatty should *do* about any of
> it is decided in `docs/ROADMAP.md`, in the open.

## Why this document exists

omatty makes three statements about its competition, and until this file
existed the repository could not support any of them.

1. `README.md`: "Every other tool in this space is either a desktop app or
   scoped to a single repository."
2. M9's thesis, in `docs/ROADMAP.md`: v0.1.0 "shipped into a field of roughly
   a hundred and fifty agent orchestrators."
3. "Not on the roadmap" refuses seven features, each against a field it never
   names.

A refusal is only worth the evidence behind it. M9 argued that omatty bets on
a different variable from everyone else — how fast a person can tell whether
the work is any good, rather than how much agent-work is in flight — and that
bet is the reason M3, M9 and M10 exist and the reason six other features do
not. A bet stated against an unnamed field is a preference. This file names
the field; #300 decides whether the bet survives it.

### What is not a source

This category is unusually polluted. Searching for any of these tools returns,
above the projects themselves, a layer of "X alternatives" and "best tools for
Y" pages — `abralo.com/alternatives`, `runpane.com/alternatives/claude-squad`,
`nimbalyst.com/blog/...`, `codeagentswarm.com/guides/...`, `munderdiffl.in/blog`
— each published by a vendor that appears in its own ranking. They are not
sources here. They may be cited only as evidence of how a vendor positions
itself, and are labelled as such where they are.

Primary sources only: the repository, its README, its issue tracker, its
releases, the vendor's own documentation.

## 1. The camps

Six of them. The first three compete with omatty for the same hour of the same
person's day; the last three shape what that person expects.

**A. Terminal session managers.** A TUI that runs several agent sessions, each
in its own git worktree, and switches between them. omatty's own camp:
`smtg-ai/claude-squad` (tmux + `gh`, AGPL, the one that defined the shape),
`kbwo/ccmanager` (no tmux, TypeScript, eight agent CLIs), `brizzai/fleet`
(Go + tmux, sessions grouped by repo, status by hooks), and the dead
`devflowinc/uzi`.

**B. Desktop and hybrid workspaces.** The same job with a window manager
instead of a terminal: `stablyai/orca`, `nimbalyst/nimbalyst` (the former
`stravu/crystal`, renamed — see below), `imbue-ai/sculptor`, and the
closed-source Conductor. Orca is the outlier in every dimension and is treated
separately in section 3.

**C. Board and queue orchestrators.** The camp M9 refuses by name. Work is
tickets; agents are workers; the human writes tasks and reads results.
`BloopAI/vibe-kanban` is the giant.

**D. Isolation and sandboxing.** `dagger/container-use`, `ykdojo/safeclaw`,
`akitaonrails/ai-jail`. Orthogonal rather than competing — they answer "what
can the agent touch", where camps A-C answer "how do I watch several of them".
A session manager can sit on top of one.

**E. The verification camp.** The one a survey shaped around parallelism would
miss entirely, and the one omatty's thesis actually competes with:
`jesseduffield/lazygit`, `dandavison/delta`, `Wilfred/difftastic`,
`gh pr diff`, AI review bots, and plain CI. If "tell me fast whether this is
any good" is the product, then the incumbent is not claude-squad. It is
lazygit beside a terminal, with CI in a browser tab. #300 owes an honest answer
to that.

**F. First-party.** Anthropic's own surfaces. Section 3.

## 2. Signal table

Stars are GitHub's raw counts and are a popularity signal, not a quality one;
they are here because a 71k-star project and a 53-star project are different
kinds of fact about the field. Last-commit and release dates are what say
whether anyone is still home.

| Project | Camp | Stars | Last commit | Latest release | Open issues | Licence |
|---|---|---:|---|---|---:|---|
| `stablyai/orca` | B | 71,802 | 2026-09-18 | v1.4.205 (2026-09-17) | 3,036 | MIT |
| `BloopAI/vibe-kanban` | C | 28,123 | 2026-09-18 | v0.1.44 (2026-04-24) | 384 | Apache-2.0 |
| `smtg-ai/claude-squad` | A | 8,495 | 2026-08-20 | v1.0.20 (2026-08-20) | 19 | AGPL-3.0 |
| `dagger/container-use` | D | 4,047 | 2026-09-14 | v0.4.2 (2025-08-19) | 47 | Apache-2.0 |
| `stravu/crystal` | B | 3,118 | 2026-02-26 | v0.3.5 (2026-02-26) | 60 | MIT |
| `nimbalyst/nimbalyst` | B | 1,737 | 2026-09-17 | v0.77.5 (2026-09-09) | 573 | MIT |
| `kbwo/ccmanager` | A | 1,246 | 2026-09-13 | v4.4.3 (2026-09-13) | 4 | MIT |
| `akitaonrails/ai-jail` | D | 1,229 | 2026-09-13 | v1.21.0 (2026-09-13) | 1 | GPL-3.0 |
| `devflowinc/uzi` | A | 583 | **2025-06-04** | v0.0.2 (2025-06-03) | 6 | MIT |
| `imbue-ai/sculptor` | B | 233 | 2026-09-18 | v0.47.0 (2026-09-08) | 0 | MIT |
| `ykdojo/safeclaw` | D | 184 | 2026-09-18 | v0.7.0 (2026-07-24) | 0 | MIT |
| `brizzai/fleet` | A | 53 | 2026-09-17 | v2.43.0 (2026-09-17) | 13 | Apache-2.0 |
| *verification camp* | | | | | | |
| `jesseduffield/lazygit` | E | 82,458 | 2026-09-18 | v0.65.1 (2026-09-13) | 862 | MIT |
| `dandavison/delta` | E | 32,227 | 2026-09-15 | 0.19.2 (2026-03-28) | 310 | MIT |
| `Wilfred/difftastic` | E | 25,915 | 2026-09-14 | 0.70.0 (2026-08-07) | 246 | MIT |

Four things the table says that a list of names does not.

- **`stravu/crystal` did not die; it was renamed.** Its repository has been
  untouched since 2026-02-26 and its own description now reads "(Crystal is
  now Nimbalyst)". `nimbalyst/nimbalyst` is MIT, active, and on v0.77.5. Any
  comparison that cites Crystal's stale repo as evidence of abandonment is
  wrong, and this is exactly the error a secondary source would have produced.
- **`devflowinc/uzi` is the one that is actually dead** — no commit in fifteen
  months, still on v0.0.2, still carrying 583 stars. Stars do not decay.
- **The board camp is an order of magnitude larger than the terminal camp.**
  vibe-kanban alone has more than three times claude-squad, ccmanager, fleet
  and uzi combined. The camp M9 refuses is where the field's attention went.
- **claude-squad, the tool that defined omatty's camp, has not been touched
  in four weeks** — last commit and last release are the same day,
  2026-08-20, while every other live project in the table committed within
  five days of capture. One month is not abandonment, and #297 should read the
  tracker before anyone concludes anything.

### Sizing the field, and what M9's number is worth

M9's "roughly a hundred and fifty agent orchestrators" was written without a
source. Probing the GitHub search API on 2026-09-18 with
`gh api "search/repositories?q=...&per_page=1" --jq .total_count`:

| Query | `total_count` |
|---|---:|
| `topic:parallel-agents` | 184 |
| `claude code parallel in:description stars:>10` | 205 |
| `coding agent orchestrator in:description stars:>10` | 462 |
| `multiple claude code sessions in:description` | 446 |
| `claude code worktree in:description` | 1,300 |

The claim survives as an order of magnitude and only that. "Roughly a hundred
and fifty" is defensible against the two narrowest queries and understates the
looser ones by up to ten times, and none of these counts filters for a tool
anyone maintains. #300 should either cite a named query with its date or drop
the number for the qualitative claim it is standing in for, which the rest of
this document supports without it.

## 3. What the notable ones actually are

Enough per project to place it. The per-project deep dives are #296, and the
sections below deliberately stop short of judgement.

**`smtg-ai/claude-squad`** — Go TUI, requires `tmux` and `gh`. One isolated git
workspace per task, agent-agnostic (Claude Code, Codex, Gemini, Aider), and a
`--autoyes` mode it labels experimental. Its README claims a review surface:
"Review changes before applying them, checkout changes before pushing them",
with a diff tab beside a preview tab.

**`kbwo/ccmanager`** — TypeScript TUI, and the most direct comparison omatty
has. No tmux; it owns the PTY itself. Eight agent CLIs. Its feature list
includes several things omatty built as milestones: **visual status indicators
for busy / waiting / idle** (M2), **restore sessions after a restart** (M6),
**status change hooks** (M2), and a `.worktreeinclude` for carrying gitignored
files such as `.env` into new worktrees, which omatty has no answer to at all.

It also documents a **Multi-Project Mode**: "CCManager can manage multiple git
repositories from a single interface", with `CCMANAGER_MULTI_PROJECT_ROOT`,
"automatic project discovery: recursively finds all git repositories", recent
projects first, and vi-like search. That is M1's multi-project sidebar and
M4's `discover`, in a terminal, shipped.

**This is the fact that breaks `README.md`'s field claim.** "Every other tool
in this space is either a desktop app or scoped to a single repository" is
false as written: ccmanager is a terminal tool that is not scoped to a single
repository, and `brizzai/fleet` groups sessions by repo in a TUI as well. The
sentence needs to be replaced with whatever is still true — that is #300's
job, not this file's, but the correction is not optional.

**`brizzai/fleet`** — Go + Bubble Tea + tmux, Apache-2.0, released the day
before capture, and architecturally the nearest sibling omatty has: "a terminal
cockpit for orchestrating Claude Code, Codex & OpenCode sessions in parallel",
with "sessions grouped by repo", "real-time status via hooks", "PR state" and
"one-key approve". Fifty-three stars against omatty's zero-audience release,
which is the useful part: this shape is being arrived at independently.

**`nimbalyst/nimbalyst`** (ex-`stravu/crystal`) — Electron, MIT, "open-source
visual workspace", parallel worktree sessions, **red/green diff review** where
you step through the agent's edits and accept or reject, and an iOS companion
that swipes through diffs.

**`stablyai/orca`** — the one that matters most, and the one omatty's README
has not accounted for. Created 2026-03-17; 71,802 stars and 4,695 forks in six
months; MIT; TypeScript; YC-backed; macOS, Windows and Linux. Its own topics
include `ade`, `parallel-agents`, `worktrees`, `terminal` and `ghostty`. It
calls itself "the ADE for working with a fleet of parallel agents" — **omatty's
own noun, from a project a hundred times its size.** Its README advertises
parallel worktrees, "Ghostty-class terminals with WebGL rendering, infinite
splits, and scrollback that survives restarts", **SSH worktrees** ("run agents
on a beefy remote box with full file editing, git, and terminals"), a mobile
companion app, and — the line that lands closest to home — **"Annotate AI
Diffs: drop comments on any diff line and ship them back to the agent."**

That last feature is M3. Whether it is *the same* feature is a real question
and not a rhetorical one: omatty anchors comments to line *content* rather
than line numbers, specifically so a comment survives the file changing under
it, and nothing in Orca's README says which it does. #296 must read the code
rather than the marketing before anyone concludes either way.

**`BloopAI/vibe-kanban`** — Apache-2.0, Rust, the board camp's giant. "Review
diffs and leave inline comments — send feedback directly to the agent without
leaving the UI." Its release tag has not moved since 2026-04-24 while its
default branch commits daily, which is a packaging signal rather than a
liveness one.

**`imbue-ai/sculptor`** — small, active, and interesting for a different
reason: its documented workflow `/sculptor-workflow:fix-bug` "runs a short
reproduction interview, writes failing tests" — the same rule as omatty's
"every bug gets a failing test before the fix", shipped as a product feature
rather than a contributor rule.

### 3.1 First-party: Anthropic has shipped the category's core

The largest single risk to a tool in camp A is that the agent's own vendor
ships it. On 2026-09-18 that has already partly happened, and it is verifiable
from Anthropic's own documentation rather than inferred.

- **`claude agents` — agent view**, in research preview, is "one screen for all
  your background sessions: what's running, what needs your input, and what's
  done. Dispatch new sessions, watch their state at a glance instead of
  scrolling through transcripts, and step in only when one needs you."
- It isolates the same way omatty does: "Before editing files, Claude moves the
  session into an isolated git worktree under `.claude/worktrees/`, so parallel
  sessions can read the same checkout but each writes to its own."
- The **desktop app** offers "review diffs visually, run multiple sessions side
  by side, schedule recurring tasks, and start cloud sessions".
- The **web surface** runs "multiple tasks in parallel", and a project will
  "coordinate the parallel sessions for you".

Read plainly: *a terminal screen listing parallel agent sessions with their
state, each isolated in a git worktree* is now a first-party feature. That is
M1 and M2's headline, and no roadmap written before this capture assumed it.

Three things omatty still has that none of the four bullets above does, and
they are stated here as observations for #299 and #300 to test rather than as
consolation: agent view's sessions are *background* sessions watched at a
glance, not the real binary running interactively in a pane you can type into;
everything above is scoped to one working directory, not several repositories
side by side; and none of it runs a project's own `fmt`/`vet`/`lint`/`test`
line and puts the verdict on the session's card, which is M9.

### 3.2 The gate, so far, is nobody else's feature

Across every README read for this file, no tool in camps A, B or C advertises
running the project's own check line per session and showing the verdict
beside the session. claude-squad, Orca, Nimbalyst and vibe-kanban all offer a
diff to review; none offers a verdict. That is a genuinely empty square on the
board as of this capture, and it is the square M9 and M10 built on.

It is one README-level observation, not a finished argument — trackers may show
users asking for it (#297), and absence from a README is not absence from a
product. #300 decides what it is worth.

## 4. Developments worth knowing

Four things changed, or became visible, between v0.1.0 shipping on 2026-09-10
and this capture on 2026-09-18. They are listed in the order they should worry
anyone.

### 4.1 The first party shipped the category's core

§3.1 has the quotations. Restated as the thing it is: `claude agents` gives a
terminal screen of parallel sessions with their state, each in its own git
worktree under `.claude/worktrees/`, in research preview, from the vendor of
the agent omatty embeds.

That is M1's skeleton and M2's status, first-party and free. Every roadmap
argument written before 2026-09-18 assumed that square was empty, and none of
them is void — omatty still runs the real binary interactively in a pane you
type into, across several repositories, with a gate on the card — but the
*reason to exist* can no longer be "watch several sessions at once". It has to
be the part agent view does not do, and that part has to be said out loud in
`README.md` (#301).

The honest read is that this is good news for the thesis and bad news for the
pitch. M9 already bet that parallelism is not the scarce thing; the first party
commoditising parallelism is that bet paying off. What it costs is the sentence
omatty currently opens with.

### 4.2 Orca took the noun

`stablyai/orca` — 71,802 stars in six months, YC-backed, topic `ade` — calls
itself "the ADE for working with a fleet of parallel agents". `README.md`'s
first line is "A terminal ADE". Whatever omatty does about this, it should do
it knowingly: the word now points somewhere else for most people who hear it.

### 4.3 The camp consolidated while the reference implementation went quiet

claude-squad, the project that defined this shape, last committed 2026-08-20
and has three open requests for multi-repository support it has not taken.
ccmanager shipped multi-project mode. fleet shipped hooks-based status, PR
state and worktree file carrying, in Go, at 53 stars. Crystal renamed itself
Nimbalyst and kept going. Nothing in the camp is dying, and nothing in it is
where the field's attention is: vibe-kanban alone outweighs the whole camp
three times over.

### 4.4 The review loop stopped being unusual

Orca annotates diff lines and ships the comments back; vibe-kanban leaves
inline comments and sends them to the agent; Nimbalyst steps through red/green
edits. M3 shipped in a field where this was rare and now sits in one where it
is table stakes. What is *not* table stakes, per #296, is surviving the file
changing underneath — and per §3.2, nobody at all runs the project's check
line per session.

## 5. What this means for omatty

Numbered so #299, #300 and #301 can cite them. Each is a recommendation, and a
recommendation here is an input to the roadmap, not a decision in it.

**R1. Rewrite `README.md`'s opening claim, and do it from evidence.** "Every
other tool in this space is either a desktop app or scoped to a single
repository" is false (ccmanager, fleet). "A terminal ADE" now collides with
Orca. The replacement should say what agent view and the camp do *not* do:
the real binary interactively in a pane, several repositories at once, a
project's own gate on the card, comments that survive the file changing.
Owner: #300 verifies, #301 writes.

**R2. Source or drop M9's "roughly a hundred and fifty".** §2 gives the queries
and the dates. The qualitative claim stands without the number.

**R3. Treat "a fresh worktree you can actually run" as a first-class feature.**
Four independent requests across three competitors, two shipped
implementations, no omatty answer. See `issues-synthesis.md` §1. This is the
recommendation with the most external evidence behind it.

**R4. Put PR and CI state on the session card.** The field asks for the remote
verdict (Orca #18484/#18485/#18487); fleet ships it; omatty has only the local
one. The two are complements. This is verification, not orchestration, so it
clears "Not on the roadmap"'s anti-orchestrator line without argument —
unlike Orca #10131, which does not.

**R5. Give the review pane a notion of *since when*.** Orca #11840 asks for
"changes this turn" / "since my last review", backed by "a refs/orca baseline
snapped at agent turn boundaries" — which is a mechanism omatty already has the
parts for, since the `Stop` hook marks the boundary. omatty shows
everything a session changed. For a tool whose whole claim is time-to-judgement
this is a gap in the thesis itself, not a nice-to-have.

**R6. Carry the anchoring difference into the pitch, narrowly.** #296 verified
that Orca flags a stale note and omatty re-resolves it. That is one property,
it is real, and it is the kind of specific claim a reader can check. Do not
inflate it into "our review is better".

**R7. Amend #152 with fleet's Codex hooks, and plan for their incompleteness.**
`internal/hooks/codex_hooks.go` shows `PermissionRequest` is obtainable at
`~/.codex/hooks.json`; fleet #220 shows a partial hook set leaves a session
confidently wrong. Both belong on the issue. *(Done 2026-09-18 — the correction
is posted on #152.)*

**R8. Cite the field in the refusals rather than reasoning alone.** "Not on the
roadmap" argues every refusal from first principles. Three of them now have
live examples: `--autoyes` (claude-squad #222, #151, and ccmanager's public
objection to it), agent-to-agent messaging (`fleet skill install`), and
CI-triggered agent runs (Orca #10131). A refusal that names what it is refusing
is harder to re-open by accident.

**R9. Do not widen the surface.** ccmanager has four open issues at 1,246
stars because it does one thing; Orca has 3,036 at 71,802 because it does
everything. M12's output should be R3, R4 and R5 — all inside the thesis — and
should refuse the rest on the record.

## 6. Sources

All read 2026-09-18.

**Primary repositories** (default branch, cloned or read via the GitHub API):
`stablyai/orca`, `kbwo/ccmanager`, `smtg-ai/claude-squad`, `brizzai/fleet`,
`nimbalyst/nimbalyst`, `stravu/crystal`, `BloopAI/vibe-kanban`,
`dagger/container-use`, `imbue-ai/sculptor`, `ykdojo/safeclaw`,
`devflowinc/uzi`, `akitaonrails/ai-jail`, `jesseduffield/lazygit`,
`dandavison/delta`, `Wilfred/difftastic`.

**GitHub API** for every number in §2 and the field-size probes: `repos/{r}`,
`repos/{r}/releases/latest`, `search/issues`, `search/repositories`.

**Anthropic documentation** for §3.1: `code.claude.com/docs/en/overview` and
`code.claude.com/docs/en/agent-view`.

**Method** — the six-artifact shape, the evidence rules, and the separation of
research from decision — is adapted from `akitaonrails/ai-memory`'s
`docs/research-2026-landscape.md`, `docs/comparison.md`,
`docs/competitive-parity.md`, `docs/prior-art-implementation-findings.md` and
its `docs/issues-*.md` set, read 2026-09-18.

**Not used as sources**, and named so the next pass does not mistake them for
any: `abralo.com/alternatives`, `runpane.com/alternatives/claude-squad`,
`nimbalyst.com/blog/*`, `codeagentswarm.com/guides/*`, `munderdiffl.in/blog/*`.
Each is published by a vendor that appears in its own ranking.

## Reproducing the table

```bash
for r in stablyai/orca BloopAI/vibe-kanban smtg-ai/claude-squad ...; do
  j=$(gh api "repos/$r")
  rel=$(gh api "repos/$r/releases/latest" --jq '.tag_name+" ("+.published_at[0:10]+")"')
  oi=$(gh api "search/issues?q=repo:$r+is:issue+is:open&per_page=1" --jq .total_count)
  echo "$j" | jq -r --arg rel "$rel" --arg oi "$oi" \
    '[.full_name,(.stargazers_count|tostring),.pushed_at[0:10],$rel,$oi,
      (.license.spdx_id//"none")]|@tsv'
done
```

Field-size counts use
`gh api "search/repositories?q=<urlencoded>&per_page=1" --jq .total_count`.

Re-run both before quoting any number in this file; replace the capture date
in the header when you do, and say in the same commit what moved.

---

The per-project deep dives are #296, tracker mining is #297 and
`issues-synthesis.md`, and the audit of all of it against our own code is
#299. What omatty *does* about §5 is #301's, in `docs/ROADMAP.md`.
