# Azure DevOps fixtures

Partly real, partly synthetic, and labelled so.

- **Real, scrubbed:** `prs-completed.json` and `threads.json` were recorded
  2026-09-28 from the public project `dev.azure.com/dnceng-public/public`
  (repository `dotnet-public-wiki`), anonymously. Every `uniqueName` and
  anything shaped like an email address is blanked.
- **Derived:** `prs-active.json` and `pr.json` are the real completed pull
  request's shape with its status, draft flag, merge status and branch
  changed, and one given a `forkSource`: the public project had no active
  pull request to record.
- **Synthetic:** `policy.json`, `wiql.json`, `workitems.json`,
  `workitem.json` and `comments.json` follow Microsoft's REST documentation
  for `policy/evaluations`, `wit/wiql`, `wit/workitems` and work item
  comments: an anonymous read of those answers with a sign-in redirect.

Replace them from a real organisation when one is available (#463). Until
then the support matrix names Azure DevOps untested.
