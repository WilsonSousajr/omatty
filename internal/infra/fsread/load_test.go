package fsread_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/fsread"
)

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

			p, err := fsread.Load(path, root, "m")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if _, ok := p.Files[c.wantFile]; !ok {
				t.Errorf("files = %v, want %s", slices.Collect(maps.Keys(p.Files)), c.wantFile)
			}
		})
	}
}

func TestLoad_aMissingFileIsAnErrorNamingIt(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.out")

	_, err := fsread.Load(missing, t.TempDir(), "m")

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

	_, err := fsread.Load(path, t.TempDir(), "m")

	if err == nil {
		t.Fatal("Load() error = nil for an unrecognised format")
	}
	if !strings.Contains(err.Error(), "not a Go or lcov") {
		t.Errorf("error = %q, want it to say which formats are understood", err)
	}
}
