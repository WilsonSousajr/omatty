# `smtg-ai/claude-squad` — issue and PR pain-point synthesis

> Source: GitHub `smtg-ai/claude-squad`, all 133 issues read 2026-09-18 via
> `gh issue list --repo smtg-ai/claude-squad --state all --limit 300`.
> Tracker character: heavily bug-weighted, and one mechanism produces most of
> the bugs. Roughly a fifth of everything filed is a feature request, and the
> open ones skew to features the maintainers have not taken.

## Top recurring pain points, ranked

### A. Capturing the tmux pane — the top bug generator

Ten of 133 issues concern capturing pane content or tmux state, and they
include the single most-discussed issue in the tracker.

- **#51 (18 comments)** "Error capturing pane content: exit status 1"
- **#216 (10)** "Error capturing pane content after starting cs"
- **#189 (4)** "TMUX error capturing pane content"
- **#132 (10)** "Bug: Failed to start new session after fresh install"
- **#96 (10, `prio:high`)** "Issues with creating a session"
- **#291** "Error on Win11 with psmux or Msys2 tmux"
- **#277** "new sessions do not inherit custom env vars when reusing an
  existing tmux server"
- **#325 (open)** "Bubble Tea's input reader races the tmux attach reader for
  stdin"

**The design choice behind the cluster:** status and liveness are read by
shelling out to tmux and matching text in the captured pane
(`session/tmux/tmux.go`, see `claude-squad.md`). Every failure of tmux — not
installed, wrong version, a foreign tmux server, Windows, a race on stdin —
surfaces as a status bug, because the status path and the multiplexing path
are the same path.

**Impossible in omatty, and by construction rather than by luck.** `internal/
termwrap` owns the PTY, so there is no pane to capture and nothing to race on
stdin; status comes from hooks and the transcript, so a multiplexer problem
cannot present as "this session's state is wrong". dtach is used for
persistence only, and #191 is the evidence that even that narrow use cost a
real bug.

### B. Multiple repositories — asked for repeatedly, still open

- **#56 (7 comments, OPEN)** "Enable multiple git repos with claude squad"
- **#299 (OPEN)** "start a new session in another repository without
  relaunching"
- **#238 (OPEN)** "Display repository name in session list view"

Three independent open requests for what omatty's sidebar has done since M1.
**#56 is the single most-commented open issue in the tracker.** This is the
strongest available external evidence that omatty's multi-repository bet is
answering a real need rather than one person's taste — and it arrives at the
same time as the finding that `README.md` is wrong to claim omatty is alone in
having it (ccmanager shipped it; see `issues-ccmanager.md` §B).

### C. Getting a new worktree into a working state

- **#260 (3, OPEN)** "Feature request: worktree environment setup hook (deps,
  env files, port isolation)"
- **#277 (OPEN)** custom env vars not inherited by new sessions

The same gap ccmanager closed with `.worktreeinclude` and fleet with
`.fleet.json` + `internal/git/exclude.go`. See `issues-synthesis.md` §1: this
is the field's most-repeated unmet need and omatty has no answer to it.

### D. Sending a prompt into an agent that is not ready

- **#266 (OPEN)** "Bug: SendPrompt sends text before CLI (Codex/others) is
  ready, prompt lost"

omatty's counterpart is invariant 8 and `internal/paste`: a prompt goes in as
one bracketed paste with no carriage return, and #190 was the bug that made
the rule explicit. Worth noting that omatty's rule was arrived at for a
different reason (a paste is a `PasteMsg`, not keystrokes) and happens to
cover this failure too.

### E. Platform and distribution

- **#275 (4, OPEN)** "Windows binary fails immediately on `n new` —
  `creack/pty` is unsupported on Windows"
- **#97** "Add to Homebrew" (closed — they did)
- **#245 (OPEN)** `CLAUDE_SQUAD_HOME` for a custom config directory

omatty is Unix-only too and has neither a Homebrew formula nor an installer,
so E is a mirror rather than a contrast. `$OMATTY_HOME` has a counterpart in
the scratch-`HOME` smoke recipe but not as a supported variable.

## Feature requests worth reading as roadmap signal

| Issue | Ask | omatty |
|---|---|---|
| #56, #299, #238 | multiple repositories, repo name on the row | **has it** since M1 |
| #312 (OPEN) | "Focus mode: type into a session directly from the list view" | **has it** — the pane *is* the session (M1 modal routing) |
| #60 | scrolling in the preview pane | partly: #191 left scrollback lost after a reattach |
| #260, #277 | worktree env/deps/gitignored files | **no answer** |
| #222, #151 | `-y` bypass-permissions mode | **refused** — "Not on the roadmap" |
| #62 | independent agent per session | `internal/agent` seam (#46), one entry so far |
| #154 | remappable keys | `~/.omatty/config.toml` (M7) |
| #300, #296 | themes, compact mode | M8's colour rule; compact is not on the roadmap |

## What the maintainers' fixes reveal

The pane-capture bugs are fixed one symptom at a time — a retry here, a
version check there — rather than by moving status off the screen, because
moving it off the screen means not using tmux, which is the project's
foundation. The architecture is load-bearing and the bug class is therefore
permanent. This is the clearest illustration in the pass of why invariant 3
is an invariant and not a preference.

## Do-not-repeat lessons for omatty

1. **Never let the transport and the status path be the same path.** When they
   are, every transport failure becomes a lie about what a session is doing.
2. **A `-y`-shaped feature will be asked for** (#222, #151) and ccmanager's
   README argues publicly against claude-squad's answer to it. The refusal in
   "Not on the roadmap" should cite both, so the next person to propose it
   reads the argument rather than re-opening it.
3. **The state of a fresh worktree is a product feature, not a user problem.**
   Three projects now treat it as one.
