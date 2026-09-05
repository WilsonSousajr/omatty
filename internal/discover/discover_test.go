package discover_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/paths"
)

// FakeGit answers for a fixed set of directories: anything under a known
// worktree resolves to its parent, anything else in repos resolves to itself,
// and everything else is not a repository. A named type, per AGENTS.md.
type FakeGit struct {
	// Repos are directories that are main checkouts.
	Repos map[string]bool
	// Worktrees maps a linked worktree to the repository it belongs to.
	Worktrees map[string]string
	// Calls records the directories MainCheckout was asked about.
	Calls []string
}

func (f *FakeGit) RepoRoot(dir string) (string, error) {
	if f.Repos[dir] {
		return dir, nil
	}
	if _, ok := f.Worktrees[dir]; ok {
		return dir, nil // git returns the worktree itself here - the whole problem
	}
	return "", fmt.Errorf("not a git repository: %q", dir)
}

func (f *FakeGit) MainCheckout(dir string) (string, error) {
	f.Calls = append(f.Calls, dir)
	if parent, ok := f.Worktrees[dir]; ok {
		return parent, nil
	}
	if f.Repos[dir] {
		return dir, nil
	}
	return "", fmt.Errorf("not a git repository: %q", dir)
}

// store builds a transcript store: one slug directory per cwd, each holding
// one transcript that names it. Fixtures are written here rather than copied
// from a real store, which would carry the user's code and prompts into the
// repository (AGENTS.md, Security).
func store(t *testing.T, cwds ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "projects")
	for i, cwd := range cwds {
		writeTranscript(t, root, cwd, time.Now().Add(-time.Duration(i)*time.Hour))
	}
	return root
}

// writeTranscript writes one transcript for cwd, with the leading records that
// carry no cwd, mirroring what claude actually writes.
func writeTranscript(t *testing.T, root, cwd string, modTime time.Time) string {
	t.Helper()
	dir := filepath.Join(root, paths.TranscriptSlug(cwd))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	lines := []string{
		`{"type":"queue-operation","operation":"add"}`,
		`{"type":"agent-setting","setting":"model"}`,
		`{"type":"user","cwd":` + quote(t, cwd) + `,"message":{"role":"user","content":"hi"}}`,
	}
	path := filepath.Join(dir, "0a6b870b.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	return path
}

func quote(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// mkdirs creates each directory so the "still on disk" filter passes.
func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPropose_ReadsTheCwdOutOfEachTranscript(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "omatty")
	mkdirs(t, repo)
	root := store(t, repo)

	got, err := discover.Propose(root, &FakeGit{Repos: map[string]bool{repo: true}})

	if err != nil {
		t.Fatalf("Propose() error = %v, want nil", err)
	}
	if len(got) != 1 || got[0].Root != repo || got[0].Name != "omatty" {
		t.Errorf("candidates = %+v, want one named omatty at %q", got, repo)
	}
}

// A third of a real store points at directories that no longer exist.
func TestPropose_SkipsADirectoryThatIsGone(t *testing.T) {
	base := t.TempDir()
	alive, dead := filepath.Join(base, "alive"), filepath.Join(base, "deleted")
	mkdirs(t, alive)
	root := store(t, alive, dead)

	got, err := discover.Propose(root, &FakeGit{Repos: map[string]bool{alive: true, dead: true}})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Root != alive {
		t.Errorf("candidates = %+v, want only the directory still on disk", got)
	}
}

func TestPropose_SkipsADirectoryThatIsNotARepository(t *testing.T) {
	base := t.TempDir()
	repo, plain := filepath.Join(base, "repo"), filepath.Join(base, "notes")
	mkdirs(t, repo, plain)
	root := store(t, repo, plain)

	got, err := discover.Propose(root, &FakeGit{Repos: map[string]bool{repo: true}})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Root != repo {
		t.Errorf("candidates = %+v, want only the git repository", got)
	}
}

// The reason MainCheckout exists: git reports a linked worktree as its own top
// level, so a naive resolve registers each worktree as a project.
func TestPropose_FoldsAWorktreeIntoItsParent_issue91(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "harness")
	wt := filepath.Join(base, "harness-impl")
	mkdirs(t, repo, wt)
	root := store(t, repo, wt)
	git := &FakeGit{Repos: map[string]bool{repo: true}, Worktrees: map[string]string{wt: repo}}

	got, err := discover.Propose(root, git)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Root != repo {
		t.Errorf("candidates = %+v, want the parent repository once, not the worktree too", got)
	}
}

