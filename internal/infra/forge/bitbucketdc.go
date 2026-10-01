package forge

import (
	"context"
	"fmt"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"sort"
	"strconv"
	"strings"
	"time"
)

// bdcBackend is the Bitbucket Data Center (and Server) backend (#461): a
// different API from Cloud's, on the operator's own host, which [forge.hosts]
// names as bitbucket. REST only, and no issues - Data Center has no tracker;
// they are in Jira, which M16 leaves out.
type bdcBackend struct {
	f      fetcher
	remote Remote
	web    string // the instance's web root, context path and all
	key    string // the project key
	slug   string // the repository slug
	ci     *ciCache
	open   func(url string) error
}

// dcCoordinates is where a Data Center clone URL puts its project: an https
// clone is <context>/scm/<KEY>/<slug>.git, an ssh one <KEY>/<slug>.git. The
// context path, if any, is part of every web and API URL.
func dcCoordinates(r Remote) (contextPath, key, slug string, err error) {
	path := r.Path
	for i, seg := range path {
		// Only an http(s) clone has the /scm/ prefix: over ssh, a project
		// keyed SCM is a project (#461's review).
		if r.Scheme != "ssh" && strings.EqualFold(seg, "scm") {
			contextPath, path = strings.Join(path[:i], "/"), path[i+1:]
			break
		}
	}
	if len(path) != 2 {
		return "", "", "", fmt.Errorf("forge: %q is not a Bitbucket Data Center repository, want [<context>/scm/]<KEY>/<slug>: %w", r.Slug(), dforge.ErrNoForge)
	}
	return contextPath, path[0], path[1], nil
}

func (b bdcBackend) repo() string {
	return "api/1.0/projects/" + b.key + "/repos/" + b.slug
}

func (b bdcBackend) listPRs(ctx context.Context, _ string) ([]dforge.PR, error) {
	var all []dforge.PR
	for _, q := range []string{"state=OPEN&limit=100", "state=MERGED&limit=" + finishedWindow, "state=DECLINED&limit=" + finishedWindow} {
		page, err := getJSON[bbPage[dcPR]](ctx, b.f, b.repo()+"/pull-requests?"+q+"&order=NEWEST")
		if err != nil {
			return nil, repoMissing(err)
		}
		all = append(all, foldDCPRs(page.Values)...)
	}
	b.ci.fill(ctx, all, b.ciKey, b.buildCI)
	return all, nil
}

func (b bdcBackend) ciKey(pr dforge.PR) string {
	return b.remote.Host + "/" + b.key + "/" + b.slug + "#" + strconv.Itoa(pr.Number) + "@" + pr.Head
}

// buildCI is the worst of the head commit's builds, from the build-status API
// every CI server reports to.
func (b bdcBackend) buildCI(ctx context.Context, pr dforge.PR) (dforge.CIState, error) {
	builds, err := b.builds(ctx, pr.Head)
	if err != nil || len(builds) == 0 {
		return dforge.CINone, err
	}
	worst := dforge.CINone
	for _, s := range builds {
		worst = max(worst, bitbucketCI(s.State))
	}
	return worst, nil
}

// builds is a commit's builds from build-status/1.0, which Atlassian marks
// deprecated since 7.14 and caps at the last hundred; it is still the one
// endpoint that lists every build of a commit - the repository-scoped one
// needs each build's key (#461's review).
func (b bdcBackend) builds(ctx context.Context, head string) ([]dcBuild, error) {
	page, err := getJSON[bbPage[dcBuild]](ctx, b.f, "build-status/1.0/commits/"+head)
	return page.Values, err
}

func (b bdcBackend) listIssues(context.Context, string) ([]dforge.Issue, error) {
	return nil, dforge.ErrNoTracker
}

func (b bdcBackend) viewIssue(context.Context, string, int) (dforge.Detail, error) {
	return dforge.Detail{}, dforge.ErrNoTracker
}

func (b bdcBackend) viewPR(ctx context.Context, _ string, number int) (dforge.Detail, error) {
	path := b.repo() + "/pull-requests/" + strconv.Itoa(number)
	pr, err := getJSON[dcPR](ctx, b.f, path)
	if err != nil {
		return dforge.Detail{}, err
	}
	activity, err := getJSON[dcActivities](ctx, b.f, path+"/activities?limit=100")
	builds, _ := b.builds(ctx, pr.FromRef.LatestCommit)
	d := foldDetail(pr.flat(activity.Values, builds))
	// A feed with more pages, or none at all, is not the whole discussion
	// (#397; #461's review).
	d.Truncated = d.Truncated || err != nil || !activity.IsLastPage
	return d, nil
}

func (b bdcBackend) browse(_ context.Context, _ string, number int, pr bool) error {
	if !pr {
		return dforge.ErrNoTracker
	}
	return b.open(b.web + "/projects/" + b.key + "/repos/" + b.slug + "/pull-requests/" + strconv.Itoa(number))
}

