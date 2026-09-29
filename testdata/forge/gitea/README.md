# Gitea / Forgejo / Codeberg fixtures

Recorded 2026-09-28 from the public repository `codeberg.org/forgejo/forgejo`
through its REST API (`/api/v1`), anonymously, with small `limit` values.
`tea api <path>` prints the same bytes through the operator's login, which is
why the tea and REST transports share these files (#458, #459).

Every `email` field is blanked: Gitea puts one in each user object. The
comments are trimmed to three. `issue.json` is #2809, chosen because it has
comments; `pull.json`, `status.json` and `pull-comments.json` are #14587.

No token, no private repository, no email address.
