# Codex (`codex`) — M17 spike

> Captured **2026-10-04** against **`codex-cli 0.160.0`** (Homebrew cask,
> darwin arm64), read at source tag `rust-v0.160.0` (`a956835`) and driven in
> a real 120x40 PTY. Spike for #528; the profile is #152. Every sample below is
> synthetic or sanitised: paths are rewritten, prompts are the spike's own.

**Verdict: Full tier, with zero footprint.** Codex reports its own id from a
startup hook, takes hooks *and their trust* per invocation through `-c`,
reports waiting, resumes by id, and marks every turn's end. Two traps decide
whether the profile works: the hook-trust hash, and a background sub-session
that shares the pane's hooks.

## 1. Identity: `Reported`

Codex takes no id from omatty. There is no `--session-id`, no config key and
no environment variable that sets one; `ThreadId` is minted inside
(`codex-rs/core`) as a UUIDv7.

It reports its own. The `SessionStart` hook's payload carries `session_id` and
`transcript_path`, and the id is the `session_meta.payload.id` of the
transcript it names:

```json
{"session_id": "01a10652-4044-7521-927f-fe1fdaed5e18",
 "transcript_path": "~/.codex/sessions/2026/10/04/rollout-2026-10-04T06-50-16-01a10652-4044-7521-927f-fe1fdaed5e18.jsonl",
 "cwd": "/home/dev/project", "hook_event_name": "SessionStart",
 "model": "gpt-5.5", "permission_mode": "default", "source": "startup"}
```

- **The session is lazy.** Neither the transcript nor `SessionStart` exists at
  launch: both appear on the **first prompt**. A Codex pane nobody has typed
  into is unbound, and that is correct, because there is nothing to resume.
- **`/clear` re-binds**, exactly as claude does for #316: `SessionStart` fires
  with `"source": "clear"` and a new `session_id`, and the old transcript stops
  growing.
- **`codex resume <id>` keeps the id**: `SessionStart` fires with
  `"source": "resume"` and the same `session_id`, appending to the same file.
- **`OMATTY_SESSION` reaches the hook.** Every hook process inherits the
  environment of *its pane's* `codex`: two concurrent panes with
  `OMATTY_SESSION=pane-A` and `pane-B` each saw their own value. (Both ran
  with `-c`, so each ran its own embedded app-server; see §7.) #523's Reported path finds the
  pane, but it needs one filter: a background thread fires the same hooks
  from the same pane (§2, "the memories sub-session"), and its `SessionStart`
  must not re-bind the row.

Scanning would also work (the filename carries the id, and `session_meta`
carries `cwd`), but it is not needed.

## 2. Hooks without a footprint: yes, through `-c`

Codex has claude-shaped hooks (`hooks` feature, **stable** in 0.160.0). They
are read from every config layer, including the **session-flags layer** that
`-c key=value` builds, so they can be supplied per invocation:

```sh
codex -c 'hooks.Stop=[{hooks=[{type="command",command="omatty hook --agent codex",timeout=5}]}]'
```