// Data Center's answers, as it writes them: dates in epoch milliseconds.
type (
	dcUser struct {
		DisplayName string `json:"displayName"`
	}
	dcRef struct {
		DisplayID    string `json:"displayId"`
		LatestCommit string `json:"latestCommit"`
		Repository   struct {
			Slug    string `json:"slug"`
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
		} `json:"repository"`
	}
	dcPR struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		State       string `json:"state"`
		Draft       bool   `json:"draft"`
		CreatedDate int64  `json:"createdDate"`
		UpdatedDate int64  `json:"updatedDate"`
		ClosedDate  int64  `json:"closedDate"`
		FromRef     dcRef  `json:"fromRef"`
		ToRef       dcRef  `json:"toRef"`
		Author      struct {
			User dcUser `json:"user"`
		} `json:"author"`
		Properties struct {
			MergeResult struct {
				Outcome string `json:"outcome"`
			} `json:"mergeResult"`
		} `json:"properties"`
		Links struct {
			Self []struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	}
	dcComment struct {
		ID          int         `json:"id"`
		Text        string      `json:"text"`
		Author      dcUser      `json:"author"`
		CreatedDate int64       `json:"createdDate"`
		Replies     []dcComment `json:"comments"`
	}
	dcActivity struct {
		Action        string    `json:"action"`
		CommentAction string    `json:"commentAction"`
		Comment       dcComment `json:"comment"`
	}
	dcActivities struct {
		Values     []dcActivity `json:"values"`
		IsLastPage bool         `json:"isLastPage"`
	}
	dcBuild struct {
		State     string `json:"state"`
		Name      string `json:"name"`
		DateAdded int64  `json:"dateAdded"`
	}
)

// millis is an epoch-millisecond date, or the zero time for none.
func millis(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

func foldDCPRs(in []dcPR) []dforge.PR {
	out := make([]dforge.PR, len(in))
	for i, p := range in {
		out[i] = dforge.PR{
			Number: p.ID, Title: cleanLine(p.Title), Branch: cleanLine(p.FromRef.DisplayID), Base: cleanLine(p.ToRef.DisplayID),
			State: bbState(p.State), Head: p.FromRef.LatestCommit, Draft: p.Draft,
			Conflict: p.Properties.MergeResult.Outcome == "CONFLICTED",
			Fork:     p.FromRef.Repository.Project.Key+"/"+p.FromRef.Repository.Slug != p.ToRef.Repository.Project.Key+"/"+p.ToRef.Repository.Slug,
			Updated:  millis(p.UpdatedDate),
		}
		if out[i].State == dforge.Merged {
			out[i].MergedAt = millis(p.ClosedDate)
		}
	}
	return out
}

// flat is the pull request as gh's detail type: its comments from its
// activity, approvals and the rest left out, and each build a check.
func (p dcPR) flat(activity []dcActivity, builds []dcBuild) ghDetail {
	d := ghDetail{Number: p.ID, Title: p.Title, Body: p.Description, CreatedAt: millis(p.CreatedDate), Author: ghUser{Login: p.Author.User.DisplayName}}
	if len(p.Links.Self) > 0 {
		d.URL = p.Links.Self[0].Href
	}
	for _, c := range thread(activity) {
		d.Comments = append(d.Comments, ghComment{Author: ghUser{Login: c.Author.DisplayName}, Body: c.Text, CreatedAt: millis(c.CreatedDate)})
	}
	for _, s := range builds {
		d.Checks = append(d.Checks, statusCheck(s.Name, bitbucketCI(s.State), millis(s.DateAdded), time.Time{}))
	}
	return d
}

// thread is a pull request's comments as written (#461's review): each one an
// added or replied activity carries, with the replies nested under it, once
// each and oldest first. An edit or a deletion is not a comment, and a deleted
// comment is left out.
func thread(activity []dcActivity) []dcComment {
	deleted, seen := map[int]bool{}, map[int]bool{}
	for _, a := range activity {
		if a.CommentAction == "DELETED" {
			deleted[a.Comment.ID] = true
		}
	}
	var out []dcComment
	for _, a := range activity {
		if a.Action == "COMMENTED" && a.CommentAction != "EDITED" && a.CommentAction != "DELETED" {
			out = gather(out, a.Comment, seen, deleted)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedDate < out[j].CreatedDate })
	return out
}

// gather adds c and its replies, depth first, to out.
func gather(out []dcComment, c dcComment, seen, deleted map[int]bool) []dcComment {
	if !seen[c.ID] && !deleted[c.ID] {
		seen[c.ID] = true
		out = append(out, c)
	}
	for _, r := range c.Replies {
		out = gather(out, r, seen, deleted)
	}
	return out
}
