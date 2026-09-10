# Changelog

Notable changes to omatty. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[semantic versioning](https://semver.org/spec/v2.0.0.html), with the pre-1.0
caveat that the key table, `~/.omatty/config.toml` keys and the `state.json`
schema are not yet frozen.

Each entry names the issues behind it. `docs/ROADMAP.md` carries the reasoning
for each milestone and what was deliberately cut.

## [v0.1.0] — 2026-09-10

First release. Eight milestones, built on `develop` between 2026-09-01 and
2026-09-10, promoted to `main` together (#134).

### Added

- **M1 — Skeleton.** Projects and sessions in `~/.omatty/state.json`,
  worktrees created on demand, the real `claude` binary running inside an
  embedded PTY pane, and modal key routing where every keystroke reaches
  Claude except the `ctrl+o` leader. (#1–#13)
- **M2 — Status.** Per-session glyphs, age and cumulative token usage in the
  sidebar, derived from Claude Code hooks over a unix socket and from each
  session's transcript JSONL — never scraped from the rendered screen.
  Desktop notifications when a session starts waiting on you. (#17–#20)
- **M3 — Review.** `ctrl+o d` opens a diff of everything a session changed —
  commits since it branched, uncommitted edits and new files in one view.
  Comments anchor to line *content*, not line numbers, so they survive Claude
  editing the file underneath you, and `S` sends the whole batch as a single
  bracketed-paste prompt. (#21–#24)
- **M4 — Lifecycle.** Rename (`ctrl+o R`), archive (`ctrl+o x`), jump by name
  (`ctrl+o /`), and project discovery from the repositories claude already
  knows you use (`ctrl+o a`). (#91, #40–#42)
- **M5 — File tree.** `ctrl+o f` shows the session's worktree in the same
  column as the diff, with per-file change markers in the diff's own colours,
  a filter, `@path` attach, and syntax-highlighted previews kept in colours
  distinct from the diff's. (#194–#200)
- **M6 — Persistence.** With `dtach` installed, quitting detaches from
  sessions rather than ending them and relaunching reattaches, so a turn in
  flight survives. Optional: without the binary omatty behaves as before and
  says so once. Sessions claude already holds can be adopted. (#43, #122)
- **M7 — Reach.** `~/.omatty/config.toml`, mouse support, the agent seam in
  `internal/agent`, automatic session naming, and a visual identity — activity
  lanes, a cache-hit token meter and denser panel chrome. (#44–#46, #127,
  #128, #130, #133, #153–#155)
- **M8 — Surface.** The frame, colour rule, session cards, header, footer and
  sidebar diffstat the panes are drawn in. (#174–#180)
- `omatty --version` reports the release the binary was built from, read from
  the linker's `-X main.version` or, failing that, the module version the go
  tool stamps. (#134)
- `docs/ARCHITECTURE.md` — data flow, the package table, the eleven invariants
  each with the failure behind it, and the four seams. (#156)

### Fixed

Bugs found by running the merged result, each with a regression test named
after its issue:

- Claude's window title drawn into the pane: `x/ansi` read the byte `0x9C` as
  a string terminator inside an OSC payload even in UTF-8, and every Dingbat
  is `E2 9C xx`. (#192)
- Every pane blank after a restart — dtach clears on attach and re-signals at
  the same size, which the kernel does not deliver. (#191)
- Paste never reached claude: a paste is a `PasteMsg`, not keystrokes, and
  nothing routed it. (#190)
- A copy made inside a pane now reaches the host clipboard: omatty lifts the
  `OSC 52` the program writes and hands it to your terminal. (#212)
- The review column ignored the mouse and was indistinguishable from the diff
  claude draws in its own pane. (#168)
- A project with no sessions could not be selected, making discovery unusable.
  (#158)
- A registered project could never be removed. (#159)
- Token counts read as near-zero on a well-cached session: claude reports
  `input_tokens` as the uncached remainder alone. (#170)

### Known limitations

- Dragging to select text scrolls instead, because omatty asks the terminal
  for the mouse. Hold `shift` (Ghostty, kitty, xterm, Alacritty) or `option`
  (Apple Terminal, iTerm2) and drag. (#217)
- `ctrl+o N` still asks for a worktree branch name. (#151)
- The agent seam has one profile, claude. Codex is a follow-up. (#152)
- Scrollback is not preserved across a detach and reattach.

[v0.1.0]: https://github.com/WilsonSousajr/omatty/releases/tag/v0.1.0
