# An LLM audit as a gate step

**A recipe, not a feature** (#333). omatty's gate runs a project's own shell
lines and takes each verdict from an exit status (invariant 12). An LLM review
is one of those lines. Nothing in omatty needs to change to run one, and this
document exists so you can add one knowing what it costs and how it lies.

`docs/examples/audit.sh` is a working script. Copy it into your repository,
read it, and rewrite the checklist — it is an example, not a dependency.

## Why this is a step and not a reviewer

An audit step **is a check**. It runs when the gate runs, it sends nothing on
its own, and its output reaches the session only through `S`, exactly as a test
failure does. That is why it fits: a person still presses the key.

Building a reviewer *agent* into omatty would be a coordinator spawning agents,
which `docs/ROADMAP.md` refuses under "Not on the roadmap". The difference is
not whether a model is involved. It is whether omatty acts while nobody is
reading.

## Adding it

Gate steps are **not** in `~/.omatty/config.toml`. That file's `[gate]` table
holds only `max_parallel` and `auto`; the steps themselves live per project in
`~/.omatty/state.json`, because a gate is a command that will be run and a
clone does not get to choose what runs on your machine (#226).

`omatty gate <project>` proposes the line it detects — `gofmt`, `go vet`,
`golangci-lint`, `go test -race`, a coverage script — and will never propose
this one. So add it by hand, as `cmd/omatty/gate.go` says when detection finds
nothing:

```json
{
  "name": "audit",
  "run": "claude --version >/dev/null && ./scripts/audit.sh"
}
```

A step has four keys — `name`, `run`, and for a coverage step `kind` and
`profile`. An audit step is an ordinary step: no `kind`, no `profile`. `name`
is short because it labels one cell of the card's gate strip.

**Put it last.** The gate stops at the first step that does not pass, so every
deterministic check should get its answer in before the expensive
non-deterministic one runs at all.

### Why the line begins with `claude` and not with the script

Before running a step, omatty resolves its **leading word** with `command -v`
in the session's own directory and reports `missing` — a statement about the
machine — without running anything. `missing` is not `fail` (invariant 12).

That pre-flight only ever sees the first word, so the two spellings differ:

| Step's `run` | With no `claude` on PATH |
|---|---|
| `./scripts/audit.sh` | **fail, exit 127** — the script's own "not found", read as a verdict about your code |
| `claude --version >/dev/null && ./scripts/audit.sh` | **missing** — nothing runs, and the card says so |

Measured against `internal/gate` on 2026-09-26. The guard costs one process
and buys the honest answer: an uninstalled tool must not send a session off to
fix code that was never broken.

## What it cannot be trusted to do

**It is not deterministic.** The same diff can pass and fail. A gate step that
sometimes fails for no reason is a gate people learn to ignore, so:

- keep the checklist **short and specific** — name the defects that have cost
  you real time, and say explicitly that formatting, naming and structure
  belong to other steps;
- demand a single verdict line and treat its absence as a failure. A model that
  would not follow the one instruction about its own output is not evidence
  that the code is sound;
- expect to re-run it, and to disagree with it.

**It costs money and time on every run.** Under `gate.auto = true` that is once
per turn, per session, across every session you have running. The script bounds
what it sends (`AUDIT_MAX_BYTES`, 128 KiB) and how long it waits
(`AUDIT_TIMEOUT`, 300 s, when `timeout` or `gtimeout` exists); read both before
turning `auto` on.

**The diff is untrusted input.** It can contain text addressed to the reviewing
model — including text written by the session you are auditing. The prompt names
the diff as data and asks for such text to be reported rather than obeyed, and
the verdict is read from the **last few lines only**, so an `AUDIT: PASS`
planted in the middle of a file does not become a verdict. That is mitigation,
not a guarantee; an audit step is a second opinion, never a permission check.

**omatty never reads the model's prose.** The script does, and the script is
code you can read. What crosses into omatty is the exit status the script
chooses — that is the whole of invariant 12, and it is why an audit step cannot
quietly redefine what "green" means.

## What you get

The step behaves like any other: a cell on the card's gate strip, its output in
the gate pane (`ctrl+o g`), and `S` sending the failure into the session that
caused it. A finding arrives as a sentence about a file, which is exactly the
shape the session can act on.

Tried against a real session on 2026-09-26, including the missing-`claude` case.
