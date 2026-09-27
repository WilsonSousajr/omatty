# The field omatty ships into, September 2026 — inventory

> Captured **2026-09-18**, against `develop` at v0.1.0 plus M9, M10 and M11.
> Every load-bearing claim below was checked against a live primary page on
> that date, not remembered. Repository numbers come from the GitHub API
> (`gh api repos/{owner}/{repo}`, `/releases/latest`, `search/issues`), and
> the script that produces them is in "Reproducing the table" so the next pass
> re-runs rather than re-invents it.
>
> **Refreshed 2026-09-27 in §7** (#513). §1-6 are the 09-18 capture, unedited;
> where §7 contradicts them, §7 is current.
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

## 7. Refresh, 2026-09-27

> Captured **2026-09-27**, against `develop` at v0.7.0 plus #505, nine days
> after §1-6. Every load-bearing claim in this section was checked against a
> live primary page on that date, not remembered. §1-6 are left as they were
> written, so the two captures can be compared; where this section contradicts
> them, this section is the current one. Analysis only, like the rest of the
> file: the recommendations below are inputs to `docs/ROADMAP.md` and #330.
>
> **Coverage.** The signal table and the field-size probes were re-run with the
> script under "Reproducing the table". New candidates came from `gh search
> repos` by topic and keyword, and from `andyrewlee/awesome-agent-orchestrators`
> used as a list of names only. Gate claims were checked by fetching each
> repository's file tree (`git/trees/HEAD?recursive=1`), grepping paths for
> check, verify, gate, lint and test, and opening the files that matched. That
> is a probe, not a read: a gate under an unexpected name would be missed.

### 7.1 The signal table moved, in one place a lot

| Project | Stars 09-18 → 09-27 | Last push | Latest release | Open issues |
|---|---:|---|---|---:|
| `stablyai/orca` | 71,802 → **79,236** | 09-27 | v1.4.215 (09-27) | 3,301 |
| `BloopAI/vibe-kanban` | 28,123 → 28,201 | **09-19** | v0.1.44 (04-24) | 387 |
| `smtg-ai/claude-squad` | 8,495 → 8,537 | 08-20 | v1.0.20 (08-20) | 16 |
| `dagger/container-use` | 4,047 → 4,046 | 09-21 | v0.4.2 (2025-08-19) | 47 |
| `stravu/crystal` | 3,118 → 3,122 | 2026-02-26 | v0.3.5 (02-26) | 60 |
| `nimbalyst/nimbalyst` | 1,737 → 1,785 | 09-24 | v0.78.5 (09-24) | 595 |
| `kbwo/ccmanager` | 1,246 → 1,250 | 09-27 | v4.4.4 (09-27) | 4 |
| `akitaonrails/ai-jail` | 1,229 → 1,300 | 09-24 | v2.2.0 (09-24) | 0 |
| `devflowinc/uzi` | 583 → 583 | 2025-06-04 | v0.0.2 | 6 |
| `imbue-ai/sculptor` | 233 → 232 | 09-26 | v0.48.0 (09-21) | 0 |
| `ykdojo/safeclaw` | 184 → 184 | 09-18 | v0.7.0 (07-24) | 0 |
| `brizzai/fleet` | 53 → 54 | 09-23 | v2.45.0 (09-23) | 13 |
| `jesseduffield/lazygit` | 82,458 → 82,707 | 09-27 | v0.65.1 (09-13) | 864 |
| `dandavison/delta` | 32,227 → 32,355 | 09-19 | 0.19.2 (03-28) | 312 |
| `Wilfred/difftastic` | 25,915 → 25,938 | 09-22 | 0.71.0 (09-18) | 247 |
| `WilsonSousajr/omatty` | 0 → 1 | 09-26 | v0.7.0 (09-26) | 25 |

- Orca added 7,434 stars in nine days, most of claude-squad's lifetime total,
  and published nine releases in the window.
- **vibe-kanban is winding down, not busy.** §2 read its daily commits as
  activity. Its README now opens "Vibe Kanban is sunsetting"; the banner
  went in on 2026-04-24, the date of its last full release, and the last push
  is 09-19. It was already true on 09-18 and §2 missed it.
- claude-squad is five weeks without a commit.

Field-size probes: `topic:parallel-agents` 184 → 200;
`claude code parallel in:description stars:>10` 205 → 205;
`coding agent orchestrator in:description stars:>10` 462 → 472;
`multiple claude code sessions in:description` 446 → 451;
`claude code worktree in:description` 1,300 → 1,370.

### 7.2 What §1 missed: the largest terminal sibling, and eight more

This is the finding of the refresh, and it is a correction, not news. Every
project below existed before 2026-09-18, overlaps camp A or B, and appears
nowhere in `docs/` or `README.md`. Several are larger than anything §1
listed in camp A.

| Project | Camp | Stars | Created | Latest release | Licence |
|---|---|---:|---|---|---|
| `herdrdev/herdr` | A | 40,951 | 2026-03-27 | v0.9.1 (09-16) | Apache-2.0 |
| `manaflow-ai/cmux` | B | 27,431 | 2026-01-28 | v0.64.25 (09-17) | GPL (README) |
| `getpaseo/paseo` | B | 18,674 | 2025-10-13 | v0.9.2 (09-24) | none declared |
| `NanmiCoder/cc-haha` | B | 14,730 | 2026-03-31 | v0.6.6 (09-22) | MIT |
| `superset-sh/superset` | B | 14,670 | 2025-10-21 | desktop-v1.30.2 (09-22) | none declared |
| `Untrivial-ai/agent-orchestrator` | B/C | 12,412 | 2026-02-13 | v0.13.1 (09-26) | Apache-2.0 |
| `generalaction/emdash` | B | 5,850 | 2025-08-28 | v1.2.7 (09-27) | Apache-2.0 |
| `xintaofei/codeg` | B | 3,703 | 2026-02-09 | v0.32.2 (09-24) | Apache-2.0 |
| `greenfield-inc/Pane` | A/B | 492 | 2026-02-27 | v2.4.133 (09-27) | none declared |

Why they were missed matters more than that they were: §1 found candidates
by searching for *Claude Code* and *parallel*, and the largest of these
describe themselves without either word ("the runtime your coding agents
live on", "a Ghostty-based macOS terminal"). The next pass should search by
shape (PTY, terminal, worktree, agents) as well as by name.

**`herdrdev/herdr` is omatty's nearest large sibling**, and the one that
changes the most. From its README: a single Rust binary, "no electron"; it
"owns their terminals" rather than wrapping the agents; "detach without
stopping work" and "can resume supported agent sessions"; "several machines,
one window" over SSH; each pane "marked working, blocked, or idle"; a
Homebrew formula; a plugin marketplace. Its plugin
`persiyanov/herdr-reviewr` (778 stars, v0.39.0 on 09-23) is "a code review +
file viewer sidebar for herdr. Comment on a diff and send back to agent",
with a last-turn diff scope and a read-only PR tab. Taken together, herdr
plus one plugin covers most of omatty's M1-M6 and M3 headline in a terminal,
at forty thousand stars. What its core does not have is a gate (the tree
holds only its own dev scripts, `scripts/*_check.py`); see 7.4 for its
plugins.

The others, one line each, from their READMEs:

- **cmux**: a macOS-only terminal; the sidebar shows branch, PR number and
  status, cwd and listening ports. No gate found.
- **paseo**: a daemon with desktop, mobile, web and CLI clients. Its checks
  are a change request's CI:
  `packages/app/src/components/sidebar/workspace-meta-row/check-summary.ts`.
- **cc-haha**: a desktop Claude Code workspace that runs its own
  `./bin/claude-haha`, not the stock binary. Gate not checked at code level.
- **superset**: a worktree per task, a diff viewer whose selected lines go
  "to a running or new agent session". Its checks are GitHub's:
  `computeChecksStatus.ts` ("GitHub's status × conclusion grid").
- **agent-orchestrator**: a Go daemon and a desktop Kanban whose cards move
  "from session, pull request, CI, and review facts", and which can "send CI
  and review feedback back to the same agent". Remote CI only.
- **emdash** (YC W26): a worktree per task, SSH remotes, "inspect CI checks,
  and merge". It "installs marker-tagged entries in the agent's user-level
  config" for its hooks, which is the thing invariant 3 exists to refuse.
- **codeg**: aggregates sessions from fifteen agent CLIs; a to-do board with
  worktrees. No gate found.
- **Pane**: "Vim for agent management". It is the vendor behind the
  `runpane.com` "alternatives" pages this file bans as sources; its own
  repository is a primary source for Pane itself. Its nearest thing to a gate
  is a bundled agent skill, `skills/quick-verify/`, which asks the agent to
  verify; the tool computes no verdict.

### 7.3 New since 09-01, small and close in shape

| Project | Stars | Created | Lang | What it is |
|---|---:|---|---|---|
| `nccapo/stvena` | 10 | 09-08 | Go | "A live review workspace for Codex and Claude Code"; selected lines go back to the agent; runs a check (7.4). |
| `xseman/pando` | 5 | 09-18 | Go | "one daemon owns PTY sessions in git worktrees, the TUI and CLI are clients over a unix socket"; several projects; no check runner in its 127 files. |
| `axonel/axonel` | 52 | 09-12 | Rust | Missions in worktrees, "independently verify their changes", human approval before integration. Linux x86_64 only. |

`r2luna/floe`, `kerim0x1/bettercode`, `emircan-sahin/gitviber`,
`TennnisAI/Agency`, `icesword0760/matou` and `YuvalSarel1/cones` were also
created in September, are under 100 stars, and have no gate.

### 7.4 The gate: still empty among the session managers, no longer empty

§3.2 said the gate was "nobody else's feature". As a literal claim that is
now false. Four small tools each do part of it, and each decides pass or fail
by exit status, as invariant 12 does:

- **stvena**, `internal/checks/checks.go`: checks a captured tree out into a
  temporary directory, runs one user command under `sh -c`, and sets
  `Status = "Passed"` if and only if it exits 0. One command per keypress; no
  multi-step line, no pending or missing state.
- **axonel**, `crates/plexis-runtime/src/verifier.rs`: runs one verification
  command per mission and compares the exit code with an expected one;
  `VerificationVerdict {Passed, Failed, Inconclusive}`. A tool that fails to
  spawn is `Failed`: there is no missing state.
- **`jpolec/herdr-plugin-odysseus`** (4 stars), `src/checks/mod.rs` and
  `src/engine/steps.rs`: named `tests`, `lint` and `security` checks, taken
  from config or detected from `Cargo.toml`, `package.json`, `go.mod` or
  `pyproject.toml`, judged by exit status. The closest thing to omatty's
  gate, but it is one step inside an autonomous worktree-to-draft-PR
  pipeline, not a verdict on a session you are watching.
- **`shindakun/herdr-testrun`** (1 star): runs a project's tests in a pane and
  sends the failures to the agent on one key. Its results come from parsing
  runner output (`go test -json`'s `Action=fail`), which is the opposite of
  invariant 12.

What still holds, and is the defensible form of the claim: **no session
manager or workspace above about a hundred stars runs the project's own
multi-step check line per session and puts per-step verdicts on the
session's card.** Every larger tool's answer is the *remote* verdict, PR and
CI state from the forge (Orca, superset, paseo, agent-orchestrator, emdash,
cmux, and now agent view; see 7.5). Orca, vibe-kanban, claude-squad,
ccmanager, fleet, Nimbalyst, Sculptor and herdr's core were each checked and
have no local gate (file evidence in the refresh notes behind #513).

The square is still empty among the tools people use. It is no longer empty
on GitHub, all four entrants appeared between 09-08 and 09-23, and two of them
are plugins for the largest terminal sibling. That is an expiry date on the
claim, not a refutation of it.

### 7.5 First party

Claude Code 2.1.276 to 2.1.283 (09-18 to 09-25, `anthropics/claude-code`
`CHANGELOG.md`) shipped nothing like a gate. Relevant lines:

- 2.1.277: "in a project with no CLAUDE.md, Claude Code reads AGENTS.md
  instead".
- 2.1.281: `--setting-sources` forwarded to `/bg`, `claude agents` and
  `--worktree --tmux`; `/batch` works with WorktreeCreate-hook worktrees.
- 2.1.282: fixed pasted multi-line text submitting line by line after
  bracketed paste mode was reset. That is the failure invariant 8 guards
  against, on claude's side of the pipe.
- 2.1.283: `/tasks` status icons; `/ultrareview` warns that it may upload
  uncommitted changes.

**§3.1 is wrong about agent view, and was probably wrong on 09-18.** Its
documentation (`code.claude.com/docs/en/agent-view`, read today) says "the
list shows every background session you've started, across all your
projects … regardless of which directory you opened agent view from", and
colours each session's pull request number by its checks status (yellow for
failed, green for passed). No changelog entry in the window announces either,
so §3.1's "scoped to one working directory" was more likely a misreading than
a change. Two of §3.1's three "still has" points are therefore gone:
several repositories is first-party, and so is the remote verdict on the row.
One remains: agent view does not run the project's own check line. Its
sessions are also background sessions you attach to one at a time, not
several live panes side by side.

### 7.6 What §5 became

- R3 (a fresh worktree you can run) shipped as #309, `omatty carry`.
- R4 (PR and CI state on the card) shipped as #310.
- R5 (review since the last turn) shipped as #311.
- R1 (rewrite the README's opening) was done, and 7.2 and 7.5 have already
  outdated it; see R11.

### 7.7 What this means for omatty

Numbered on from §5. Inputs, not decisions.

**R10. Deep-dive herdr and herdr-reviewr before any launch post.** It is the
tool a reader of a Show HN thread is most likely to name, and the only one
that is terminal-native, owns the PTYs and sends diff comments back. A
`docs/research/herdr.md` on the §3 skeleton (session model, status source,
anchoring of review comments, what "resume supported agent sessions"
means) is what `comparison.md` needs before it can name it fairly.

**R11. Stop leading with "several repositories" and "works over SSH".**
Agent view spans all projects; herdr, cmux and emdash reach remote machines;
Orca runs headless with `orca serve`. Those are no longer distinctions a
reader will grant. The claims that survived this refresh are narrower: the
real `claude` binary in several live panes at once, review comments anchored
on content, and the gate. The #513 fact-check lists each sentence in
`README.md`, `comparison.md` and omatty.com that needs to change.

**R12. State the gate claim in its defensible form, with its date.** "No
session manager runs your own check line on every session's card" survives;
"nobody else does this" does not, and a stranger can falsify it with
stvena's README. A dated, narrow claim is also the one that ages visibly.

**R13. Re-run this refresh at a month, not at a milestone.** Four partial
gates appeared in fifteen days, and Orca gains most of a claude-squad every nine days.
§7.4's claim has the shortest shelf life of anything in this file.

**R14. Search by shape, not by name.** Add `terminal agents`, `pty agents`,
`agent runtime` and `worktree tui` to the candidate queries, and treat an
awesome-list as a list of names to check, never as a source.

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
