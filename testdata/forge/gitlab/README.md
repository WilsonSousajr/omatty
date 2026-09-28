# GitLab fixtures

Recorded 2026-09-28 from the public project `gitlab.com/gitlab-org/cli`
through its REST API (`/api/v4`), anonymously, with small `per_page` values so
the files stay readable. `glab api <path>` prints the same bytes, which is why
the CLI and REST transports share these files (#454, #455).

`mr-notes.json` and `issue-notes.json` are **synthetic**: GitLab answers an
anonymous notes request with 401 even on a public project. They follow the
documented note schema (`id`, `body`, `author.username`, `created_at`,
`system`) and include a system note, which omatty skips. They are stored newest
first, as GitLab answers the `sort=desc` omatty asks for. Replace them from a
real authenticated run when one is available (#463).

No token, no private project, no email address.
