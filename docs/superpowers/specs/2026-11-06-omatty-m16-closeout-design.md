# M16 close-out: #594, #585, #596 — design

Approved 2026-11-06 in a brainstorming session. Three issues M16 deferred to
Backlog when it shipped as v0.9.0. Nobody has hit them yet; the point is to
finish M16. One issue, one branch, one PR into `develop`, in this order:

1. **#594**: Bitbucket Cloud's 1,000 requests/hour limit can be reached with
   two projects, and no forge recognises a 429. Fixed for **every forge**
   (the operator's call).
2. **#585**: a self-managed forge served under a relative URL root
   (`https://host/gitlab`) is read at the wrong API base and with the wrong
   slug. Fixed by allowing **a path in a `[forge.hosts]` key** (the
   operator's call).
3. **#596**: Azure DevOps Server. Today it is refused *silently*, because
   `ErrNoForge` has no phrase. It will be read with a token of its own,
   bound to the instance. It is built and tested **against fakes only**,
   documented as not yet run against a real Server, and a follow-up issue
   is opened for that run (the operator's call).

Slice 3 starts after slice 2 merges, because both change token binding next
to each other in `tealogin.go`.

---

## Slice 1: #594, fewer requests, and a 429 waits (all forges)

### How it works today

- **The CI cache** (`internal/infra/forge/cicache.go`) keeps only
  `CIPassing` and has no clock. A repository with no CI therefore asks for
  statuses on every open pull request every poll, up to 30 of them.
- **The finished list** is read on every poll by every backend:
  `bitbucket.go` `listPRs`, GitLab's `glLists`, Gitea, Bitbucket DC, Azure,
  and gh's `listPRs`.
- **A 429** becomes an untyped error, both in `answerError` (`rest.go`) and
  in `statusError` (`fetch.go`). The TUI marks the project failed, the card
  shows `#N ?`, and the next poll asks again.

### Design

- **Clock.** `forge.Options` gains `Now func() time.Time`, defaulting to
  `time.Now`, and `cmd/omatty/wiring.go` wires it. The Router hands it to
  the CI cache and to the finished cache.
- **CI cache with lifetimes.** Each cached verdict is kept with an expiry
  time:

  | Verdict | Kept for |
  |---|---|
  | pass | for good (as today) |
  | failing | 5 minutes |
  | none | 5 minutes |
  | running | 1 minute |

  The cache key is still per head commit, so a push is seen at once. The
  rotation across polls is unchanged.
- **Finished list thinned.** A Router-level finished cache, keyed by
  project. A backend re-reads its finished list only in two cases:
  - the set of open changes (number and head) differs from the last read;
  - or 10 polls of that project have passed.

  A change leaving the open set is exactly when the finished list has
  something new, so nothing is lost. In the steady state, a project costs
  one list read per poll.
- **429 typed.** New `RateLimitError{Host string; Until time.Time}` in
  `internal/domain/forge/errors.go`.
  - **Where it comes from:** `answerError` and `statusError` return it on a
    429. They also return it on a 403 that `rateLimited` already recognises
    (GitHub's empty `X-Ratelimit-Remaining`, a `Retry-After`, or "rate limit"
    in the body).
  - **`Until`** is read from `Retry-After` (seconds, or an HTTP date). Failing
    that, it is read from `X-Ratelimit-Reset` (epoch seconds). Failing both,
    it is **zero**, meaning the forge said nothing.
  - **During a CI fill:** a CI ask that is rate-limited stops the rest of
    that fill, and the list call returns the error. Other ask errors are
    still swallowed, as today.
- **gh too.** gh's `pr list` and `issue list` give only stderr text. If that
  text carries gh's "rate limit" wording, it becomes a `RateLimitError` with
  a zero `Until`. The CLI-403-to-`AuthError` path checks for rate-limit
  wording first, so a rate limit never stops a project as an auth failure.
- **The TUI waits, per host.** The quota belongs to the account, so it is
  shared by every project on a host. The wait is therefore keyed by host:
  - **How long:** a known `Until` waits until then. A zero `Until` backs off
    5 minutes, then doubles each time up to 60 minutes. The first successful
    answer from any project on that host clears the wait and resets the
    backoff. (A 60 s wait would do nothing against a one-minute poll.)
  - **What is paused:** both polls, pull requests and issues, skip every
    project on a waiting host.
  - **What it is not:** it is not a stop. The stale lists stay, and the card
    keeps its `?`, because the verdict is still unknown.
  - **What the tracker says:** "rate-limited by <host> until 15:04" instead
    of an empty or failed state.
- **Acceptance.** Two no-CI Bitbucket Cloud projects, with 10 open pull
  requests each, are polled every minute for a simulated hour against a
  counting fake. Together they make fewer than 1,000 requests.

## Slice 2: #585, a root path in `[forge.hosts]` keys

### How it works today

- **The host table.** `Hosts` is `map[string]Kind`, keyed by a bare host,
  and `refuseBadForgeHosts` refuses a key with a path in it.
- **The repository path.** `Remote.Path` holds every segment after the
  host, so `https://host/gitlab/group/proj` reads as slug
  `gitlab/group/proj`.
- **The web root.** `webBase` is `scheme://host[:port]`, with no path.
- **Token binding.** `namesInstance` compares host and port only.

### Design

- **Config.** A key may be a bare host, or a host followed by root
  segments:

  ```toml
  [forge.hosts]
  "git.corp.example"         = "gitlab"
  "host.corp.example/gitlab" = "gitlab"
  "host.corp.example/gitea"  = "forgejo"
  ```

  Validation:
  - the host part matches today's `hostName`;
  - each root segment is a plain path segment: no scheme, port, query,
    empty segment, `.` or `..`;
  - two keys are duplicates when their hosts match case-insensitively and
    their roots match exactly.

  The README's `[forge.hosts]` section documents the shape.
- **Remote gets a root.** `Remote` gains `Root []string`. A new
  `Hosts.Resolve(Remote) (Remote, Kind, error)` replaces `Hosts.KindOf` at
  the router's naming step:
  - it picks the entry on the remote's host whose root is the **longest
    segment prefix** of `Path`;
  - it moves those segments from `Path` to `Root`, so `Slug()` no longer
    includes them;
  - with no rooted match it falls back to today's host lookup, built-in
    table included.
- **ssh remotes** carry no root in their path. If the host has a bare entry,
  that entry wins, as today. Otherwise:
  - with exactly one rooted entry, that root is used;
  - with several, omatty refuses aloud with
    `AmbiguousHostError{Host}`. It unwraps to `ErrNoForge`, so it stops the
    project for good, and its phrase reads "<host> serves several forges
    under different paths; clone over https so omatty can tell which".
- **Base URLs.** `webBase` appends the root, so every backend built on it
  follows: GitLab REST, Gitea REST, web links, Bitbucket DC and Azure.
- **No CLI for a rooted GitLab or Gitea.** glab's `--hostname` takes no
  path, so a rooted GitLab is read over REST only, and glab is not tried.
  With no `GITLAB_TOKEN`, or with the transport forced to the CLI, the
  project stops with `MissingToolError{TokenEnv: "GITLAB_TOKEN"}`, which can
  be retried like any missing token. A tea login matches a rooted remote
  only when its URL path equals the root.
- **Token binding.** `namesInstance` compares paths **only for a rooted
  remote**: the variable's URL path must then equal the root. A remote with
  no root keeps today's host-and-port rule exactly. So a Bitbucket DC ssh
  clone whose `BITBUCKET_DC_URL` carries `/bitbucket` keeps working, and so
  does a tea login under a subpath. `GITLAB_TOKEN` stays unbound, as today.
- **Bitbucket DC under a rooted key.** The root is split off before
  `dcCoordinates` runs, so the `/scm/KEY/slug` it reads is unchanged.

## Slice 3: #596, Azure DevOps Server

### How it works today

- **The refusal.** `pickAzure` refuses any host that is not Services with a
  plain `ErrNoForge`, so the project is stopped with no note.
- **The collection.** `azureCoordinates` already keeps the collection path
  in the base URL.
- **The API version.** `azureVersion = "api-version=7.1"` is a constant.

### Design

- **Token.** `AZURE_DEVOPS_SERVER_TOKEN` is a PAT, bound by
  `AZURE_DEVOPS_SERVER_URL`.
  - **What the URL may name:** the server (`https://host/tfs`, or
    `https://host` for an install at the root) or one collection
    (`https://host/tfs/DefaultCollection`).
  - **When it binds:** scheme, host and port match, and the URL's path is a
    **segment prefix** of the remote's web path (root plus collection). A
    server URL therefore covers every collection under it, and a collection
    URL covers only that one. The operator sets the token's reach by what
    they write.
  - **How it is sent:** Basic auth with an empty user, plus `noSignIn`.
  - **Refusals:** a plain-http instance gives
    `PlainHTTPError{TokenEnv: "AZURE_DEVOPS_SERVER_TOKEN"}`. No token, or a
    URL that does not bind, gives
    `MissingToolError{TokenEnv: "AZURE_DEVOPS_SERVER_TOKEN for <web root>"}`.
    This is Bitbucket DC's `dataCenter` pattern.
  - **What is never sent:** az's token and `AZURE_DEVOPS_EXT_PAT` never reach
    a Server. Services is untouched.
- **API version.** The version is negotiated, not configured.
  - `azBackend` gains a `version` field; Services stays at `7.1`.
  - For a Server, the first time an instance is picked, a probe reads
    `<collection>/_apis/projects?$top=1&api-version=V` for V = 7.1, 6.0 and
    5.0 in turn. It moves to the next version only on a 400.
  - The first version that answers 2xx is kept per instance on the Router,
    for the rest of the run.
  - A 401 or 403 during the probe is the usual `AuthError`.
  - Preview suffixes keep working, e.g. `6.0-preview.1`.
- **Unsupported.** If all three versions answer 400, the probe returns
  `UnsupportedError{Host, Reason}`. It stops the project, and the tracker
  says "Azure DevOps Server at <host> speaks no API version omatty reads
  (5.0 to 7.1)" instead of `?`. A retry may ask again, since the server can
  be upgraded.
- **Docs.** All marked "built against a fake, not yet run against a real
  Server":
  - the README forge table gets a Server row and its two variables;
  - the paragraph on token binding names the pair;
  - the README's "named but not read yet" line and the ROADMAP's #596 lines
    are updated.

  A follow-up issue (`test`, `M16`, `area:forge`, Backlog) tracks the real
  run.

---

## Calls made by Claude (for the operator to check)

- **Cache lifetimes:** running 1 minute, failing and none 5 minutes. The
  finished list is re-read when the open set changes, or every 10 polls.
- **Rate-limit scope:** a rate limit pauses the pull-request and issue polls
  of every project on that host. It is a wait with a note, not a stop. With
  no reset header, it backs off from 5 to 60 minutes.
- **gh:** gh's stderr "rate limit" counts as a rate limit, never as an auth
  stop.
- **The card during a wait** keeps its `?`; the tracker carries the note.
- **ssh with several roots:** an ssh remote whose host has several rooted
  entries is refused aloud, not guessed.
- **Path comparison:** `namesInstance` compares paths only for a rooted
  remote, so setups without a root are untouched.
- **No CLI under a root:** a rooted GitLab or Gitea uses REST, not the CLI.
  With no token, it is a missing-token stop.
- **`AZURE_DEVOPS_SERVER_URL`** may name a server or a collection, and binds
  by segment prefix.
- **Azure's API version** is probed once per instance, not configured.

## Out of scope

- Polling Bitbucket less often than other forges. Caching and backoff make
  it unnecessary.
- A per-host setting for the API version or the root, beyond the key's path.
- Running against a real Azure DevOps Server. That is the follow-up issue.
