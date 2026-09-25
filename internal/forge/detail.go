// One item in full (#397): an issue's or a pull request's body and comments,
// read only when the operator opens it.
//
// Never part of a list poll. #358's lesson is that a per-item call inside a
// list is what trips GitHub's secondary rate limit, so this is one call for one
// item, made on a keypress and cached by whoever asked.

package forge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// detailFields is everything FoldDetail reads. One field set for both kinds: an
// issue and a pull request answer to the same names, so one fold serves both.
const detailFields = "number,title,body,author,createdAt,comments,url"

// DetailMax bounds an item's text. Someone else's issue can be any size, and a
// pane must not hold megabytes of it - the preview's argument (#24), at the same
// 64 KiB the preview draws plain above.
const DetailMax = 64 << 10

// Comment is one comment on an item.
type Comment struct {
	Author string
	Body   string
	At     time.Time
}

// Detail is one issue or pull request in full.
type Detail struct {
	Number   int
	Title    string
	Author   string
	Body     string
	Comments []Comment
	URL      string
	Created  time.Time
	// Truncated says the body or the comments were cut at DetailMax. A short
	// item must never read as the whole one.
	Truncated bool
}

// ghDetail is `gh <kind> view --json` with the fields detailFields asks for.
type ghDetail struct {
	Number    int         `json:"number"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	Author    ghUser      `json:"author"`
	CreatedAt time.Time   `json:"createdAt"`
	Comments  []ghComment `json:"comments"`
	URL       string      `json:"url"`
}

type ghComment struct {
	Author    ghUser    `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

// ViewIssue is one issue in full.
//
//	item, err := forge.NewCLI().ViewIssue("/p/omatty", 397)
func (c *CLI) ViewIssue(repoRoot string, number int) (Detail, error) {
	return c.view(repoRoot, "issue", number)
}

// ViewPR is one pull request in full. A separate call rather than a guess: gh
// has two subcommands, and the tracker knows which list its row came from.
func (c *CLI) ViewPR(repoRoot string, number int) (Detail, error) {
	return c.view(repoRoot, "pr", number)
}

func (c *CLI) view(repoRoot, kind string, number int) (Detail, error) {
	ctx, cancel, err := c.bounded()
	if err != nil {
		return Detail{}, err
	}
	defer cancel()
	out, err := c.run(ctx, repoRoot, kind, "view", strconv.Itoa(number), "--json", detailFields)
	if err != nil {
		return Detail{}, err
	}
	return FoldDetail(out)
}

// FoldDetail turns `gh <kind> view --json` output into a Detail, bounded at
// DetailMax.
//
//	item, err := forge.FoldDetail(out)
func FoldDetail(raw []byte) (Detail, error) {
	var in ghDetail
	if err := json.Unmarshal(raw, &in); err != nil {
		return Detail{}, fmt.Errorf("forge: reading gh's view of an item: %w", err)
	}
	body, left := bound(in.Body, DetailMax)
	comments, dropped := foldComments(in.Comments, left)
	return Detail{
		Number: in.Number, Title: in.Title, Author: in.Author.Login,
		Body: body, Comments: comments, URL: in.URL, Created: in.CreatedAt,
		Truncated: dropped || len(body) < len(in.Body),
	}, nil
}

// foldComments takes comments while budget lasts, and says whether any was
// dropped. Whole comments rather than a cut one: half a comment attributed to
// its author is worse than a missing one the view admits to.
func foldComments(in []ghComment, budget int) ([]Comment, bool) {
	out := make([]Comment, 0, len(in))
	for _, c := range in {
		if len(c.Body) > budget {
			return out, true
		}
		budget -= len(c.Body)
		out = append(out, Comment{Author: c.Author.Login, Body: c.Body, At: c.CreatedAt})
	}
	return out, false
}

// bound cuts s to budget bytes and returns what is left of it.
func bound(s string, budget int) (string, int) {
	if len(s) <= budget {
		return s, budget - len(s)
	}
	return s[:budget], 0
}

// Browse opens one issue or pull request in the operator's own browser, through
// their own gh: `gh browse <number>` in repoRoot, which resolves either kind.
//
// Read-only on the forge - it writes nothing and opens a page the operator asked
// for - and it is here rather than behind an `open`/`xdg-open` of its own because
// gh is already this package's business (invariant 4 in spirit).
//
//	err := forge.NewCLI().Browse("/p/omatty", 399)
func (c *CLI) Browse(repoRoot string, number int) error {
	ctx, cancel, err := c.bounded()
	if err != nil {
		return err
	}
	defer cancel()
	_, err = c.run(ctx, repoRoot, "browse", strconv.Itoa(number))
	return err
}
