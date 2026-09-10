package review_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

func TestReadPreview_SplitsTextIntoLines_issue24(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n\nfunc F() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := review.ReadPreview(dir, "a.go")

	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(p.Lines, "|") != "package a||func F() {}" || p.Binary || p.Truncated {
		t.Errorf("Preview = %+v", p)
	}
	if p.Path != "a.go" {
		t.Errorf("Path = %q, want the relative path a.go", p.Path)
	}
}

func TestReadPreview_FlagsBinaries_issue24(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "img"), []byte("PNG\x00\x01"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := review.ReadPreview(dir, "img")
	if err != nil || !p.Binary || len(p.Lines) != 0 {
		t.Errorf("Preview = %+v, %v; want Binary with no lines", p, err)
	}
}

func TestReadPreview_TruncatesLargeFiles_issue24(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("0123456789abcdef\n", 20000) // 340 KiB
	if err := os.WriteFile(filepath.Join(dir, "big"), []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := review.ReadPreview(dir, "big")
	if err != nil || !p.Truncated || len(p.Lines) >= 20000 {
		t.Errorf("Preview truncated=%v lines=%d err=%v; want a truncated, shorter preview", p.Truncated, len(p.Lines), err)
	}
	if last := p.Lines[len(p.Lines)-1]; last != "0123456789abcdef" {
		t.Errorf("last line = %q, want a whole line: the cut lands on a newline", last)
	}
}

func TestReadPreview_RefusesToLeaveTheDirectory_issue24(t *testing.T) {
	for _, rel := range []string{"../etc/passwd", "/etc/passwd", ".."} {
		if _, err := review.ReadPreview(t.TempDir(), rel); err == nil || !strings.Contains(err.Error(), rel) {
			t.Errorf("ReadPreview(%q) error = %v, want a refusal naming the path", rel, err)
		}
	}
}

func TestReadPreview_MissingFileNamesIt(t *testing.T) {
	_, err := review.ReadPreview(t.TempDir(), "nope.txt")
	if err == nil || !strings.Contains(err.Error(), "nope.txt") {
		t.Errorf("error = %v, want one naming nope.txt", err)
	}
}

func TestReadPreview_EmptyFileIsOneEmptyLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := review.ReadPreview(dir, "empty")
	if err != nil || p.Binary || p.Truncated {
		t.Errorf("Preview = %+v, %v; want a plain empty preview", p, err)
	}
}
