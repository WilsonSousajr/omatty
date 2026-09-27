# How this camp found its users — go-to-market research, September 2026

> Captured **2026-09-27**, against `develop` at v0.7.0 plus #505. Every
> load-bearing claim was checked against a live page on that date, not
> remembered. Hacker News scores and dates come from the HN Algolia API
> (`hn.algolia.com/api/v1/items/<id>`), and each one quoted here was re-read
> from it. Subreddit rules come from archived rule pages, dated where cited.
>
> **Analysis only.** No decision is made in this file. It is an input to #330
> ("ten users, and one issue not written by the maintainer"); what omatty
> actually posts, where and when, is decided there. Recommendations are
> numbered G1..Gn so they do not collide with `2026-landscape.md`'s R1..R14.
>
> **Coverage, stated honestly.**
> - **Star trajectories come from Wayback Machine captures** of each
>   repository's GitHub page, not from the API. The stargazer-timestamp
>   endpoint returns 404 for repositories you do not own, and GraphQL's
>   `stargazers.totalCount` returns 0 from this environment. Captures are
>   irregular, so a trajectory is a set of dated points, not a curve.
> - **Reddit was not reachable directly.** Thread scores come from PullPush
>   and are lower bounds (marked ≥). PullPush then rate-limited the pass.
> - **Product Hunt and X were not checked.**
> - Nine projects were traced, chosen from `2026-landscape.md` plus herdr
>   (§7.2 there) and lazygit as the reference for a Go TUI's adoption. It is not a
>   census.

## 1. The finding

**In this camp, adoption tracked a launch event, not the quality of the
repository.** The evidence is the contrast between two projects:

