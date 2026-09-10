# omatty M6 persistence and adoption design

Written 2026-09-05, after M4 was built and reviewed.

## Context

Quitting omatty kills work. `ui.Run` defers `closeTerminals`, which closes every
embedded terminal; closing the PTY master hangs up the slave, and the kernel
sends `SIGHUP` to the foreground process group. Every `claude` dies with it. The
*conversation* survives, because the transcript is on disk and #36 taught the
launcher to `--resume` it — but the **turn in flight does not**. Quit while
Claude is three tool calls into a task and that work is gone; relaunching
resumes the conversation at the last completed turn.

That is the whole of M6's first half: quitting should detach, not kill.

The second half is adoption. M4's discovery (#91) reads Claude Code's transcript
store and proposes *projects* to register. It deliberately stops there — the
roadmap says so: reconstructing a session from its transcript and reattaching to
one omatty started are the same machinery, and building it twice is the waste.
M6 is where that machinery exists, so adoption lands here.

## Persistence

### The detach layer is dtach

`dtach` is a program that does one thing: hold a PTY on behalf of a client that
comes and goes. It has no status bar, no prefix key, no window management, and
no configuration file. That absence is the entire argument for it over `tmux`:
omatty renders a terminal inside a terminal already, and a multiplexer in the
middle would add a third keyboard owner and a second escape-sequence translator
between the operator and Claude. `dtach` adds neither.

**It is optional.** dtach is not installed on the author's machine and omatty
must not require it. `detach.New` looks the binary up on `PATH`:

- found → `*Dtach`, and sessions persist;
- not found → `*Plain`, which returns the command unchanged and stops nothing.
  omatty behaves exactly as it does today and says so once, in the status line:
  `dtach not found: sessions will not survive quit (brew install dtach)`.

