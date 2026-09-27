# `herdrdev/herdr` and `persiyanov/herdr-reviewr`: deep dive

> Captured **2026-09-27** against `herdrdev/herdr` at `master` (Apache-2.0,
> Rust, 40,960 stars, v0.9.1 released 2026-09-16) and
> `persiyanov/herdr-reviewr` at `main` (MIT, Rust, 778 stars, v0.39.0
> released 2026-09-23). Every load-bearing claim was checked against a live
> page on that date, not remembered. **Read at code and documentation level**:
> herdr's own docs for the release in `docs/versions/0.9.1/website/`, its agent
> manifests, and reviewr's `src/`. Neither binary was run. Analysis only; no
> decision is made in this file (#548, R10 in `2026-landscape.md` §7.7).

herdr is the tool a reader of an omatty launch thread is most likely to name
(`go-to-market.md` §3, G6). It is terminal-native, owns the PTYs, runs the
real agent binaries, and with one plugin sends diff comments back to the
agent. Those are omatty's M1, M3 and M6 headlines. This file says where the
two actually differ, file by file, so `docs/comparison.md` can name herdr
without either overclaiming or conceding too much.

## 1. Purpose and scope

"The runtime your coding agents live on" (`README.md`). A terminal workspace
manager: workspaces, tabs and split panes, each a real terminal, with agent
state rolled up in a sidebar. It runs "claude code, codex, cursor, opencode,
grok and the rest. herdr doesn't wrap or replace them; it owns their
terminals." Agents can drive herdr itself through a CLI and socket API
("spawn panes, prompt each other, and wait until another agent is genuinely
blocked"). Plugins extend it, through a marketplace.

It is general-purpose where omatty is narrow. A pane can hold a shell, a
server or a test run as easily as an agent.

## 2. Session model

A **workspace** is "the top-level project container. Use one workspace per
repo, task, or investigation" (`concepts.mdx`). A workspace owns tabs, a tab
owns panes, and a pane is a terminal. An **agent** is "a process Herdr
recognizes inside a pane". A herdr **session** is a server namespace, not a
conversation (`concepts.mdx`, "Session").

Worktrees are in core: `src/app/worktrees.rs`, `src/cli/worktree.rs`,
`src/client/shell/worktrees.rs`. Several repositories in one window is the
default shape, one workspace each.

## 3. Process model

A background server owns the PTYs; the TUI is a client (`session-state.mdx`,
"Live persistence"). Terminal emulation is Ghostty's VT engine, vendored as
`crates/ghostty-vt`. Windows is "generally available" on ConPTY
(`windows-beta.mdx`).

## 4. Status and state: the screen, for Claude Code

This is the sharpest difference from omatty, and it is invisible from the
README, which says only that each pane is "marked working, blocked, or idle".

`agents.mdx` has a table titled "which signal determines `idle`, `working`,
and `blocked` for each one". For **Claude Code** the state authority is
**"screen manifest"**, and the integration's role is **"session"** only. The
same holds for Codex, Cursor Agent CLI, Copilot, Devin and eight others.
Lifecycle hooks are authoritative only for Pi, OMP, Kimi, OpenCode, Kilo and
MastraCode.

`integrations.mdx`, "Claude Code": "The hook reports Claude Code session
identity to the local Herdr socket on session start. Claude Code state comes
from Herdr's screen manifest detection."

The manifest is `distribution/agent-detection/claude.toml`: regular
expressions over regions of the rendered screen, versioned against Claude's
UI. For example, rule `osc_title_working` matches the spinner glyph in the
window title, with the comment "Braille covers <= 2.1.227; half-circles are
the 2.1.228 busy spinner"; rule `live_turn_working` matches
`esc to interrupt` in the bottom twelve non-empty lines; a `not` list refuses
`do you want to proceed?` and `waiting for permission`. The manifest carries
its own version (`2026.09.11.1`) because each Claude UI change can require a
new one.

That is the design omatty's invariant 2 forbids, for the reason
`issues-claude-squad.md` and `issues-ccmanager.md` document: status read from
the screen breaks each time the agent redraws it. herdr has invested heavily
in doing it well (a rules engine, priorities, regions, negative matches), but
it is the same class of source.

## 5. Footprint in `~/.claude`

`integrations.mdx`, "Claude Code": "Install writes `hooks/herdr-agent-state.sh`
and updates `settings.json` with Herdr hook entries. Uninstall removes the
matching hook entries and deletes the hook script." The Codex, Copilot, Devin
and Kimi integrations do the same to their agents' configuration.

The integration is optional (detection works without it), but native session
restore needs it (§7). omatty's invariant 3 is the opposite choice: hooks
passed per session with `--settings`, nothing written to the user's settings.

## 6. Review surface: herdr-reviewr

herdr's core has no diff view. `persiyanov/herdr-reviewr` is a plugin pane:
diff review over four scopes (uncommitted, branch, last turn, commits), line
and range comments, a file viewer, search, and a read-only PR tab for GitHub,
GitLab and Azure DevOps (`README.md`). `s` sends every comment to the agent.

**Anchoring is by line number.** `src/model.rs`:

```rust
/// A reviewer comment anchored to a run of diff lines, carrying the snippet.
pub struct Comment {
    pub file: String,
    pub side: Side,
    pub start: u32,
    pub end: u32,
    /// Verbatim diff lines the comment anchors to, each keeping its `+`/`-`/space marker.
    pub lines: String,
    ...
```

and `src/app.rs`, `fn line_in`, which decides where a comment is drawn:
`no.is_some_and(|n| c.start <= n && n <= c.end)`. `fn is_stale` marks a diff
comment stale only "once its file leaves the changeset". The README says it
plainly under "Review model": "**No line-number rebasing**: a comment stays
locatable by its diff snippet, not its line number. reviewr flags a stale
comment instead of dropping it."

So when the agent inserts lines above a comment while you are reading, the
comment is drawn on whatever line now has its number, and is not flagged,
because the file is still in the changeset. The snippet travels with the
comment when it is sent, so the agent receives the right text; the reviewer's
view is what drifts. This is the failure M12 found in Orca (`orca.md`,
`lineNumber`) and the one omatty's invariant 7 exists for
(`internal/review/anchor.go`: file, hunk header, line hash, occurrence).

Other review-model limits, from the same README section: comments are
"in-memory and single-session" (closing the pane loses unsent ones), and
sending is all-or-nothing. "last turn relies on polling (2 s default)", so a
turn inside one poll is missed.

reviewr runs on macOS and Linux only, and needs truecolor.

## 7. Persistence across restart

`session-state.mdx`, "What survives":

- **Detach and reattach**: processes keep running. The strongest path.
- **Server restart**: processes do not survive. The layout returns; a Claude
  pane is restarted with `claude --resume <id>` if integration version 6 or
  newer reported its session id ("native agent session restore", on by
  default). Otherwise the pane "come[s] back as new shells".
- **Update with `--handoff`**: experimental live transfer of PTYs to a new
  server.

This is omatty's M6 shape (dtach holds the process; `--resume <uuid>` is crash
recovery), reached independently. herdr is ahead on handoff across upgrades
and on screen-history replay (`[experimental] pane_history`).

## 8. Gate / CI integration

None in core. `2026-landscape.md` §7.4 found the tree holds only herdr's own
development scripts (`scripts/*_check.py`). Two plugins do part of it:
`jpolec/herdr-plugin-odysseus` (named checks inside an autonomous
worktree-to-draft-PR pipeline, judged by exit status) and
`shindakun/herdr-testrun` (runs tests in a pane and parses the runner's
output). reviewr's PR tab shows the forge's checks, read-only.

## 9. Multi-repository and remote

Several repositories: yes, one workspace each (§2). Remote: "several machines,
one window: keep local work and saved ssh machines together, with a combined
agent list and independent reconnects" (`README.md`). herdr is ahead of omatty here: omatty runs in the
terminal you SSH into, but does not aggregate machines.

## 10. Deployment

`curl -fsSL https://herdr.dev/install.sh | sh`, `brew install herdr` (a core
formula), `mise`, PowerShell on Windows, release binaries, and a Nix flake
(`flake.nix`). One Rust binary. Documentation in three languages.

## 11. Strengths worth borrowing

- **Live handoff on upgrade** (`herdr update --handoff`): an upgrade that does
  not end the sessions. omatty's dtach holder survives the app, but a new
  omatty binary still has to reattach.
- **A combined agent list across machines.**
- **Distribution breadth**: a core Homebrew formula, a `curl | sh` installer,
  Nix, Windows. `competitive-parity.md` already lists the installer gap.

## 12. Weaknesses to avoid

- **Status from the screen for Claude Code**, with a manifest versioned
  against Claude's UI (§4). invariant 2.
- **Writing to `~/.claude/settings.json`** to get session identity (§5).
  invariant 3.
- **Comments anchored by line number** in the review plugin (§6).
  invariant 7.

## 13. Bottom line

herdr is the best-built tool in omatty's camp and ahead of it on breadth:
more agents, Windows, several machines, handoff, distribution, forty thousand
users. With herdr-reviewr it has a review loop that sends comments back. A
reader who says "herdr plus a plugin does this" is mostly right about
omatty's M1-M6 and M3 headlines.

What does not transfer, each with a file behind it:

| | herdr (+ reviewr) | omatty |
|---|---|---|
| Claude Code status | screen manifest, regex over the rendered UI (`agents.mdx`; `claude.toml`) | hooks and transcript, never the screen (invariant 2) |
| `~/.claude/settings.json` | written by `herdr integration install claude` (`integrations.mdx`) | never written (invariant 3) |
| Review comment anchor | line number; "No line-number rebasing" (`src/model.rs`, `fn line_in`, reviewr `README.md`) | file, hunk, line hash (invariant 7) |
| The project's own check line, per session, verdict on the card | not in core; partial plugins (§8) | `internal/gate` |

## Sources

All opened 2026-09-27.

- `herdrdev/herdr`: `README.md`;
  `docs/versions/0.9.1/website/src/content/docs/{concepts,agents,integrations,session-state,windows-beta}.mdx`;
  `distribution/agent-detection/claude.toml`; the file tree via
  `gh api repos/herdrdev/herdr/git/trees/HEAD?recursive=1`.
- `persiyanov/herdr-reviewr`: `README.md` ("The three tabs", "Diff scopes",
  "Limitations"); `src/model.rs` (`Comment`); `src/app.rs` (`line_in`,
  `is_stale`, `card_rows`).
- Stars, licences and releases: `gh api repos/<r>` and
  `gh api repos/<r>/releases/latest`.
