package coverage

import (
	"errors"
	"strings"
	"testing"
)

// Every shape a tool could emit that this parser must survive. A profile is
// written by something else, and one odd record is never worth losing the
// rest of the file over - so each of these is skipped, not fatal.
func TestGoRecord_malformedShapesAreSkipped(t *testing.T) {
	cases := []struct{ name, line string }{
		{"no colon at all", "m/a.go 1.1,2.2 1 1"},
		{"wrong module", "other/a.go:1.1,2.2 1 1"},
		{"too few fields", "m/a.go:1.1,2.2 1"},
		{"too many fields", "m/a.go:1.1,2.2 1 1 extra"},
		{"no comma in the range", "m/a.go:1.1 1 1"},
		{"no dot in the start position", "m/a.go:1,2.2 1 1"},
		{"no dot in the end position", "m/a.go:1.1,2 1 1"},
		{"start line is not a number", "m/a.go:x.1,2.2 1 1"},
		{"end line is not a number", "m/a.go:1.1,y.2 1 1"},
		{"count is not a number", "m/a.go:1.1,2.2 1 many"},
		{"end before start", "m/a.go:9.1,2.2 1 1"},
		{"line zero", "m/a.go:0.1,2.2 1 1"},
		{"empty path", "m/:1.1,2.2 1 1"},
		{"blank", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, ok := goRecord(c.line, "m/"); ok {
				t.Errorf("goRecord(%q) accepted a record it cannot trust", c.line)
			}
		})
	}
}

// And the well-formed one still parses, so the table above is not passing by
// rejecting everything.
func TestGoRecord_acceptsAWellFormedRecord(t *testing.T) {
	path, block, ok := goRecord("m/a.go:10.2,14.3 2 7", "m/")
	if !ok {
		t.Fatal("goRecord() rejected a well-formed record")
	}
	if path != "a.go" || block.first != 10 || block.last != 14 || !block.covered {
		t.Errorf("goRecord() = %q %+v, want a.go 10..14 covered", path, block)
	}
}

// errReader fails partway, standing in for a truncated or unreadable profile.
type errReader struct{ read bool }

func (e *errReader) Read(p []byte) (int, error) {
	if e.read {
		return 0, errors.New("disk went away")
	}
	e.read = true
	n := copy(p, "mode: set\n")
	return n, nil
}

// A read that fails is an error, unlike a record that will not parse: the
// difference is whether the rest of the profile was ever seen.
func TestParseGo_aFailedReadIsAnError(t *testing.T) {
	_, err := ParseGo(&errReader{}, "m")

	if err == nil {
		t.Fatal("ParseGo() error = nil, want the read failure surfaced")
	}
	if !strings.Contains(err.Error(), "coverage:") {
		t.Errorf("error = %q, want it to name the package doing the work", err)
	}
}

// A module path written with a trailing slash is the same module.
func TestParseGo_toleratesATrailingSlashOnTheModulePath(t *testing.T) {
	p, err := ParseGo(strings.NewReader("mode: set\nm/a.go:1.1,2.2 1 1\n"), "m/")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Files["a.go"]; !ok {
		t.Errorf("files = %v, want a.go", p.Files)
	}
}
