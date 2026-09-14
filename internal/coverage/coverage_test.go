package coverage_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/coverage"
)

const omattyModule = "github.com/WilsonSousajr/omatty"

// Recorded from this repository's own cover.out. The import path is the whole
// difficulty: the profile names a package, the diff names a file.
const goProfile = `mode: atomic
github.com/WilsonSousajr/omatty/internal/config/config.go:53.35,58.2 1 16
github.com/WilsonSousajr/omatty/internal/config/config.go:68.46,71.36 3 12
github.com/WilsonSousajr/omatty/internal/gate/run.go:30.10,33.4 2 0
`

func TestParseGo_keysFilesByRepoRelativePath(t *testing.T) {
	p, err := coverage.ParseGo(strings.NewReader(goProfile), omattyModule)
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}

	if _, ok := p.Files["internal/config/config.go"]; !ok {
		t.Errorf("files = %v, want the import path stripped to a repo-relative one", keys(p))
	}
}

// A block is startLine.col,endLine.col numStmts count. Every line it spans is
// covered when the count is positive.
func TestParseGo_marksEveryLineOfACoveredBlock(t *testing.T) {
	p, _ := coverage.ParseGo(strings.NewReader(goProfile), omattyModule)

	f := p.Files["internal/config/config.go"]
	for line := 53; line <= 58; line++ {
		if covered, known := f.Lines[line]; !known || !covered {
			t.Errorf("line %d: covered=%v known=%v, want covered", line, covered, known)
		}
	}
}

func TestParseGo_marksEveryLineOfAZeroCountBlock(t *testing.T) {
	p, _ := coverage.ParseGo(strings.NewReader(goProfile), omattyModule)

	f := p.Files["internal/gate/run.go"]
	for line := 30; line <= 33; line++ {
		if covered, known := f.Lines[line]; !known || covered {
			t.Errorf("line %d: covered=%v known=%v, want uncovered", line, covered, known)
		}
	}
}

// The whole point of three states. A line in no block is a declaration, a
// brace or a comment - not an uncovered statement. Marking those would be
// noise, and noise trains the eye to ignore the marker.
func TestParseGo_aLineInNoBlockHasNoVerdict(t *testing.T) {
	p, _ := coverage.ParseGo(strings.NewReader(goProfile), omattyModule)

	f := p.Files["internal/config/config.go"]
	for _, line := range []int{1, 52, 59, 67, 200} {
		if _, known := f.Lines[line]; known {
			t.Errorf("line %d has a verdict; it is in no block and is not a statement", line)
		}
	}
}

// One line can host two statements. If either ran, the line was reached, so
// covered wins however the blocks are ordered in the file.
func TestParseGo_overlappingBlocks_resolveToCovered(t *testing.T) {
	for _, order := range []string{
		"mode: set\nm/a.go:10.1,12.2 1 0\nm/a.go:11.1,11.9 1 4\n",
		"mode: set\nm/a.go:11.1,11.9 1 4\nm/a.go:10.1,12.2 1 0\n",
	} {
		p, err := coverage.ParseGo(strings.NewReader(order), "m")
		if err != nil {
			t.Fatalf("ParseGo() error = %v", err)
		}
		if covered := p.Files["a.go"].Lines[11]; !covered {
			t.Errorf("line 11 uncovered for order %q; one statement running is enough", order)
		}
		if covered := p.Files["a.go"].Lines[10]; covered {
			t.Error("line 10 covered; only line 11 had a positive count")
		}
	}
}

// A record for another module is not this repository's file, and guessing a
// path for it would put markers on the wrong lines.
func TestParseGo_skipsRecordsFromAnotherModule(t *testing.T) {
	in := "mode: set\ngithub.com/other/thing/x.go:1.1,2.2 1 1\nm/a.go:5.1,5.9 1 1\n"

	p, _ := coverage.ParseGo(strings.NewReader(in), "m")

	if len(p.Files) != 1 {
		t.Errorf("files = %v, want only this module's", keys(p))
	}
}

// A profile is written by a tool, and a truncated or odd line is not worth
// losing the rest of the file over.
func TestParseGo_skipsMalformedLinesRatherThanFailing(t *testing.T) {
	in := "mode: set\nnot a record\nm/a.go:bad.block 1 1\nm/a.go:5.1,5.9 1 1\n"

	p, err := coverage.ParseGo(strings.NewReader(in), "m")

	if err != nil {
		t.Fatalf("ParseGo() error = %v, want the good record kept", err)
	}
	if _, known := p.Files["a.go"].Lines[5]; !known {
		t.Error("the well-formed record after a malformed one was lost")
	}
}

// Nothing to say about a file the profile never mentions, and the caller must
// be able to ask without a nil check.
func TestProfile_anUnknownFileAnswersNothing(t *testing.T) {
	p, _ := coverage.ParseGo(strings.NewReader(goProfile), omattyModule)

	if covered, known := p.Files["internal/nope/nope.go"].Lines[1]; known || covered {
		t.Errorf("an unmentioned file answered covered=%v known=%v, want both false", covered, known)
	}
}

func TestParseGo_anEmptyProfileIsNotAnError(t *testing.T) {
	p, err := coverage.ParseGo(strings.NewReader("mode: set\n"), "m")
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	if len(p.Files) != 0 {
		t.Errorf("files = %v, want none", keys(p))
	}
}

func keys(p coverage.Profile) []string {
	out := make([]string, 0, len(p.Files))
	for k := range p.Files {
		out = append(out, k)
	}
	return out
}
