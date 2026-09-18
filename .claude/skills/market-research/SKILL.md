---
name: market-research
description: Survey the field omatty ships into and turn it into roadmap items. Use when re-running the M12 competitive pass, checking a claim README or ROADMAP makes about competitors, evaluating a newly-found competing tool, or before any release that changes what omatty claims about the field. Produces dated documents in docs/research/ and issues on project 13; never edits code.
---

# Market research

The method M12 ran on 2026-09-18, captured so the next pass re-runs it instead
of re-deriving it worse. Adapted from `akitaonrails/ai-memory`'s `docs/` — the
same artifact set one project-stage later.

**Read `docs/research/2026-landscape.md` first.** It is the previous pass. A
new pass is a follow-up that says what it is a follow-up to and what moved, not
a fresh start.

## The rules, before the artifacts

These are what make the output worth anything. Breaking one quietly is worse
than not running the pass.

1. **A capture date in every header**, and the sentence that goes with it:
   *"every load-bearing claim was checked against a live page on <date>, not
   remembered."* If you did not open it, do not write it.
2. **Primary sources only.** The repository, its README, its issue tracker, its
   releases, the vendor's own documentation, a published paper. **Never a
   third-party "X alternatives" or "best tools for Y" page.** This category is
   full of them and they are all published by a vendor that appears in its own
   ranking. Named, so the next pass recognises them on sight:
   `abralo.com/alternatives`, `runpane.com/alternatives/*`,
   `nimbalyst.com/blog/*`, `codeagentswarm.com/guides/*`,
   `munderdiffl.in/blog/*`. They may be cited **only** as evidence of how a
   vendor positions itself, labelled as such.
3. **Read the code, not the marketing.** Every claim about how a competitor's
   feature works names the file it was read in. M12's sharpest finding — that
   Orca anchors diff comments on `lineNumber` — is invisible from its README,
   which says only "drop comments on any diff line".
4. **Label competitor benchmark and performance claims vendor-self-reported.**
   Verify our own against the code or the gate, never against `README.md`.
5. **Separation of powers.** Research documents make **no** implementation
   decisions and say so in their header. Recommendations are numbered `R1..Rn`
   and are inputs. `docs/ROADMAP.md` decides, in the open.
6. **Delete a claim that cannot name a file.** #300 deleted three moat lines
   this way. That is the document working, not failing.
7. **State your coverage honestly.** A probe is not a read. `issues-orca.md`
   says in its header that 3,036 open issues were sampled against chosen
   queries and that the absence of a theme is not evidence of its absence.

## The artifacts

Produced in this order; each is independently shippable as one issue and one PR
labelled `docs`, and stopping after any of them leaves the repository better.

| # | File | Job |
|---|---|---|
| 1 | `docs/research/<year>-landscape.md` §1-3 | Camps, who is in each, the signal table, a field-size probe. |
| 2 | `docs/research/<project>.md` | Per-competitor deep dive, read at code level. Only for the ones that genuinely overlap. |
| 3 | `docs/research/issues-<project>.md` | Tracker mining. **The feature-discovery engine** — see below. |
| 4 | `docs/research/<year>-landscape.md` §4-6 | Developments, `R1..Rn`, sources. |
| 5 | `docs/research/prior-art-findings.md` | Everything against our own code. P0/P1/P2 + **Ideas Not To Copy**. |
| 6 | `docs/research/competitive-parity.md` | The self-critical audit. Migration bar, verified moat, honest gaps, per-competitor verdicts. |
| 7 | `docs/comparison.md` + `README.md` + `AGENTS.md` + `docs/ROADMAP.md` + issues | Publish and decide. |

### The deep-dive skeleton (artifact 2)

Chosen so each document answers, per project, the exact claims `README.md`
makes about the field:

purpose and scope → session model (what *is* a session; worktree or not) →
process model (tmux, dtach, PTY, Electron, container) → status and state (hooks,
transcript, or screen-scrape) → review surface (is there a diff, and how good) →
gate/CI integration → multi-repository support → persistence across restart →
deployment and dependencies → **strengths worth borrowing** → **weaknesses to
avoid** → bottom line → sources.