There is no install step, no vendored binary, and no hard dependency in
`go.mod`. The feature degrades to today's behaviour, the way a failed hook
socket degrades to tailer-only (#49).

### Invariant 4: dtach lives in one package

`internal/detach` is the only package that names the dtach binary, exactly as
`internal/vcs` is the only one that shells out to git and `internal/termwrap`
the only one that imports bubbleterm. `supervisor` holds a `detach.Holder`
interface; `ui` never hears of dtach at all, only of a `Stop` function and a
warning string.

```go
type Holder interface {
	Wrap(id string, cmd *exec.Cmd) (*exec.Cmd, error)
	Stop(id string) error
	Persists() bool
}
```

### The command line

For a session whose claude command is `claude <flag> <uuid> --settings <hooks>`
(where `<flag>` is `--session-id` or `--resume`, decided as today by whether a
transcript exists — #36), `Dtach.Wrap` produces:

```
dtach -A <sock> -E -z -r winch \
  sh -c 'echo $$ > "$0"; exec "$@"' <pidfile> \
  claude <flag> <uuid> --settings <hooks>
```

Every piece earns its place:

- **`-A <sock>`** is attach-or-create. If the socket is live, this attaches to
  the running claude; if it is not, dtach creates the master and runs the
  command. First launch and reattach are therefore one code path, and no caller
  has to ask which case it is in. On reboot the socket is gone, dtach creates a
  fresh master, and the claude it runs carries `--resume` because the transcript
  exists — #36's fallback, unchanged and unaware of any of this.
- **`-E`** disables dtach's detach key (`Ctrl-\`) and **`-z`** its suspend key
  (`Ctrl-Z`). Both keys would otherwise be swallowed by dtach instead of
  reaching Claude. Invariant 1 says every keystroke goes to the PTY except the
  `Ctrl+O` leader; these two flags are what keep that true with dtach in the
  path. omatty detaches by closing its end, never by a key.
- **`-r winch`** makes dtach send `SIGWINCH` on attach. Claude Code redraws on a
  window-size change, so this is what repaints the pane when omatty reattaches.
  Without it the pane is blank until the next output. This is the single
  riskiest line in the milestone and the reason the smoke test exists.
- **The `sh` wrapper** records claude's pid. dtach exposes neither its own pid
  nor its child's, and archiving a session has to be able to stop the claude
  behind it. `sh -c 'echo $$ > "$0"; exec "$@"' <pidfile> claude ...` sets `$0`
  to the pidfile and `$@` to the claude command; `exec` replaces the shell, so
  the `$$` already written is claude's own pid. It runs once, when dtach creates
  the master, and not on a reattach.

`Wrap` carries `cmd.Dir` and `cmd.Env` onto the new command, so the session
still starts in its own directory.

### Paths, and why the directory is called `s`

Per-session paths are derived from the session uuid:

```
~/.omatty/s/<uuid>.sock
~/.omatty/s/<uuid>.pid
```

Nothing new is persisted. **Invariant 9 holds without a schema change**:
`state.json` already carries the uuid, and every dtach path is a pure function
of it, so `state.json` still suffices to relaunch every session.

The directory is one letter because a unix socket path is capped — `sun_path`
is 104 bytes on macOS, 108 on Linux — and `bind(2)` fails past it. A uuid is 36
characters and `.sock` is 5, so the fixed prefix is the only part omatty can
spend less of. `detach.SocketPath` enforces the cap and returns an error naming
the path and the limit rather than letting dtach fail with a message the
operator cannot act on. `PidPath` needs no cap: nothing binds it.

### Quitting, archiving, restarting

- **Quit (`ctrl+o q`) always detaches.** No code changes: `closeTerminals`
  closes omatty's PTY, the dtach *client* sees EOF and exits, and the master and
  claude live on. This is the behaviour change the milestone exists for, and it
  arrives by deleting nothing.
- **Archive (`ctrl+o x`) stops the process.** This is the one place a claude is
  deliberately killed, and now it must say so explicitly: `dropSession` closes
  the terminal and then calls `Stop(id)`. Without it, archiving would leave an
  orphan claude holding a socket with no row in `state.json`, reachable from
  nothing. `Dtach.Stop` reads the pidfile, sends `SIGTERM`, waits a bounded
  couple of seconds, escalates to `SIGKILL`, and removes the pidfile. A missing
  pidfile is not an error: there is nothing to stop.
- **Restart (`ctrl+o r`) improves for free.** Its close-then-start order is
  unchanged, but the new start is `dtach -A` on the same socket, so a live claude
  behind a frozen pane is *reattached* rather than killed and resumed.

### Status after a reattach

Invariant 2 is untouched. Status comes from the hook socket and the JSONL
tailer, never the screen, and `watcher.Tail` starts at offset zero and replays
the transcript. A reattached session therefore reports correct status with no
new code, and `StatusExited` still comes from the `SessionEnd` hook rather than
from process death — which matters here, because a detached claude that is very
much alive must never render as exited.

## Adoption

### Candidates

`discover.ProposeSessions(storeRoot, git, projectRoot, known)` returns the
sessions worth offering for one registered project:

```go
type SessionCandidate struct {
	ID       string    // the uuid, from the transcript filename
	Title    string    // the first typed prompt, flattened and truncated
	Dir      string    // the cwd the transcript records
	LastUsed time.Time // the transcript's mtime
}
```

It reuses everything `Propose` already built: `transcripts` for the newest-first
listing, `readCwd` for the recorded working directory, `resolveRoot` for the
filesystem-and-git validation, and the same `maxHeadLines` / `maxHeadBytes`
caps, which exist because one transcript record is routinely hundreds of
kilobytes (#64, #91). A slug directory whose cwd does not resolve to
`projectRoot` is skipped whole; within a kept directory, each transcript becomes
one candidate unless its uuid is already in `known`.

**Transcript text is untrusted** (AGENTS.md, Security). The title is the only
prose read out of it, it is flattened to one line and truncated, and it is used
as a display string and nothing else. omatty parses transcripts for status; it
never acts on text found inside them.

### Registering one

`registry.AdoptSession(store, id, project, title, dir)` writes
`Session{ID, Project, Title, Dir, Worktree: false}`.

`Worktree` is false and that is load-bearing, not incidental: omatty did not
create that directory, so archive must never offer to delete it. This is the
same rule `archiveChoices` already applies to a main-checkout session (#40).
Adoption refuses a duplicate id, naming it, and a blank title, reusing
`RenameSession`'s trimmed-title guard.

Starting an adopted session needs no new code. The launcher stats the transcript,
finds one, and uses `--resume` (#36).

### The surfaces

`ctrl+o A` opens a picker over the project under the cursor. `ctrl+o a` is
already discovery, so adoption takes the shifted letter, matching the `n` / `N`
pair the new-session prompt already uses. It is the same `pickList` widget with
the same multi-mark keys, opened as its own `modalKind` so that committing it
adopts rather than registers, and scanned in the background behind the same
token guard that keeps a slow scan from overwriting a newer list (#91).

`omatty adopt <project>` is the CLI twin of `omatty discover`. Both selections
are parsed by one shared function rather than two copies — `dupl` would flag the
second copy, and a collision policy that exists twice drifts (#91).

## Testing

Per package, with named fake types:

- `detach`: the wrapped command line is asserted piece by piece, including that
  `-E` and `-z` are present (a test named for invariant 1, since dropping them
  would silently steal two keys from Claude). `Stop` signals a real child the
  test spawns and owns.
- `supervisor`: a `fakeHolder` proves the launcher wraps through the holder and
  that invariant 3 still holds — nothing on the command line names the user's
  own settings file.
- `registry`: adoption registers with `Worktree:false`, refuses a duplicate id
  and a blank title.
- `discover`: candidates are scoped to the project, exclude known ids, are
  ordered newest first, and carry a flattened title.
- `ui`: the picker lists, marks and commits; archive stops the process; the
  missing-dtach warning renders; and `ui.LeaderKeys()` documents `A`, which
  guards the #103 class of bug — a key that exists in the router and in no
  keymap the operator can see.

The gate is unchanged and is met with `Plain`, since `testdata/fake-claude`
never touches dtach.

## Done when

Verified with the real binary in a sized PTY, not only the coverage gate
(roadmap rule 2):

1. With dtach installed: start a session, send a prompt so a turn is in flight,
   quit with `ctrl+o q`, relaunch, and watch the turn finish on screen. Without
   dtach: omatty starts, shows the warning once, and behaves as today.
2. `ctrl+o A` over a registered project lists the claude sessions in that
   directory omatty does not yet track; adopting one starts its pane and resumes
   it. `omatty adopt <project>` does the same from the CLI.