- `brizzai/fleet` has the best README in the set: a tagline ("Run 10 coding
  agents. Stay sane."), a captioned screenshot, a See / Act / Ship structure,
  and four install paths. It never had a launch that left a trace on HN or
  Reddit, and it has 54 stars.
- `jesseduffield/lazygit` got nowhere from its first posts (r/webdev, 5
  points). One Show HN took it to about 9,000 stars in three weeks. Its
  author's own account is in [Lazygit 5 years on](https://jesseduffield.com/Lazygit-5-Years-On/):
  the earlier posts "fell on deaf ears".

fleet's 54 is omatty's base rate: a good repository with no launch stays
where it is. omatty has 1 star, 0 forks, and no issue from anyone but the
maintainer, as of today.

## 2. How each project got its users

| Project | Launch | Venue, score / comments | Stars around it (Wayback) |
|---|---|---|---|
| `jesseduffield/lazygit` | 2018-08-05 | [Show HN](https://news.ycombinator.com/item?id=17689014) 551 / 229; r/golang the same day ≥112 | 2 → 9,006 (08-26) |
| `BloopAI/vibe-kanban` | 2025-07-11 | [Show HN](https://news.ycombinator.com/item?id=44533004) 195 / 132; r/ClaudeCode 07-16 ≥48 | 307 → 1,647 (07-14) → 3,527 (07-31) |
| `dagger/container-use` | 2025-06-05 | Open-sourced on stage at the AI Engineer World's Fair keynote; [Show HN](https://news.ycombinator.com/item?id=44193933) 82 / 17 | 87 → 1,518 (06-15), then flat |
| `herdrdev/herdr` | 2026-06-29 | [HN](https://news.ycombinator.com/item?id=48714802) 166 / 110; then [HN](https://news.ycombinator.com/item?id=48756578) 404 / 178 (07-02, submitted by a third party, after a run on GitHub Trending); YC launch 08-06, 281 / 189 | 6,961 (06-24) → 9,322 (07-01) → 17,793 (07-18) |
| `stravu/crystal` | 2025-06 to 10 | Eight HN posts, none above 9 points; one [r/ClaudeAI post](https://www.reddit.com/r/ClaudeAI/comments/1lwet1z/) 07-10 ≥125 / 47 | 366 (06-30) → 827 (07-15) |
| `kbwo/ccmanager` | 2025-06-10 | [r/ClaudeAI](https://www.reddit.com/r/ClaudeAI/comments/1l80jd4/) ≥49 / 19; HN 1 point | 44 (06-11) → 316 (07-28) |
| `smtg-ai/claude-squad` | 2025-04-03 | [HN](https://news.ycombinator.com/item?id=43575127) 5 / 1, posted by someone else | 14 → 290 (04-06) → 2,851 (2025-07-14) |
| `stablyai/orca` | 2026-03-27 | [Show HN](https://news.ycombinator.com/item?id=47549197) 21 / 10 | 9 → 10,394 (07-02) → 52,503 (08-24) |
| `imbue-ai/sculptor` | 2025-09-30 | [Show HN](https://news.ycombinator.com/item?id=45427697) 176 / 85, linking to `imbue.com`, not GitHub | 21 on launch day → 166 (2026-05) |
| `brizzai/fleet` | none found | — | 54 today |

Four patterns, each with its counter-example:

1. **One high-scoring thread moves a small project by thousands.** lazygit,
   vibe-kanban and herdr each got a step change from one thread. Crystal's
   eight low posts did less than its single Reddit post.
2. **The thread has to land on something you can star or install without
   asking anyone.** Sculptor's 176-point Show HN pointed at a landing page for
   a closed, email-gated beta, and the repository gained almost nothing
   (commenters: "not quite ready", [45428093](https://news.ycombinator.com/item?id=45428093);
   the email gate, [45428677](https://news.ycombinator.com/item?id=45428677)).
   lazygit's author credits keeping users "one click away from starring".
3. **The largest numbers did not come from HN at all.** Orca's Show HN scored
   21. Its growth, about 800 stars a day between 07-02 and 08-24
   ((52,503 − 10,394) / 53 days), came with Y Combinator backing, an X
   account, iOS and Android apps, and READMEs in six languages. claude-squad's
   2025 climb matches Claude Code going mainstream, not any post found. Both
   are evidence that riding the category's growth matters, and neither is
   reproducible by a single maintainer.
4. **A launch is a spike, not a slope.** container-use went from 87 to 1,518
   in ten days and then flattened. What kept lazygit growing afterwards is
   not in this data.

### What the READMEs lead with

From each README's first screen, read with `gh api repos/<owner>/<repo>/readme`:

- claude-squad: a one-line tagline, a screenshot and a video. Install with
  `brew install` or `curl | bash`. tmux and `gh` are listed as prerequisites.
- ccmanager: a video, and a "Why CCManager over Claude Squad?" section built
  on one difference (no tmux). Install with `npm i -g`.
- vibe-kanban: "Get 10X more out of…", screenshots. Install with
  `npx vibe-kanban`.
- container-use: a tagline, a GIF demo, `brew install`. Its SVG demo crashed
  mobile Chrome during the launch thread
  ([44197698](https://news.ycombinator.com/item?id=44197698)).
- fleet: a tagline, a captioned screenshot, and four install paths (brew,
  curl, `go install`, Docker).

Every one of them shows the product moving in the first screen. omatty's
`README.md` has no image, GIF or recording. The recording exists, on
omatty.com, and the repository's `homepage` field is empty, so the GitHub page
does not link to it.

## 3. What the threads said

Objections, ranked by how many separate threads raised them. Numbers are HN
item ids.

| Objection | Where | What answered it, when something did |
|---|---|---|
| **"Why not tmux (plus a hook)?"** | herdr [48823176](https://news.ycombinator.com/item?id=48823176), [48823440](https://news.ycombinator.com/item?id=48823440); Agent-Manager [49109390](https://news.ycombinator.com/item?id=49109390); the ccmanager Reddit thread | herdr's `/compare/` page and a six-minute video, which commenters linked to each other; "a TUI is a view over what's running" ([49115870](https://news.ycombinator.com/item?id=49115870)) |
| **"Why not plain worktrees and terminals?"** | [44538682](https://news.ycombinator.com/item?id=44538682), [45434888](https://news.ycombinator.com/item?id=45434888), [44196502](https://news.ycombinator.com/item?id=44196502) | container-use: "worktrees isolate file edits; containers isolate execution" ([44196575](https://news.ycombinator.com/item?id=44196575)) |
| **"There are dozens of these."** | [49109488](https://news.ycombinator.com/item?id=49109488) ("Everybody has a TUI for running Claude Code these days"), [49109763](https://news.ycombinator.com/item?id=49109763), [47220690](https://news.ycombinator.com/item?id=47220690) | Nothing did, in any thread read. |
| **"How is this different from X?"** | Nearly every thread | A comparison written before launch. |
| **"Vibecoded. Nope."** | Top comment on herdr ([48823147](https://news.ycombinator.com/item?id=48823147)) | — |
| **Telemetry, email capture, OAuth scopes** | vibe-kanban [44535243](https://news.ycombinator.com/item?id=44535243), [44533689](https://news.ycombinator.com/item?id=44533689) ("hard NO"); Sculptor [45428677](https://news.ycombinator.com/item?id=45428677) | The founder merged an opt-in telemetry PR during the thread and was thanked for it ([44536953](https://news.ycombinator.com/item?id=44536953)). |
| **"Anthropic will build or buy this."** | [45428447](https://news.ycombinator.com/item?id=45428447), [49112708](https://news.ycombinator.com/item?id=49112708) | — |
| **Are parallel agents even productive?** | [44534052](https://news.ycombinator.com/item?id=44534052), [46993479](https://news.ycombinator.com/item?id=46993479), [47220440](https://news.ycombinator.com/item?id=47220440) | — |
| **Cost; needs a Max plan** | [47221725](https://news.ycombinator.com/item?id=47221725), [47222550](https://news.ycombinator.com/item?id=47222550) | — |
| **Remote / SSH / phone** | [44267000](https://news.ycombinator.com/item?id=44267000), [45446103](https://news.ycombinator.com/item?id=45446103), [49111995](https://news.ycombinator.com/item?id=49111995) | — |
| **Electron bloat** | Sculptor [45462236](https://news.ycombinator.com/item?id=45462236) | — |
| **No LICENSE file** | Orca [47552848](https://news.ycombinator.com/item?id=47552848) | — |

How the objections map onto omatty:

- **tmux, worktrees, "dozens of these", "different from X".** Every one of
  these is a request for a comparison that already exists. `docs/comparison.md`
  is that comparison, but it has to be correct first: #515 lists seventeen
  claims in it, the README and omatty.com that a commenter could refute
  today. It also has to name herdr, which is the tool a commenter is most
  likely to bring up (`2026-landscape.md` §7.2).
- **Vibecoded.** omatty is built with Claude Code, and says so. What it can
  show instead is its gate: 90% coverage, C.R.A.P. under 12, SDP enforced,
  and a regression test per bug. That is a direct answer to the objection,
  and it is the same thing omatty sells.
- **Telemetry and scopes.** omatty sends no telemetry and never writes to
  `~/.claude/settings.json` (invariant 3). This is worth one line in a
  launch post, because it pre-empts the thread that nearly sank vibe-kanban's
  launch.
- **Anthropic will ship it.** Agent view already ships the session list
  (`2026-landscape.md` §7.5). The honest answer is the gate, which is the one
  thing agent view does not do.
- **Electron, no LICENSE.** omatty is a single Go binary under MIT.
- **Remote.** omatty runs in the terminal you SSH into, but that no longer
  sets it apart (§7.7 R11 there).

## 4. Who the buyer is, and the words they use

The threads with the most engagement are about **judging** agent output, not
producing more of it. That is omatty's thesis, stated by strangers:

- [r/ExperiencedDevs, "I won't be reviewing AI generated PRs"](https://www.reddit.com/r/ExperiencedDevs/comments/1towli9/)
  (≥1,560 / 406): "look good and plausible… bugs"; "If you submit it, you own
  it"; "Good automated checks".
- [Ask HN: Are you using an agent orchestrator?](https://news.ycombinator.com/item?id=46993479)
  (41 / 60): "The bottleneck has not been how quickly you can generate
  reasonable code"; "the review pipeline should be heavier than the
  generation pipeline"; "sweet-spot is 2-3 agents".
- ["Parallel coding agents with tmux and Markdown specs"](https://news.ycombinator.com/item?id=47218318)
  (189 / 131): "The current bottleneck is validation."
- [Agent-Manager](https://news.ycombinator.com/item?id=49107749) (98 / 80):
  "can't tell what state any of them is in".

The buyer is a developer already running two or three Claude Code sessions
who has stopped trusting the output without checking it. The words they use
are **validation**, **review**, **automated checks**, and "can't tell what
state they're in". Nobody in these threads says **gate**. omatty's own
vocabulary ("the gate", "the card", "send-back") is internal until it is
explained.

## 5. Where omatty can post, under each venue's own rules

| Venue | Rule that matters (source) | Fit for omatty |
|---|---|---|
| **Show HN** | "Don't post quickly-generated one-offs; anybody can do that now." It must be tryable "without barriers such as signups or emails". Do not ask friends to vote or comment. ([showhn.html](https://news.ycombinator.com/showhn.html), read today) | **Eligible.** Tryable with `brew install`, no signup. The one-off rule is answered by seven releases, a public changelog and the gate. |
| **r/ClaudeAI** | Rule 7: promotion is encouraged if it was built with or for Claude *by you*, says how Claude helped, is free to try and says so, and keeps promotional language minimal. **Posts require OP karma above 50.** (archived rules page, 2026-08-23) | **Eligible if the posting account has >50 karma.** |
| **r/ClaudeCode** | Rule 5: simple project shares go in the weekly showcase thread. A standalone post must explain what you built, how Claude Code was used, and "what you learned". (archived 2026-08-05) | **Eligible**, as a what-I-learned post, not an announcement. |
| **r/commandline** | Rule 6: "We do not allow projects or software that interacts with generative AIs, including LLMs." Rule 4 also bans largely AI-generated projects. (archived 2026-08-11) | **Not eligible.** |
| **r/golang** | Rules not captured in this pass. | Unknown. One Go TUI for Claude tmux sessions scored ≥8 there on 2026-02-08. |
| **Claude Developers Discord** | Listed on [claude.com/community](https://claude.com/community). | Not assessed. |
| **Akita's community** | Named in #330. | Not assessed here. |

Base rate for HN: since 2026-01-01, 3,371 Show HN posts match the words
`claude code`, and 129 of them (3.8%) passed 50 points. (HN Algolia,
`tags=show_hn`, `numericFilters=created_at_i>1767225600[,points>50]`,
unquoted query. As an exact phrase the counts are 2,595 and 96, also 3.7%.) A Show HN is likely to fail. What the successful ones share,
from §2 and §3: a first-person outcome in the title (lazygit: "I made a tool
that made me faster at Git"), a link straight to the repository, and a
comparison ready before the "how is this different" comment.

## 6. Positioning check

- **omatty.com and the README lead with scope** ("multiple projects and
  multiple parallel Claude Code sessions in one window"). The threads are
  tired of that ("dozens of these"), and `2026-landscape.md` §7 shows agent
  view, herdr and Orca all claim it. **The pain the threads describe is
  judgement**, and that is what omatty's gate, review column and content
  anchoring are for. Today the pitch leads with its weakest distinction and
  holds back its strongest one.
- **The site's SPIN structure fits the demand evidence.** Situation and
  Problem are these threads' own complaint. The Compare table is the part
  #515 must fix before anyone reads it critically.
- **"Gate" is omatty's word, not the buyer's.** The buyer's words
  (validation, checks, review) belong in the first sentence. "Gate" can
  follow, once it has been explained.

## 7. Recommendations

Inputs to #330. None is a decision.

**G1. Fix #515 before any post.** Every launch thread in §3 asked "how is
this different". A comparison with a refutable claim in it, answered in the
thread by a stranger, costs more than having no comparison at all.

**G2. Put the recording in the README, and set the repository's homepage to
omatty.com.** Every README that grew shows the product moving in its first
screen. omatty's README has no image, and the GitHub page does not link to
the site that has the recording.

**G3. Lead with validation, not with scope.** One sentence in the buyer's own
words ("validation is the bottleneck"), then what omatty does about it. The
several-projects claim goes second or goes.

**G4. One Show HN, linked to the repository, titled as an outcome.** No
waitlist, no signup, no email. Prepare the comparison (G1) and one line each
for telemetry (none) and settings (untouched) before it goes up. Do not ask
anyone to vote.

**G5. Reddit as an account that has taken part first.** r/ClaudeAI needs
more than 50 karma to post, and r/ClaudeCode wants a what-I-learned post.
The honest what-I-learned for omatty is its gate on its own codebase:
building a TUI with Claude Code under 90% coverage, C.R.A.P. < 12 and a
regression test per bug. That is also the answer to "vibecoded". Skip
r/commandline, whose rules exclude omatty.

**G6. Answer herdr before a commenter raises it.** After R10's deep dive,
`comparison.md` should say plainly what herdr plus herdr-reviewr does that
omatty does not, and the reverse.

**G7. Measure against the base rate, not a hope.** #330's metric (issues
opened by someone else) is the right one. For stars, the camp's reference
points are fleet (54, no launch), ccmanager (≈270 in seven weeks after one
Reddit post) and vibe-kanban (≈3,200 in 20 days after one Show HN). They say
what a launch did for comparable tools. They do not promise what it will do
for omatty.

## 8. Sources

All opened 2026-09-27 unless an archive date is given.

- HN threads and comments: the item ids linked inline, read through
  `https://hn.algolia.com/api/v1/items/<id>`.
- Show HN guidelines: <https://news.ycombinator.com/showhn.html>.
- Subreddit rules, from Wayback captures: r/ClaudeAI (20260823233716),
  r/ClaudeCode (20260805054530), r/commandline (20260811152913).
- Reddit threads: scores via PullPush, marked ≥ as lower bounds.
- Star trajectories: Wayback captures of `github.com/<owner>/<repo>`, one
  point per capture.
- lazygit's retrospective: <https://jesseduffield.com/Lazygit-5-Years-On/>.
- vibe-kanban's shutdown notice: <https://www.vibekanban.com/blog/shutdown>
  (dated 2026-04-10; the README banner followed on 2026-04-24).
- READMEs: `gh api repos/<owner>/<repo>/readme`.
- omatty's own numbers: `gh api repos/WilsonSousajr/omatty`, and
  `gh issue list --state all` filtered by author.

Not used as sources: third-party "alternatives" and "best tools" pages, for
the reasons given in `2026-landscape.md`. Orca's own site (`onorca.dev`) is
cited only as evidence of how that vendor positions itself.
