package discover_test

import (
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/discover"
)

func threeCandidates() []discover.Candidate {
	now := time.Now()
	return []discover.Candidate{
		{Name: "omatty", Root: "/p/omatty", LastUsed: now},
		{Name: "api-svc", Root: "/work/api-svc", LastUsed: now.Add(-48 * time.Hour)},
		{Name: "notes", Root: "/p/notes", LastUsed: now.Add(-90 * 24 * time.Hour)},
	}
}

func TestList_NumbersEachCandidateAndSaysWhenItWasUsed(t *testing.T) {
	now := time.Now()

	got := discover.List(threeCandidates(), now)

	if len(got) != 3 {
		t.Fatalf("List() returned %d lines, want 3", len(got))
	}
	if !strings.HasPrefix(strings.TrimSpace(got[0]), "1") || !strings.Contains(got[0], "/p/omatty") {
		t.Errorf("first line = %q, want it numbered 1 and naming the root", got[0])
	}
	if !strings.Contains(got[0], "today") {
		t.Errorf("first line = %q, want it to say the repo was used today", got[0])
	}
	if !strings.Contains(got[2], "months ago") {
		t.Errorf("third line = %q, want a coarse age for a 90-day-old repo", got[2])
	}
}

func TestChoose_Selection(t *testing.T) {
	for _, tc := range []struct {
		name, selection string
		want            []string
	}{
		{"one", "2", []string{"api-svc"}},
		{"spaces", "1 3", []string{"omatty", "notes"}},
		{"commas", "1,3", []string{"omatty", "notes"}},
		{"mixed separators", "1, 2", []string{"omatty", "api-svc"}},
		{"all", "all", []string{"omatty", "api-svc", "notes"}},
		{"all is case-insensitive", "ALL", []string{"omatty", "api-svc", "notes"}},
		{"empty chooses nothing", "", nil},
		{"whitespace chooses nothing", "   ", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := discover.Choose(threeCandidates(), tc.selection)
			if err != nil {
				t.Fatalf("Choose(%q) error = %v, want nil", tc.selection, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Choose(%q) = %+v, want %v", tc.selection, got, tc.want)
			}
			for i, name := range tc.want {
				if got[i].Name != name {
					t.Errorf("Choose(%q)[%d] = %q, want %q", tc.selection, i, got[i].Name, name)
				}
			}
		})
	}
}

func TestChoose_RejectsSomethingThatIsNotANumber(t *testing.T) {
	_, err := discover.Choose(threeCandidates(), "omatty")

	if err == nil {
		t.Fatal("Choose() with a name returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "omatty") || !strings.Contains(err.Error(), "all") {
		t.Errorf("error %q does not name the offending field and the expected shape", err)
	}
}

func TestChoose_RejectsAnOutOfRangeNumber(t *testing.T) {
	for _, sel := range []string{"0", "4", "-1"} {
		_, err := discover.Choose(threeCandidates(), sel)
		if err == nil {
			t.Errorf("Choose(%q) returned nil, want an out-of-range error", sel)
			continue
		}
		if !strings.Contains(err.Error(), "3") {
			t.Errorf("Choose(%q) error %q does not say how many entries there are", sel, err)
		}
	}
}

// One bad field must reject the whole selection rather than registering the
// good half: a partial answer to "which of these" is not an answer.
func TestChoose_OneBadFieldRejectsTheWholeSelection(t *testing.T) {
	got, err := discover.Choose(threeCandidates(), "1 99")

	if err == nil {
		t.Fatal("Choose() with one bad field returned nil, want an error")
	}
	if got != nil {
		t.Errorf("Choose() returned %+v alongside an error, want nothing", got)
	}
}

func threeSessions() []discover.SessionCandidate {
	now := time.Now()
	return []discover.SessionCandidate{
		{ID: "s1", Title: "fix the parser", Dir: "/p/omatty", LastUsed: now},
		{ID: "s2", Title: "add a file tree", Dir: "/p/omatty", LastUsed: now.Add(-48 * time.Hour)},
		{ID: "s3", Title: "chase a flake", Dir: "/p/omatty", LastUsed: now.Add(-90 * 24 * time.Hour)},
	}
}

func TestListSessions_NumbersEachSessionAndSaysWhenItWasUsed_issue122(t *testing.T) {
	got := discover.ListSessions(threeSessions(), time.Now())

	if len(got) != 3 {
		t.Fatalf("ListSessions() returned %d lines, want 3", len(got))
	}
	if !strings.HasPrefix(strings.TrimSpace(got[0]), "1") || !strings.Contains(got[0], "fix the parser") {
		t.Errorf("first line = %q, want it numbered 1 and carrying the title", got[0])
	}
	if !strings.Contains(got[0], "today") {
		t.Errorf("first line = %q, want it to say the session was used today", got[0])
	}
}

// The same selection grammar as `omatty discover`, because an operator who has
// learnt one has learnt the other - and because two copies of it would drift.
func TestChooseSessions_Selection_issue122(t *testing.T) {
	for _, tc := range []struct {
		name, selection string
		want            []string
	}{
		{"one", "2", []string{"s2"}},
		{"spaces", "1 3", []string{"s1", "s3"}},
		{"commas", "1,3", []string{"s1", "s3"}},
		{"all", "all", []string{"s1", "s2", "s3"}},
		{"empty chooses nothing", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := discover.ChooseSessions(threeSessions(), tc.selection)
			if err != nil {
				t.Fatalf("ChooseSessions(%q) error = %v, want nil", tc.selection, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ChooseSessions(%q) = %+v, want %v", tc.selection, got, tc.want)
			}
			for i, id := range tc.want {
				if got[i].ID != id {
					t.Errorf("ChooseSessions(%q)[%d] = %q, want %q", tc.selection, i, got[i].ID, id)
				}
			}
		})
	}
}

func TestChooseSessions_RejectsSomethingThatIsNotANumber_issue122(t *testing.T) {
	_, err := discover.ChooseSessions(threeSessions(), "s1")

	if err == nil {
		t.Fatal("ChooseSessions() with an id returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "s1") || !strings.Contains(err.Error(), "all") {
		t.Errorf("error %q does not name the offending field and the expected shape", err)
	}
}

func TestChooseSessions_RejectsAnOutOfRangeNumber_issue122(t *testing.T) {
	_, err := discover.ChooseSessions(threeSessions(), "4")

	if err == nil {
		t.Fatal("ChooseSessions(\"4\") returned nil, want an out-of-range error")
	}
	if !strings.Contains(err.Error(), "3") {
		t.Errorf("error %q does not say how many entries there are", err)
	}
}
