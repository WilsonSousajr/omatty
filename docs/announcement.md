# Putting omatty in front of people

**A draft, not a post** (#330). Nothing here has been published. It exists so
that when it is, it is not written in a hurry, and so the claims in it are the
ones this repository can actually defend.

## Why this is an issue and not a nice-to-have

Every defect found in omatty so far was found by its maintainer's own sessions,
CI and smoke tests — by someone who already agreed with the design. M12 produced
roughly 1,900 lines about other projects' users and issue trackers; omatty has
neither. The method was copied from `ai-memory`, and the outside feedback loop
that makes that method work was not.

Until someone else has run it, another research pass and another gate are
checking the maintainer's assumptions against themselves.

**The measure**, and the only one: the share of issues opened by someone other
than the maintainer — the same number Akita publishes for `ai-memory`. The
target in #330 is ten people who ran it and one issue not written by the
maintainer. Read the number off the tracker; do not write it down here, because
a count in a document is stale by the time anyone reads it.

## The claim to make

One sentence: **omatty runs your project's own check line inside each session's
worktree and puts the verdict on the session's card.**

That is the part no other tool in this space does. Everything else it does —
several repositories in one window, the real `claude` binary in an embedded
terminal, a diff pane with comments anchored to line content — is either
available elsewhere or a detail.

The frame that makes it land: every other tool optimises *how much agent work
you can have in flight*. Fleets, queues, boards, coordinators. omatty optimises
**how quickly you can tell whether what came back is any good.**

## What must not be claimed

These are the claims this repository has already had to retract once. Do not
reintroduce them in a post, where they cannot be corrected by a commit.

- **"The only tool that…"** — README carried a false field claim until #384 and
  #309 removed it. `docs/comparison.md` is the fair version, including where
  other tools are ahead. Link it in the post; people will find it anyway, and
  it is better to be the one who pointed at it.
- **"Roughly a hundred and fifty agent orchestrators."** Unsourced, and softened
  in the documents for that reason.
- **Anything about distribution beyond what exists.** A Homebrew cask, four
  release archives with checksums, and `go install`. There is no `curl | sh` and
  no Linux package (apt, AUR, nix).
- **`claude agents` does not exist / does not do this.** It exists and it
  overlaps. The honest line is what it does *not* do: run the project's own
  verification in each session's directory and show the verdict.

And state the limits in the post itself, not in a reply after someone finds
them: pre-1.0, no Windows, `claude` plus a half-spiked Codex, `dtach` and `gh`
optional but the experience is thinner without them.

## Where to post

In this order, because the first is the one that most needs the recording to
already exist.

| Venue | Note |
|---|---|
| **Show HN** | Title is `Show HN: omatty – a terminal ADE that runs your gate in each agent's worktree`. No adjectives, no "revolutionary". Be in the thread for the first few hours; the comments are the point, not the ranking. |
| **r/ClaudeAI** | Lead with the problem, not the tool. Flair as a project/tool post and read the subreddit's self-promotion rule first. |
| **Claude Developers Discord** | The showcase channel. Shorter, informal, and the best place for the GIF alone. |
| **Akita's community** | #330's own source is his post on shipping and the safety net, and `docs/llm-audit-gate.md` is directly downstream of his audit skills. Say so — the credit is owed and it is the honest framing. |

Every venue gets the same two links: the repository, and `docs/comparison.md`.

## The draft

> **omatty — a terminal ADE that runs your gate in each agent's worktree**
>
> I kept having the same problem with parallel Claude Code sessions: starting
> them is easy, and finding out whether what came back is any good is not. Three
> sessions in three worktrees means three terminals, three `git diff`s and three
> `go test` runs, by hand, every time.
>
> omatty is one terminal window with several projects and several sessions in it.
> It runs the real `claude` binary in an embedded terminal — it does not
> reimplement Claude's interface — and it owns the panes around it:
>
> - **The gate.** Each project carries its own check line — `gofmt`, `vet`,
>   `lint`, `test -race`, a coverage script. omatty runs it in the session's own
>   directory, puts the verdict on that session's card, and sends the failures
>   back into the session that caused them with one keystroke.
> - **Review.** A diff of everything a session changed, or just its last turn,
>   with your comments anchored to the *content* of lines rather than their
>   numbers — because the agent edits the file while you are reading it — sent
>   back as one message.
> - **Coverage on the diff.** The lines this change added that no test reached,
>   marked in the diff itself.
>
> What it does not do: delegate, plan, schedule, or decide. It never sends a
> prompt on your behalf. `docs/ROADMAP.md` lists what that rules out and argues
> each refusal, and `docs/comparison.md` compares it to everything else in this
> space including where those tools are ahead.
>
> macOS: `brew install WilsonSousajr/tap/omatty`. Linux: a release archive.
> Pre-1.0, no Windows, and I have been its only user — which is why I am posting.
> If you try it and stop, there is an issue template for exactly that, and it is
> the most useful thing you could send me.

Trim for Discord, keep whole for Show HN.

## The recording

Two clips, both short. One gate run that fails and is sent back; one review
comment that lands.

**Rehearse against `fake-claude`, record against the real thing.** The scratch
`HOME` recipe below is the smoke-test setup, and it is how you check the
recording geometry and the key sequence without spending tokens. Do not publish
that take: scripted output presented as a model's answer would be a lie, and a
small one is still one.

```sh
# 1. Rehearse, with the smoke-test setup: a scratch HOME so nothing touches
#    your real ~/.omatty, and `fake-claude` standing in for the binary. The
#    path is short on purpose - a long one wraps the footer on camera.
#    fake-claude is one executable, so it is symlinked in as `claude`; the
#    launcher resolves the name on PATH.
T=/tmp/oh; mkdir -p "$T/.omatty" "$T/bin"
ln -sf "$PWD/testdata/fake-claude" "$T/bin/claude"
#    write $T/.omatty/state.json with one project and one session
env HOME="$T" PATH="$T/bin:$PATH" TERM=xterm-256color \
  PTY_COLS=100 PTY_ROWS=30 go run ./testdata/ptyrun omatty
#    The card will say `demo is stopped`: `sessions.lazy_start` is on, so
#    press `enter` on it to start the session. That is the real behaviour,
#    not a fault of the setup.

# 2. Record. A throwaway demo repository with one deliberately failing
#    test, a real `claude`, and a terminal at 100x30 - the card column is
#    28 wide, so anything narrower truncates the branch name on camera.
asciinema rec omatty-gate.cast --cols 100 --rows 30 --idle-time-limit 2
#    ctrl+o N   a worktree session
#    ask for the change that breaks the test
#    ctrl+o g   run the gate, let it go red
#    S          send the failure back
#    watch it fix it, ctrl+o g again, green
#    exit
agg omatty-gate.cast omatty-gate.gif --font-size 16
```

`--idle-time-limit 2` is what makes it watchable: the interesting part is the
verdict appearing, not the model thinking. Keep the finished GIF under about
45 seconds and under 5 MB, and put it in the README above the Status section.

## Then the templates

`.github/ISSUE_TEMPLATE/` has two, added with this document:

- **Bug report** — version, OS and terminal, which of `dtach`/`gh`/a gate are
  installed, what happened, **what the card said versus what claude was
  actually doing** (those two disagreeing is itself the bug, since status comes
  from hooks and the transcript and never from the screen), and the log at
  `~/.omatty/logs/`.
- **I tried it and stopped because…** — three questions, no reproduction, and
  "the whole idea is wrong" is a valid answer.

Blank issues stay enabled. A stranger with something to say should never meet a
form that will not let them say it.
