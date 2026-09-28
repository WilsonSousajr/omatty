# Bitbucket Data Center fixtures

**Synthetic.** No public Bitbucket Data Center instance was available to
record from on 2026-09-28, so these follow the shapes in Atlassian's REST
documentation for `/rest/api/1.0` (paged `values`, `fromRef`/`toRef`,
`properties.mergeResult`, epoch-millisecond dates, pull request
`activities`) and `/rest/build-status/1.0/commits/{sha}`. The project is a
made-up `OPS/platform` on `git.corp.example`.

Replace them from a real instance when one is available, and until then the
support matrix names Data Center as untested (#463, #465).
