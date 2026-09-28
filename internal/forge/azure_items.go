package forge

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// openItemsWIQL is the open work items of the project, newest first. A query,
// not a board: which column an item sits in is the board, and boards are out
// of M16 for every forge.
const openItemsWIQL = "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project" +
	" AND [System.State] NOT IN ('Closed','Done','Removed') ORDER BY [System.CreatedDate] DESC"

// workItemFields is what the tracker shows of a work item.
const workItemFields = "System.Id,System.Title,System.Tags,System.AssignedTo,System.CreatedBy,System.ChangedDate"

// listIssues is the open work items: WIQL for their ids, then their fields in
// one batch - Azure takes at most two hundred ids, and #358's window is a
// hundred.
func (a azBackend) listIssues(ctx context.Context, _ string) ([]Issue, error) {
	body, _ := json.Marshal(map[string]string{"query": openItemsWIQL})
	raw, err := a.rest.post(ctx, a.api("wit/wiql?$top=100"), bytes.NewReader(body), a.auth, a.env)
	if err != nil {
		return nil, repoMissing(err)
	}
	var found azWIQL
	if err := json.Unmarshal(raw, &found); err != nil || len(found.WorkItems) == 0 {
		return nil, err
	}
	items, err := azGet[azList[azWorkItem]](ctx, a, a.api("wit/workitems?ids="+found.ids()+"&fields="+workItemFields))
	if err != nil {
		return nil, err
	}
	return a.foldItems(items.Value), nil
}

func (a azBackend) viewIssue(ctx context.Context, _ string, number int) (Detail, error) {
	item, err := azGet[azWorkItem](ctx, a, a.api("wit/workitems/"+strconv.Itoa(number)))
	if err != nil {
		return Detail{}, err
	}
	comments, _ := azGet[azComments](ctx, a, a.api("wit/workItems/"+strconv.Itoa(number)+"/comments")+"-preview.4")
	return foldDetail(item.flat(a.itemURL(number), comments.Comments)), nil
}

func (a azBackend) viewPR(ctx context.Context, _ string, number int) (Detail, error) {
	pr, err := azGet[azPR](ctx, a, a.repoAPI("pullrequests/"+strconv.Itoa(number)))
	if err != nil {
		return Detail{}, err
	}
	threads, _ := azGet[azList[azThread]](ctx, a, a.repoAPI("pullRequests/"+strconv.Itoa(number)+"/threads"))
	builds, _ := a.builds(ctx, number)
	return foldDetail(pr.flat(a.prURL(number), threads.Value, builds)), nil
}

func (a azBackend) browse(_ context.Context, _ string, number int, pr bool) error {
	if pr {
		return a.open(a.prURL(number))
	}
	return a.open(a.itemURL(number))
}

func (a azBackend) prURL(number int) string {
	return a.base + "/" + a.project + "/_git/" + a.repo + "/pullrequest/" + strconv.Itoa(number)
}

func (a azBackend) itemURL(number int) string {
	return a.base + "/" + a.project + "/_workitems/edit/" + strconv.Itoa(number)
}

func (w azWIQL) ids() string {
	ids := make([]string, 0, len(w.WorkItems))
	for _, it := range w.WorkItems {
		ids = append(ids, strconv.Itoa(it.ID))
	}
	return strings.Join(ids, ",")
}