### The mining skeleton (artifact 3)

Per project: the tracker's character (roughly what fraction is feature
requests, bugs closed by the next release, and hard still-open bugs at
architectural seams) → pain points **ranked by recurrence**, with issue numbers
→ **the design choice that caused each cluster** → what the maintainers have
not solved → do-not-repeat lessons.

Hunt three things, or the pass drifts into a bug list:

1. features their users keep asking for **that omatty already does** → lines in
   `comparison.md`;
2. bugs **structural to their architecture and impossible in ours** → evidence
   for an invariant that was previously argued from principle;
3. features their users keep asking for **that omatty also lacks** → the
   strongest roadmap candidates, because someone else's users already paid to
   discover the need.

**A need that appears independently in three trackers is a different kind of
fact from one that appears in one.** That is what
`docs/research/issues-synthesis.md` exists for, and it is what found M12's P0.

## The commands

Signal table:

```bash
for r in owner/repo ...; do
  j=$(gh api "repos/$r")
  rel=$(gh api "repos/$r/releases/latest" --jq '.tag_name+" ("+.published_at[0:10]+")"')
  oi=$(gh api "search/issues?q=repo:$r+is:issue+is:open&per_page=1" --jq .total_count)
  echo "$j" | jq -r --arg rel "$rel" --arg oi "$oi" \
    '[.full_name,(.stargazers_count|tostring),.pushed_at[0:10],$rel,$oi,
      (.license.spdx_id//"none")]|@tsv'
done
```

Field size (urlencode the query):

```bash
gh api "search/repositories?q=<urlencoded>&per_page=1" --jq .total_count
```

Tracker mining:

```bash
gh issue list --repo <r> --state all --limit 300 \
  --json number,title,state,labels,comments \
  --jq '.[]|[(.number|tostring),.state,(.comments|length|tostring),
             (.labels|map(.name)|join("/")),.title]|@tsv' \
  | sort -t$'\t' -k3 -rn
```

Sort by comment count, not date: engagement is the recurrence signal. For a
tracker too large to read, `gh search issues --repo <r> "<term>"` against terms
drawn from omatty's own design — and say in the header that you did.

Finding candidates: `gh search repos`, `gh search repos --topic=<t>`. Multi-word
queries return nothing surprisingly often; try narrower ones and go by topic.

## Traps this pass already hit

- **A stale repository can mean a rename, not a death.** `stravu/crystal` had
  not been touched in seven months because it became `nimbalyst/nimbalyst`,
  which is active. Check the description and the org before concluding
  abandonment. `devflowinc/uzi` is the one that is actually dead.
- **Stars do not decay.** A dead project keeps its 583.
- **`gh issue list --json comments` returns the comment objects, not a count.**
  Use `(.comments|length)`, or the output is megabytes.
- **Cite invariants by number, and check the number.** M12 wrote "invariant 3"
  for screen-scraping across four documents; it is invariant 2. The
  anti-orchestrator stance is not an invariant at all — it is
  `docs/ROADMAP.md`'s "Not on the roadmap".
- **`gh issue create` does not put an issue on the board**, and neither does
  labelling it. Adding is always a second step.
- **Check the first party first.** M12 nearly missed that `claude agents`
  shipped the category's core feature, which mattered more than any competitor
  in the table.

## When to run it

- **At each milestone close-out**, as a diff against the previous capture: re-run
  the signal table, note what moved, and only deep-dive what changed.
- **In full before any release** that changes what `README.md` claims about the
  field — which, after M12, it does by naming the gate as the thing nobody else
  has. That claim has an expiry date and this is what checks it.
- **On demand** when a new competitor appears, which is artifact 2 alone.

Do not run the full suite more often than the field changes. M12 took one pass
to produce three issues; a second pass that produces none is a pass that should
have been a signal-table refresh.
