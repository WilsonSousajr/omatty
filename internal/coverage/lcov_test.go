package coverage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/coverage"
)

// Recorded shapes, not invented. lcov is the one format that reaches every
// language on this machine that is not Go, and each writer emits a slightly
// different subset of the record types.

// c8 (node) writes absolute SF paths and a full function block.
const c8LCOV = `TN:
SF:/repo/src/index.ts
FN:3,greet
FNF:1
FNH:1
FNDA:2,greet
DA:1,1
DA:3,2
DA:4,0
BRDA:4,0,0,1
LF:3
LH:2
end_of_record
`

// tarpaulin (rust) writes relative SF paths and only line records.
const tarpaulinLCOV = `TN:
SF:src/main.rs
DA:1,1
DA:5,0
DA:6,0
LF:3
LH:1
end_of_record
`

func TestParseLCOV_readsLineVerdicts(t *testing.T) {
	p, err := coverage.ParseLCOV(strings.NewReader(tarpaulinLCOV), "/repo")
	if err != nil {
		t.Fatalf("ParseLCOV() error = %v", err)
	}

	f := p.Files["src/main.rs"]
	for line, want := range map[int]bool{1: true, 5: false, 6: false} {
		got, known := f.Lines[line]
		if !known || got != want {
			t.Errorf("line %d: covered=%v known=%v, want covered=%v", line, got, known, want)
		}
	}
}

// Same rule as the Go parser: a line the profile never mentions is not a
// statement and gets no verdict.
func TestParseLCOV_aLineWithNoRecordHasNoVerdict(t *testing.T) {
	p, _ := coverage.ParseLCOV(strings.NewReader(tarpaulinLCOV), "/repo")

	if _, known := p.Files["src/main.rs"].Lines[2]; known {
		t.Error("line 2 has a verdict; no DA record mentions it")
	}
}

// Writers disagree about absolute versus relative, so the root is a parameter
// and an absolute path is made relative to it - the diff names repo-relative
// paths and nothing else will match.
func TestParseLCOV_makesAnAbsolutePathRepoRelative(t *testing.T) {
	p, err := coverage.ParseLCOV(strings.NewReader(c8LCOV), "/repo")
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := p.Files["src/index.ts"]; !ok {
		t.Errorf("files = %v, want src/index.ts", keys(p))
	}
}

// A file outside the checkout is not this diff's business, and inventing a
// path for it would put markers on the wrong lines.
func TestParseLCOV_skipsFilesOutsideTheRoot(t *testing.T) {
	in := "TN:\nSF:/elsewhere/vendor/x.js\nDA:1,0\nend_of_record\n" + tarpaulinLCOV

	p, _ := coverage.ParseLCOV(strings.NewReader(in), "/repo")

	if len(p.Files) != 1 {
		t.Errorf("files = %v, want only the one inside the root", keys(p))
	}
}

// Function, branch and summary records carry no line verdict and must not be
// mistaken for one.
func TestParseLCOV_ignoresRecordsThatAreNotLineVerdicts(t *testing.T) {
	p, _ := coverage.ParseLCOV(strings.NewReader(c8LCOV), "/repo")

	f := p.Files["src/index.ts"]
	if len(f.Lines) != 3 {
		t.Errorf("lines = %v, want exactly the three DA records", f.Lines)
	}
}

// Several files in one profile, which is the normal case.
func TestParseLCOV_readsEveryRecordInTheFile(t *testing.T) {
	p, _ := coverage.ParseLCOV(strings.NewReader(c8LCOV+tarpaulinLCOV), "/repo")

	if len(p.Files) != 2 {
		t.Errorf("files = %v, want both", keys(p))
	}
}

// A DA line before any SF names no file and is dropped rather than panicking.
func TestParseLCOV_aVerdictBeforeAnyFileIsDropped(t *testing.T) {
	p, err := coverage.ParseLCOV(strings.NewReader("DA:1,1\n"+tarpaulinLCOV), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Files) != 1 {
		t.Errorf("files = %v, want only the properly opened record", keys(p))
	}
}

func TestParseLCOV_malformedRecordsAreSkipped(t *testing.T) {
	in := "TN:\nSF:src/a.rs\nDA:\nDA:notaline,1\nDA:7,notacount\nDA:9,1\nend_of_record\n"

	p, err := coverage.ParseLCOV(strings.NewReader(in), "/repo")
	if err != nil {
		t.Fatalf("ParseLCOV() error = %v, want the good record kept", err)
	}
	if got := p.Files["src/a.rs"].Lines; len(got) != 1 || !got[9] {
		t.Errorf("lines = %v, want only line 9 covered", got)
	}
}

// Load sniffs the format instead of trusting the extension, because .info,
// .lcov, .out and .txt are all in use for both formats.
func TestLoad_sniffsTheFormat(t *testing.T) {
	root := t.TempDir()
	cases := []struct{ name, body, wantFile string }{
		{"go.out", "mode: set\nm/a.go:1.1,2.2 1 1\n", "a.go"},
		{"lcov.info", "TN:\nSF:src/b.rs\nDA:1,1\nend_of_record\n", "src/b.rs"},
		{"named-like-go.out", "TN:\nSF:src/c.rs\nDA:1,1\nend_of_record\n", "src/c.rs"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(root, c.name)
			if err := os.WriteFile(path, []byte(c.body), 0o600); err != nil {
				t.Fatal(err)
			}

			p, err := coverage.Load(path, root, "m")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if _, ok := p.Files[c.wantFile]; !ok {
				t.Errorf("files = %v, want %s", keys(p), c.wantFile)
			}
		})
	}
}

func TestLoad_aMissingFileIsAnErrorNamingIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.out")

	_, err := coverage.Load(missing, t.TempDir(), "m")

	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("Load() error = %v, want one naming the path", err)
	}
}

// A file that is neither format is an error rather than an empty profile: an
// empty profile would silently mean "nothing is uncovered", which is a lie.
func TestLoad_anUnrecognisedFormatIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weird.json")
	if err := os.WriteFile(path, []byte(`{"coverage": 91}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := coverage.Load(path, t.TempDir(), "m")

	if err == nil {
		t.Fatal("Load() error = nil for an unrecognised format")
	}
	if !strings.Contains(err.Error(), "not a Go or lcov") {
		t.Errorf("error = %q, want it to say which formats are understood", err)
	}
}

// A DA record with a line and no count. Found by reading markDA rather than
// by a crash in the field: strings.Fields("") is empty, and indexing [0] on
// it panics - which in the UI would take the whole model down while drawing a
// diff, and invariant 6 only recovers a supervisor goroutine, not this.
func TestParseLCOV_aRecordWithNoCountDoesNotPanic(t *testing.T) {
	in := "TN:\nSF:src/a.rs\nDA:7,\nDA:8,   \nDA:9,1\nend_of_record\n"

	p, err := coverage.ParseLCOV(strings.NewReader(in), "/repo")

	if err != nil {
		t.Fatalf("ParseLCOV() error = %v", err)
	}
	if got := p.Files["src/a.rs"].Lines; len(got) != 1 || !got[9] {
		t.Errorf("lines = %v, want only line 9 covered", got)
	}
}