**This is not the seam #522 built.** #522 hands a profile a settings *file*
(`~/.omatty/hooks-<agent>.json`, through `Command`'s `settingsFile`). Codex
reads no such file without a write into `$CODEX_HOME` (`-p` layers
`$CODEX_HOME/<name>.config.toml`), so its hooks must travel as argv. #152 has
to give a profile a way to render hooks as arguments rather than as a file.
A profile that only wrote `hooks-codex.json` would fire nothing and fall to
the transcript tier, losing `Waiting`.

**A hook runs through the user's login shell:** `$SHELL -lc <command>`
(`hooks/src/engine/command_runner.rs`, `build_command`). So the line
`hooks.HookCommand` already renders, `'<omatty>' hook --agent 'codex'
2>/dev/null || true`, keeps its quoting, its silence and its exit 0
(invariant 11). Verified with that exact line: all five events fired, no
review screen. The cost is that the user's login profile runs on every hook.
Under the 5 s timeout that is fine for a normal profile, and it is Codex's
choice, not one omatty can change.

Events: `SessionStart`, `SessionEnd`, `UserPromptSubmit`, `PreToolUse`,
`PermissionRequest`, `PostToolUse`, `PreCompact`, `PostCompact`,
`SubagentStart`, `SubagentStop`, `Stop`, `Interrupt`.

### The trust gate, and how to pass it without a write

A hook Codex has not seen before **does not run**. Codex stops on a "Hooks
need review" screen ("Trust all and continue" / "Continue without trusting
(hooks won't run)"), and trusting writes this into `~/.codex/config.toml`:

```toml
[hooks.state."/<session-flags>/config.toml:session_start:0:0"]
trusted_hash = "sha256:5b89…"
```

That write is the user's config, so it is not a route (invariant 3). But the
trust record is read from the session-flags layer too
(`hooks/src/config_rules.rs`, `hook_states_from_stack` merges `User` and
`SessionFlags`), so **omatty can pass the trust alongside the hooks**:

```sh
-c 'hooks.state={"/<session-flags>/config.toml:stop:0:0"={trusted_hash="sha256:<hex>"}, …}'
```

Verified twice in a scratch `CODEX_HOME` with no stored hook trust:
`SessionStart`, `UserPromptSubmit`, `Stop` and `PermissionRequest` in one
run, then all five including `Interrupt` (hashed at its clamped 3 s) with
omatty's exact hook line. Each time there was no review screen, every hook
fired, and that home's `config.toml` was byte-identical before and after the
run. (§7's first-run write predates these runs and happened in another
home.)

**`-c` keeps codex off the shared daemon.** Corrected by #152's smoke test,
which read codex's own startup warning: "Running without the shared
background server: command-line configuration overrides (-c, --enable,
--disable, or --search) requires embedded mode." An earlier version of this
paragraph said the `-c` hooks reached an already-running daemon. The daemon
seen then had been started by a run *without* `-c`, and the hooked runs never
used it. So a codex omatty starts always runs its own embedded app-server,
and its hooks cannot be lost to a daemon started elsewhere. The cost is the
warning (codex shows "⚠ 1 warning" in its footer on every start) and §7's
"Run in background".

The key is `/<session-flags>/config.toml:<event_label>:<group>:<handler>`,
with the event in snake_case. The hash is `version_for_toml` over the
**normalised** hook identity (`hooks/src/engine/discovery.rs`, `hook_hash`):
SHA-256 of the canonical (sorted-key, compact) JSON of

```json
{"event_name":"stop","hooks":[{"async":false,"command":"<cmd>","timeout":5,"type":"command"}]}
```

Reproduced in the spike byte for byte. Two details cost a run each:

- **The timeout is normalised before hashing.** It defaults to 600 when
  omitted. `Interrupt` and `SessionEnd` are clamped to `[1, 3]` s, default 1.
  A 5 s `Interrupt` hook is stored as 3 s, hashes as 3 s, and with a 5 s hash
  lands back on the review screen as "Modified since last trusted". The
  profile must declare what Codex will normalise to.
- **The hash covers the command string**, so the command omatty passes must be
  the one it hashes. A change to `omatty hook`'s argv re-gates every hook. That
  is the point of the gate, and it costs nothing here, because the hash is
  computed at launch from the same value.

`--dangerously-bypass-hook-trust` also works, but it runs **every** enabled
untrusted hook, including the user's own. It is the wrong tool.

**Robustness.** The hash is an implementation detail, not a documented
interface. If a release changes it, the hooks fall back to the review screen:
loud, not silent, and the transcript tier still works. The profile should be
pinned by a test against recorded `codex` output, and agentprobe (#545)
re-checks it per release. Asking upstream for a documented per-invocation
trust route (for example "hooks given on the command line are trusted for
this invocation") is worth filing; see §8.

### Hook payloads (sanitised)

```json
{"session_id":"…","turn_id":"…","transcript_path":"…","cwd":"/home/dev/project",
 "hook_event_name":"UserPromptSubmit","model":"gpt-5.5","permission_mode":"default","prompt":"…"}
{"session_id":"…","turn_id":"…","transcript_path":"…","cwd":"/home/dev/project",
 "hook_event_name":"PermissionRequest","model":"gpt-5.5","permission_mode":"default",
 "tool_name":"Bash","tool_input":{"command":"curl -sI https://example.com | head -1","description":"…"}}
{"session_id":"…","turn_id":"…","transcript_path":"…","cwd":"/home/dev/project",
 "hook_event_name":"Stop","model":"gpt-5.5","permission_mode":"default",
 "stop_hook_active":false,"last_assistant_message":"PONG"}
```

The field names are claude's (`session_id`, `hook_event_name`,
`transcript_path`, `tool_name`). `Interrupt` fires on Esc and carries the same
envelope; `Stop` does **not** fire for an interrupted turn.

### Trap: the memories sub-session shares the pane's hooks

With the user's `[features] memories = true`, Codex starts a background
"Memory Writing Agent" thread in the same process. It fires `SessionStart` and
`UserPromptSubmit` through the **same hooks, with the same `OMATTY_SESSION`**,
but with `cwd` = `~/.codex/memories`, `transcript_path: null`, a different
`session_id` and a different model. A naive Reported binding would bind the
pane to that thread.

The seam as built cannot tell the two apart: `status.HookPayload` carries
neither `transcript_path` nor `cwd`, and `Adapter.KindOf` keeps no state. The
cheapest correct filter is in the hook process itself. Codex's `ParseHook`
treats `transcript_path` as a required routable field. `hooks.ParsePayload`'s
scanner already refuses a routable field that is not a string, so a `null`
drops the payload before it reaches the socket. The real session always
carries a path (every hooked payload in the spike did); the memories thread
never does. What is left is what #523 already covers: the row binds on its
first `SessionStart` with source `startup`, and `clear` re-binds it.

## 3. Transcript

`~/.codex/sessions/<YYYY>/<MM>/<DD>/rollout-<local ISO8601>-<uuid>.jsonl`
(or under `$CODEX_HOME`). #152's half-spike was right. The date directory and
timestamp cannot be predicted from the id, so the path is the "not yet"
locator #523 already has; `SessionStart` hands it over anyway.

Each line is `{"timestamp", "type", "payload"}`. The records that matter,
from one session's file, in order, with the records status ignores left out:

```json
{"timestamp":"2026-10-04T09:50:23.332Z","type":"session_meta","payload":{"id":"01a10652-4044-7521-927f-fe1fdaed5e18","timestamp":"2026-10-04T09:50:16.181Z","cwd":"/home/dev/project","originator":"codex-tui","cli_version":"0.160.0","source":"cli"}}
{"timestamp":"2026-10-04T09:50:23.333Z","type":"event_msg","payload":{"type":"task_started","turn_id":"01a10652-5c3a-…","started_at":1791107423,"model_context_window":258400}}
{"timestamp":"2026-10-04T09:50:25.044Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":15524,"cached_input_tokens":4480,"output_tokens":6,"reasoning_output_tokens":0,"total_tokens":15530},"model_context_window":258400}}}
{"timestamp":"2026-10-04T09:50:25.060Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"01a10652-5c3a-…","last_agent_message":"PONG","duration_ms":1752}}
```

An interrupted turn, from a second session, ends with this record in place
of `task_complete`:

```json
{"timestamp":"2026-10-04T09:51:57.614Z","type":"event_msg","payload":{"type":"turn_aborted","turn_id":"01a10653-2f21-…","reason":"interrupted","duration_ms":40325}}
```

- **Busy/idle:** `event_msg/task_started` → busy; `event_msg/task_complete`
  or `event_msg/turn_aborted` → idle. The other records (`response_item`,
  `turn_context`, `world_state`, `token_usage_record`, `item_completed`) do
  not change the status.
- **Waiting:** **not in the transcript.** While the approval prompt is up, the
  file sits between a `function_call` and its output with nothing that says
  "waiting". Only the `PermissionRequest` hook knows, which is why the
  transcript tier alone would not be Full.
- **Usage:** `event_msg/token_count.info.total_token_usage`, cumulative, plus
  `model_context_window` for the meter. `info` can be `null` on the first
  count.

## 4. Resume: `codex resume <uuid>`

`codex resume <SESSION_ID> [PROMPT]`, where the id is the UUID (a session name
also works; a UUID takes precedence). It reopens the conversation in the TUI,
keeps the id, and fires `SessionStart` with `source: "resume"` on the next
prompt. The `-c` hook and trust flags go after `resume` the same way.

A **missing id** prints
`ERROR: No saved session found with ID <id>. Run \`codex resume\` without an ID to choose from existing sessions.`
and exits. That is the "conversation lost" case: invariant 9's fresh start,
said out loud.

## 5. Input

- **Bracketed paste plus one `\r` submits once (invariant 8).** A two-line
  prompt pasted inside `ESC[200~ … ESC[201~` sat in the composer as two lines
  and was sent as one message on the single `\r`; `UserPromptSubmit` fired
  once, carrying both lines.
- **Esc** during a turn interrupts it ("Conversation interrupted"),
  `Interrupt` fires, and the transcript writes `turn_aborted`. Esc on the
  approval prompt declines ("You canceled the request…") and also ends the
  turn.
- **ctrl+c** during a turn opens a "Task is still running" chooser: *Cancel
  task*, *Run in background* (exit Codex and leave the task running in the
  daemon) and *Exit*. A second ctrl+c cancels.
- **ctrl+o collides.** Codex binds `ctrl+o` to its global **copy** action
  (`tui/src/keymap.rs`, `app.copy`), and it is omatty's default leader. Inside
  omatty, Codex's copy is unreachable and the router has no "send the leader
  literally" chord. Not a bug in either: invariant 1 is doing its job. The
  profile can rebind it per invocation if Codex's keymap takes `-c` (untested),
  or the user can change omatty's leader. Recorded for #152 to decide.
- No key needs special handling. Every key was forwarded, and nothing in
  Codex's own behaviour depended on omatty guessing.

## 6. Tier: **Full**

| `Caps` | Value | Evidence |
|---|---|---|
| `Identity` | `Reported` | `SessionStart.session_id` (§1) |
| `Status` | `Hooks` | `-c` hooks + `-c` trust, zero writes (§2) |
| `Waiting` | yes | `PermissionRequest` (§2, §3) |
| `Resume` | yes | `codex resume <uuid>` (§4) |
| `TurnBoundary` | yes | `Stop` hook, and `Interrupt` for a turn cut short; `task_complete` / `turn_aborted` |

`agent.Caps{Identity: agent.Reported, Status: agent.StatusHooks, Waiting: true, Resume: true, TurnBoundary: true}.Tier()` is `agent.Full`.

The subscription Full needs is `SessionStart`, `UserPromptSubmit`,
`PermissionRequest`, `Stop` **and `Interrupt`**. `Stop` does not fire for an
interrupted turn, so without `Interrupt` an Esc leaves the hook-driven status
busy until the tailer reads `turn_aborted`. `PreToolUse` and `PostToolUse`
exist too, but this spike did not subscribe to them. Whether they carry
claude's tool status is for #152 to check before relying on it.

## 7. Other things the profile must know

- **Folder trust is Codex's own prompt.** In a directory Codex has not seen,
  it asks "Trust this folder? … Your trust decision will be saved", and
  answering writes `[projects."<dir>"]` to the user's `config.toml`. omatty
  must not suppress or answer it, and cannot: `-c
  projects."<dir>".trust_level="trusted"` is ignored, because the TUI asks the
  app-server (`config/read`) and that does not see the client's CLI overrides,
  with or without `--no-daemon`. This is the same as claude's own folder-trust
  dialog: the user answers it, in Codex's pane, once per directory. It is not
  an omatty write.
- **Codex writes its own config on first run.** The first TUI start added
  `[tui] screen_reader_detection_done = true`. Codex's business, recorded so
  nobody blames omatty for it.
- **A persistent app-server daemon, which omatty's codex does not use.** A
  plain `codex` starts (or reuses) `codex app-server --listen unix://
  --managed-daemon` under `$CODEX_HOME/packages/…`, and it outlives the TUI.
  Threads run there, and "Run in background" leaves a task running after the
  pane exits. A codex started with `-c`, which is every codex omatty starts,
  runs embedded instead (§2), so its pane exiting does end its thread.
  Whether ctrl+c's *Run in background* is still offered in embedded mode was
  not checked.
- **Auth.** `codex login status` reports "Logged in" from the presence of
  `auth.json` alone. A stale refresh token shows up only as the TUI's sign-in
  screen. Not omatty's to fix; agentprobe should say so rather than time out.

## 8. Upstream

Nothing keeps Codex below Full, so the spike files no blocking request. One
non-blocking ask is worth making: a **documented** per-invocation trust route
for hooks given on the command line, so the profile does not depend on
re-implementing `hook_hash`. That would make §2's robustness note moot.

## Method

Codex was installed with `brew install --cask codex`. Experiments ran in a
scratch git repository with a scratch `CODEX_HOME` holding a copy of the
user's `auth.json` and `config.toml`, so trust answers and transcripts landed
there and not in `~/.codex` (verified by checksum before and after). A small
PTY driver (pyte as the screen) sent scripted keys and printed the screen; a
logging hook recorded each payload with the `OMATTY_SESSION` it saw. Source
reading pinned what the binary did, never the other way round.