// Newest first: the list reads as a history of what you have been working on.
func TestPropose_OrdersByMostRecentlyUsed(t *testing.T) {
	base := t.TempDir()
	older, newer := filepath.Join(base, "older"), filepath.Join(base, "newer")
	mkdirs(t, older, newer)
	root := filepath.Join(t.TempDir(), "projects")
	writeTranscript(t, root, older, time.Now().Add(-48*time.Hour))
	writeTranscript(t, root, newer, time.Now())

	got, err := discover.Propose(root, &FakeGit{Repos: map[string]bool{older: true, newer: true}})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Root != newer || got[1].Root != older {
		t.Errorf("candidates = %+v, want the newer one first", got)
	}
}

// Two slug directories can resolve to one repository - a worktree and its
// parent, or a path that moved. The candidate keeps the more recent time, so
// ordering reflects the last time the repository was touched at all.
func TestPropose_DeduplicatesKeepingTheMostRecentTime(t *testing.T) {
	base := t.TempDir()
	repo, wt := filepath.Join(base, "repo"), filepath.Join(base, "wt")
	mkdirs(t, repo, wt)
	root := filepath.Join(t.TempDir(), "projects")
	writeTranscript(t, root, repo, time.Now().Add(-48*time.Hour))
	recent := time.Now().Truncate(time.Second)
	writeTranscript(t, root, wt, recent)
	git := &FakeGit{Repos: map[string]bool{repo: true}, Worktrees: map[string]string{wt: repo}}

	got, err := discover.Propose(root, git)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %+v, want one", got)
	}
	if !got[0].LastUsed.Equal(recent) {
		t.Errorf("LastUsed = %v, want the more recent %v", got[0].LastUsed, recent)
	}
}

// A transcript whose head holds no cwd is skipped, not guessed at: the slug
// cannot be reversed.
func TestPropose_SkipsATranscriptWithNoCwd(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projects")
	dir := filepath.Join(root, "-some-slug")
	mkdirs(t, dir)
	body := strings.Repeat(`{"type":"queue-operation"}`+"\n", 5)
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := discover.Propose(root, &FakeGit{})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("candidates = %+v, want none from a transcript with no cwd", got)
	}
}

// The read is bounded (issue #64): a cwd buried past the line cap is not
// found, and the reader must not sit there consuming an enormous file.
func TestPropose_StopsReadingAfterTheLineCap_issue64(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "buried")
	mkdirs(t, repo)
	root := filepath.Join(t.TempDir(), "projects")
	dir := filepath.Join(root, "-buried")
	mkdirs(t, dir)
	lines := strings.Repeat(`{"type":"queue-operation"}`+"\n", 200)
	lines += `{"type":"user","cwd":` + quote(t, repo) + `}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := discover.Propose(root, &FakeGit{Repos: map[string]bool{repo: true}})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("candidates = %+v, want none: the cwd is past the bounded head read", got)
	}
}

// A malformed line must not abort the scan: the next record may still carry
// the cwd, and transcript content is untrusted input.
func TestPropose_SkipsMalformedLinesAndKeepsReading(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	mkdirs(t, repo)
	root := filepath.Join(t.TempDir(), "projects")
	dir := filepath.Join(root, "-repo")
	mkdirs(t, dir)
	body := "not json at all\n{\"type\":\"user\",\"cwd\":" + quote(t, repo) + "}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := discover.Propose(root, &FakeGit{Repos: map[string]bool{repo: true}})

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Root != repo {
		t.Errorf("candidates = %+v, want the repository named after the malformed line", got)
	}
}

func TestPropose_MissingStoreNamesIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-store")

	_, err := discover.Propose(missing, &FakeGit{})

	if err == nil {
		t.Fatal("Propose() on a missing store returned nil, want an error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the offending store", err)
	}
}

// An empty store is not an error: a machine that has never run claude has
// nothing to propose, which is a valid answer.
func TestPropose_EmptyStoreIsNotAnError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "projects")
	mkdirs(t, root)

	got, err := discover.Propose(root, &FakeGit{})

	if err != nil {
		t.Fatalf("Propose() on an empty store error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("candidates = %+v, want none", got)
	}
}
