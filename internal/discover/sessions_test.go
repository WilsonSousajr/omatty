package discover_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/paths"
)

// sessionStore writes one slug directory for cwd holding the given transcripts,
// newest first by the order they are passed.
type fixture struct {
	ID     string
	Prompt string
	// RawContent is written as the message content verbatim, for the shapes a
	// plain string cannot express - a prompt carrying an attachment is a list
	// of blocks (#62). Prompt is ignored when it is set.
	RawContent string
	Used       time.Time
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
	if content := promptContent(t, f); content != "" {
		lines = append(lines, `{"type":"user","cwd":`+jsonString(t, cwd)+
			`,"message":{"role":"user","content":`+content+`}}`)
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

// promptContent is the JSON a fixture's prompt is written as, or "" when it has
// none.
func promptContent(t *testing.T, f fixture) string {
	t.Helper()
	if f.RawContent != "" {
		return f.RawContent
	}
	if f.Prompt == "" {
		return ""
	}
	return jsonString(t, f.Prompt)
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

// Regression, issue #122: registry.AddProject stores what `rev-parse
// --show-toplevel` returns, and inside a linked worktree that is the worktree
// itself - while resolveRoot folds a worktree back to the repository it was
// forked from. The two were compared directly, so they never matched and
// adoption reported "no unregistered sessions in this project" forever, for
// the sessions that ran in that very directory.
//
// The fake's RepoRoot and MainCheckout must disagree for this to be visible at
// all, which is why no earlier test could see it (#91).
func TestProposeSessions_FindsThemForAProjectRegisteredInsideAWorktree_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	wt := filepath.Join(base, "wt", "fix")
	mkdirs(t, repo, wt)
	root := sessionStore(t, wt, fixture{ID: "s1", Prompt: "fix the parser"})
	git := &FakeGit{Repos: map[string]bool{repo: true}, Worktrees: map[string]string{wt: repo}}

	// wt, not repo: that is the Root AddProject would have stored for a project
	// registered from inside the worktree.
	got, err := discover.ProposeSessions(root, git, wt, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("candidates = %+v, want the session that ran in the worktree", got)
	}
	if got[0].Dir != wt {
		t.Errorf("Dir = %q, want the worktree %q: that is where --resume has to run", got[0].Dir, wt)
	}
}

// Regression, issue #122: TranscriptSlug maps every non-alphanumeric to '-', so
// "<base>/b-c" and "<base>/b/c" produce the same slug and claude writes both
// directories' transcripts into one directory. One cwd was read for the whole
// directory and stamped onto every transcript in it, so a session that ran in
// one repository was offered under the other's project and registered with a
// Dir that is not where it ran - the directory `claude --resume` is launched in.
func TestProposeSessions_KeepsCollidingSlugDirectoriesApart_issue122(t *testing.T) {
	base := t.TempDir()
	dashed, nested := filepath.Join(base, "b-c"), filepath.Join(base, "b", "c")
	mkdirs(t, dashed, nested)
	if paths.TranscriptSlug(dashed) != paths.TranscriptSlug(nested) {
		t.Fatalf("the fixture does not collide: %q vs %q",
			paths.TranscriptSlug(dashed), paths.TranscriptSlug(nested))
	}
	root := sessionStore(t, dashed, fixture{ID: "in-dashed", Prompt: "the dashed one"})
	writeSession(t, filepath.Join(root, paths.TranscriptSlug(nested)), nested,
		fixture{ID: "in-nested", Prompt: "the nested one"})
	git := &FakeGit{Repos: map[string]bool{dashed: true, nested: true}}

	got, err := discover.ProposeSessions(root, git, nested, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "in-nested" {
		t.Fatalf("candidates = %+v, want only the session that ran in %q", got, nested)
	}
	if got[0].Dir != nested {
		t.Errorf("Dir = %q, want %q, which is where that session actually ran", got[0].Dir, nested)
	}
}

// Regression, issue #122: a prompt carrying an attachment is a list of blocks
// rather than a string (#62). Only the string form was read, so the prompt was
// skipped and the row fell back to the raw uuid - the unreadable cell titleOf
// exists to avoid.
func TestProposeSessions_TitlesASessionWhoseFirstPromptCarriesAnAttachment_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{
		ID:         "s1",
		RawContent: `[{"type":"text","text":"look at this screenshot"},{"type":"image","source":{}}]`,
	})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %+v, want one", got)
	}
	if got[0].Title != "look at this screenshot" {
		t.Errorf("title = %q, want the text block of a prompt that carried an attachment", got[0].Title)
	}
}

// unicode.IsControl covers only Cc, so the Cf format characters passed through
// into a title that AdoptSession persists to state.json and every later run
// draws in a sidebar row. U+202E reverses the visible order of everything after
// it (#122).
func TestProposeSessions_StripsFormatCharactersFromATitle_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: "fix \u202Ethe parser"})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(got[0].Title, '\u202E') {
		t.Errorf("title %q still carries the right-to-left override", got[0].Title)
	}
}

// The bound exists because a row is one line of a fifty-column pane, so it has
// to be a bound on columns. Sixty CJK runes is a hundred and twenty of them, so
// counting runes let through exactly the overflow it was written to prevent
// (#122).
func TestProposeSessions_TruncatesATitleToDisplayCellsNotRunes_issue122(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: strings.Repeat("界", 60)})

	got, err := discover.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil)

	if err != nil {
		t.Fatal(err)
	}
	if w := runewidth.StringWidth(got[0].Title); w > 60 {
		t.Errorf("title occupies %d columns, want at most 60: the pane is fifty wide", w)
	}
}
