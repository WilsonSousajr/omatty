package vcs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// The first of #338's three detections is the one the project itself declares:
// `linguist-generated` in .gitattributes. It is asked of git rather than
// guessed, because git is the only authority on what the attribute resolves
// to - it honours .gitattributes at every level, plus .git/info/attributes.
func TestAttr_ReportsTheFilesTheRepositoryDeclaresGenerated_issue338(t *testing.T) {
	dir := newRepo(t)
	writeIn(t, dir, ".gitattributes", "api/schema.pb.go linguist-generated=true\ndocs/* linguist-generated=true\n")
	writeIn(t, dir, "api/schema.pb.go", "package api\n")
	writeIn(t, dir, "docs/guide.md", "# guide\n")
	writeIn(t, dir, "internal/ui/model.go", "package ui\n")

	got, err := vcs.NewCLI().Attr(dir, "linguist-generated",
		[]string{"api/schema.pb.go", "docs/guide.md", "internal/ui/model.go"})
	if err != nil {
		t.Fatal(err)
	}

	if !got["api/schema.pb.go"] {
		t.Error("api/schema.pb.go is declared generated and was not reported")
	}
	if !got["docs/guide.md"] {
		t.Error("a glob in .gitattributes did not reach docs/guide.md")
	}
	if got["internal/ui/model.go"] {
		t.Error("model.go is not declared generated and was reported as such")
	}
}

// An unset attribute is not an error and not a yes. Most repositories declare
// nothing at all, and that path has to be quiet.
func TestAttr_ARepositoryThatDeclaresNothingReportsNothing_issue338(t *testing.T) {
	dir := newRepo(t)
	writeIn(t, dir, "a.go", "package a\n")

	got, err := vcs.NewCLI().Attr(dir, "linguist-generated", []string{"a.go"})
	if err != nil {
		t.Fatalf("a repository with no .gitattributes must not be an error: %v", err)
	}
	if got["a.go"] {
		t.Error("a.go was reported generated with nothing declaring it")
	}
}

// No paths means no git call: check-attr --stdin with empty input still forks,
// and a tree can be listed before any diff names a file.
func TestAttr_NoPathsAsksGitNothing_issue338(t *testing.T) {
	got, err := vcs.NewCLI().Attr(newRepo(t), "linguist-generated", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want an empty map", got)
	}
}

// A path with a space in it is one path. check-attr's output is
// `<path>: <attr>: <value>` per line, so a space is only ambiguous if the
// reader splits on whitespace - which is the bug this test exists to stop.
func TestAttr_APathWithASpaceStaysOnePath_issue338(t *testing.T) {
	dir := newRepo(t)
	writeIn(t, dir, ".gitattributes", "\"my docs/note one.md\" linguist-generated=true\n")
	writeIn(t, dir, "my docs/note one.md", "hi\n")

	got, err := vcs.NewCLI().Attr(dir, "linguist-generated", []string{"my docs/note one.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !got["my docs/note one.md"] {
		t.Errorf("got %v, want the spaced path reported generated", got)
	}
}

// writeIn puts a file in the repository at a relative path, making the
// directories above it. The package's own write takes an absolute path and
// makes nothing.
func writeIn(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, path, body)
}
