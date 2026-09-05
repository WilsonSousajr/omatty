package discover_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/paths"
)

// sessionStore writes one slug directory for cwd holding the given transcripts,
// newest first by the order they are passed.
type fixture struct {
	ID     string
	Prompt string
	Used   time.Time
}

func sessionStore(t *testing.T, cwd string, fixtures ...fixture) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "projects")
	dir := filepath.Join(root, paths.TranscriptSlug(cwd))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		writeSession(t, dir, cwd, f)
	}
	return root
}

// writeSession writes one transcript with the leading records claude actually
// writes before anything the operator typed.
func writeSession(t *testing.T, dir, cwd string, f fixture) {
	t.Helper()
	lines := []string{
		`{"type":"queue-operation","operation":"add"}`,
		`{"type":"user","cwd":` + jsonString(t, cwd) +
			`,"message":{"role":"user","content":` + jsonString(t, "<command-name>/clear</command-name>") + `}}`,
	}
	if f.Prompt != "" {
		lines = append(lines, `{"type":"user","cwd":`+jsonString(t, cwd)+
			`,"message":{"role":"user","content":`+jsonString(t, f.Prompt)+`}}`)
	}
	path := filepath.Join(dir, f.ID+".jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	used := f.Used
	if used.IsZero() {
		used = time.Now()
	}
	if err := os.Chtimes(path, used, used); err != nil {
		t.Fatal(err)
	}
}

func jsonString(t *testing.T, s string) string {
	t.Helper()
	return quote(t, s)
}

func TestProposeSessions_ListsTheProjectsSessionsNewestFirst_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	now := time.Now()
	root := sessionStore(t, repo,
		fixture{ID: "older", Prompt: "fix the parser", Used: now.Add(-48 * time.Hour)},
		fixture{ID: "newer", Prompt: "add a file tree", Used: now})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatalf("ProposeSessions() error = %v, want nil", err)
	}
	if len(got) != 2 {
		t.Fatalf("candidates = %+v, want 2", got)
	}
	if got[0].ID != "newer" || got[1].ID != "older" {
		t.Errorf("order = %s, %s; want the newer session first", got[0].ID, got[1].ID)
	}
	if got[0].Dir != repo {
		t.Errorf("Dir = %q, want the cwd the transcript recorded (%q)", got[0].Dir, repo)
	}
}

// Adoption is per project, so a session belonging to another repository must not
// be offered: adopting it would register a session whose directory the project's
// diff and file tree know nothing about.
func TestProposeSessions_SkipsSessionsInAnotherProject_issue122(t *testing.T) {
	base := t.TempDir()
	mine, theirs := filepath.Join(base, "mine"), filepath.Join(base, "theirs")
	mkdirs(t, mine, theirs)
	root := filepath.Join(t.TempDir(), "projects")
	for _, pair := range []struct{ cwd, id string }{{mine, "keep"}, {theirs, "drop"}} {
		dir := filepath.Join(root, paths.TranscriptSlug(pair.cwd))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeSession(t, dir, pair.cwd, fixture{ID: pair.id, Prompt: "hello"})
	}
	git := &FakeGit{Repos: map[string]bool{mine: true, theirs: true}}

	got, err := discover.ProposeSessions(root, git, mine, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "keep" {
		t.Errorf("candidates = %+v, want only the session in the named project", got)
	}
}

// Offering a session omatty already tracks makes every row of a second adopt
// fail on commit with "already registered" and says nothing about why - the same
// reasoning that keeps registered roots out of project discovery (#91).
func TestProposeSessions_SkipsSessionsAlreadyRegistered_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo,
		fixture{ID: "known", Prompt: "one"},
		fixture{ID: "fresh", Prompt: "two"})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo,
		[]string{"known"})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "fresh" {
		t.Errorf("candidates = %+v, want only the session omatty does not already hold", got)
	}
}

// The uuid alone is unreadable, so the row is titled with what the session was
// actually about.
func TestProposeSessions_TitlesASessionFromItsFirstTypedPrompt_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: "fix the parser crash"})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "fix the parser crash" {
		t.Errorf("candidates = %+v, want the first prompt as the title", got)
	}
}

// The leading records of a real transcript are claude's own - a slash command,
// its output, a caveat - and none of them is something the operator typed.
// Titling a row with one would label every session "<command-name>/clear".
func TestProposeSessions_SkipsThePromptsClaudeWroteItself_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: "the real question"})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || strings.Contains(got[0].Title, "command-name") {
		t.Errorf("title = %q, want the typed prompt rather than claude's own record", got[0].Title)
	}
}

// Transcript content is untrusted (AGENTS.md, Security). It reaches a rendered
// row, so a prompt carrying newlines or escape sequences must not be able to
// draw outside its cell or move the cursor.
func TestProposeSessions_FlattensAnUntrustedPromptIntoOneLine_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo,
		fixture{ID: "s1", Prompt: "first line\n\x1b[2Jsecond\tline"})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	title := got[0].Title
	if strings.ContainsAny(title, "\n\r\t\x1b") {
		t.Errorf("title %q still carries control characters", title)
	}
}

// A row is one line of a narrow pane, so a pasted essay must not push the
// detail column off the screen.
func TestProposeSessions_TruncatesALongPrompt_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: strings.Repeat("long ", 200)})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(got[0].Title)); n > 64 {
		t.Errorf("title is %d runes, want it truncated to something a row can hold", n)
	}
}

// A session that never got a typed prompt still exists and is still adoptable,
// so it gets a title rather than an empty cell.
func TestProposeSessions_NamesASessionWithNoPromptByItsID_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "0a6b870b-9ccc-4f80-82cc-f9ede44b9123"})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title == "" {
		t.Errorf("candidates = %+v, want a non-empty title for a session with no prompt", got)
	}
}
