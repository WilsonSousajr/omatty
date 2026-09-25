package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// modelWithBothLists wires the two forge lists on one model, which is what a
// header needs: its counts come from both (#395).
func modelWithBothLists(t *testing.T) (*ui.Model, *FakeIssues, *FakePRs) {
	t.Helper()
	terms, _ := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	d.Clock = func() time.Time { return fi.Now }
	return ui.NewModel(d), fi, fp
}

// size gives the model a window, so the sidebar is drawn at all.
func size(m *ui.Model) { m.Update(tea.WindowSizeMsg{Width: 120, Height: 32}) }

// emptyHeaderLine is the sidebar line of withEmptyProject's third project: the
// omatty header, two cards, the api-svc header, its card. Computed from
// CardLines so it follows the renderer rather than being rechased (#230).
var emptyHeaderLine = 2 + 3*ui.CardLines()

// openIssues is n issues, numbered from 1, as a project's list.
func openIssues(n int) []forge.Issue {
	out := make([]forge.Issue, n)
	for i := range out {
		out[i] = forge.Issue{Number: i + 1, Title: fmt.Sprintf("issue %d", i+1)}
	}
	return out
}

// headerLine is the sidebar's line for a project's header row.
func headerLine(t *testing.T, m *ui.Model, line int) string {
	t.Helper()
	lines := frameLines(m)
	y := ui.SidebarTop() + line
	if y >= len(lines) {
		t.Fatalf("the frame has %d lines, no sidebar line %d", len(lines), line)
	}
	return lipgloss.NewStyle().MaxWidth(ui.SidebarWidth).Render(lines[y])
}

// omattyHeader is sidebar line 0: the first project's header row.
func omattyHeader(t *testing.T, m *ui.Model) string { return headerLine(t, m, 0) }

// The first question the tracker answers should not need a keypress: each
// project's header carries its own counts.
func TestHeaderRow_CarriesItsProjectsOpenCounts_issue395(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	fi.Lists["/p/omatty"] = openIssues(13)
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 1, State: forge.Open}, {Number: 2, State: forge.Open}}
	size(m)

	deliver(m, m.PollIssues())
	deliver(m, m.PollPRs())

	if got := omattyHeader(t, m); !strings.Contains(got, "13i 2p") {
		t.Errorf("header = %q, want it to carry 13i 2p", got)
	}
}

// Only what is open: the pull request map holds the finished ones too, for the
// card that shows "merged" (#310), and a header counting those would answer a
// question nobody asked.
func TestHeaderRow_CountsOnlyOpenPullRequests_issue395(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	fi.Lists["/p/omatty"] = openIssues(1)
	fp.Lists["/p/omatty"] = []forge.PR{
		{Number: 1, State: forge.Open}, {Number: 2, State: forge.Merged}, {Number: 3, State: forge.Closed},
	}
	size(m)

	deliver(m, m.PollIssues())
	deliver(m, m.PollPRs())

	if got := omattyHeader(t, m); !strings.Contains(got, "1i 1p") {
		t.Errorf("header = %q, want 1i 1p - the merged and closed ones are not open", got)
	}
}

// A project holding no session is never asked for pull requests (#310), so its
// header shows the count it has and says nothing about the one it does not.
func TestHeaderRow_AnEmptyProjectShowsItsIssuesAlone_issue395(t *testing.T) {
	m, fi, _ := modelWithBothLists(t)
	fi.Lists["/p/empty"] = openIssues(4)
	size(m)

	deliver(m, m.PollIssues())
	deliver(m, m.PollPRs())

	// The counts are the line's right edge, so the issue count being its last
	// visible token is exactly "no pull request count" - a check for the letter
	// would find the p in "empty".
	got := strings.TrimRight(ansi.Strip(headerLine(t, m, emptyHeaderLine)), "│ ")
	if !strings.HasSuffix(got, "4i") {
		t.Errorf("header = %q, want it to end with 4i and no pull request count", got)
	}
}

// Without gh, for a checkout that is not on GitHub, or before the first poll,
// the header is exactly what it was before this feature: an unknown count is
// never drawn as zero, the rule a card's "?" follows (Orca #18484).
func TestHeaderRow_UnknownCountsLeaveTheHeaderAsItWas_issue395(t *testing.T) {
	plain, _, _ := modelWithBothLists(t)
	size(plain)
	want := omattyHeader(t, plain) // polled by nobody

	for _, tt := range []struct {
		name string
		err  error
	}{
		{"gh is missing", forge.ErrNoGH},
		{"not a GitHub repository", fmt.Errorf("forge: no git remotes found: %w", forge.ErrNotGitHub)},
	} {
		m, fi, fp := modelWithBothLists(t)
		fi.Lists["/p/omatty"], fp.Lists["/p/omatty"] = openIssues(13), []forge.PR{{State: forge.Open}}
		for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
			fi.Errs[root], fp.Errs[root] = tt.err, tt.err
		}
		size(m)

		deliver(m, m.PollIssues())
		deliver(m, m.PollPRs())

		if got := omattyHeader(t, m); got != want {
			t.Errorf("%s: header = %q, want it unchanged at %q", tt.name, got, want)
		}
	}
}

// The counts come off before the name does. A count is worth reading; a header
// that has given its whole row to counts is not a header any more.
func TestHeaderRow_CountsComeOffBeforeTheName_issue395(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	fi.Lists["/p/omatty"] = openIssues(1)
	fp.Lists["/p/omatty"] = []forge.PR{{State: forge.Open}}
	size(m)
	deliver(m, m.PollIssues())
	deliver(m, m.PollPRs())
	if got := omattyHeader(t, m); !strings.Contains(got, "omatty") {
		t.Fatalf("header = %q, want the project's name", got)
	}

	if got := ui.FitCounts([]string{"99999i", "9999p"}); got != "99999i" {
		t.Errorf("FitCounts(99999i 9999p) = %q, want the issue count alone", got)
	}
	if got := ui.FitCounts([]string{"99999999999i"}); got != "" {
		t.Errorf("FitCounts of a count wider than the row = %q, want nothing: the name keeps it", got)
	}
}

// The header is still one sidebar-wide line, whatever it carries.
func TestHeaderRow_IsStillOneSidebarWideLine_issue395(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	fi.Lists["/p/omatty"] = openIssues(13)
	fp.Lists["/p/omatty"] = []forge.PR{{State: forge.Open}}
	size(m)
	deliver(m, m.PollIssues())
	deliver(m, m.PollPRs())

	if got := lipgloss.Width(omattyHeader(t, m)); got != ui.SidebarWidth {
		t.Errorf("header row is %d cells wide, want %d", got, ui.SidebarWidth)
	}
}
