package forge

// The GitHub HTTP backend reads through GraphQL (#462). GitHub's REST pull
// request list carries no merge state, no review decision and no checks, so
// REST would cost two or three calls per open pull request every minute -
// past the operator's 5,000-an-hour budget on a busy repository - while one
// GraphQL query answers the whole list with exactly the fields gh's own
// `pr list --json` reads, from the same endpoint gh itself calls. The answers
// are reshaped into gh's own JSON types and folded by gh's own folds, which is
// what makes the two transports indistinguishable to the UI.

// checkFields is a pull request's checks as gh's statusCheckRollup reports
// them: the last commit's rollup, each context a CheckRun or a StatusContext.
const checkFields = `commits(last: 1) { nodes { commit { statusCheckRollup { contexts(first: 100) { nodes {
	__typename
	... on CheckRun { name status conclusion startedAt completedAt }
	... on StatusContext { context state }
} } } } } }`

// prsQuery is ListPRs in one call: the open pull requests with everything a
// card and a tracker row read (openFields), and the recently finished ones
// with what a finished card reads (finishedFields). Both are ordered as gh
// orders `pr list`, newest first by creation.
const prsQuery = `query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) {
	open: pullRequests(states: OPEN, first: 100, orderBy: {field: CREATED_AT, direction: DESC}) { nodes {
		number title headRefName baseRefName headRefOid isCrossRepository state isDraft updatedAt
		mergeStateStatus reviewDecision ` + checkFields + `
	} }
	finished: pullRequests(states: [CLOSED, MERGED], first: 30, orderBy: {field: CREATED_AT, direction: DESC}) { nodes {
		number headRefName baseRefName headRefOid isCrossRepository state mergedAt
	} }
} }`

// issuesQuery is ListIssues: the open issues with issueFields.
const issuesQuery = `query($owner: String!, $name: String!) { repository(owner: $owner, name: $name) {
	issues(states: OPEN, first: 100, orderBy: {field: CREATED_AT, direction: DESC}) { nodes {
		number title url updatedAt author { login }
		labels(first: 20) { nodes { name } } assignees(first: 1) { nodes { login } }
	} }
} }`

// itemFields is one item in full, detailFields; a pull request adds its checks.
const itemFields = `number title body url createdAt author { login }
	comments(first: 100) { totalCount nodes { author { login } body createdAt } }`

// itemQuery reads either kind by number: GitHub numbers issues and pull
// requests in one sequence, so the number alone names the item.
const itemQuery = `query($owner: String!, $name: String!, $number: Int!) { repository(owner: $owner, name: $name) {
	issueOrPullRequest(number: $number) {
		... on Issue { ` + itemFields + ` }
		... on PullRequest { ` + itemFields + ` ` + checkFields + ` }
	}
} }`
