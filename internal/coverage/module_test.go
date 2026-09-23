package coverage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/coverage"
)

// A Go profile is keyed by import path, so reading one needs the module path
// of the tree it describes. It comes from go.mod rather than from `go list`:
// the answer is one line of a file the checkout already holds, and asking the
// toolchain would put a subprocess in front of a diff being drawn.
func TestModulePath(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "the plain directive",
			body: "module github.com/WilsonSousajr/omatty\n\ngo 1.26\n",
			want: "github.com/WilsonSousajr/omatty",
		},
		{
			name: "a file that opens with comments",
			body: "// generated, do not edit\n\nmodule example.com/x\n",
			want: "example.com/x",
		},
		{
			name: "a trailing comment on the directive",
			body: "module example.com/x // why\n",
			want: "example.com/x",
		},
		{
			name: "a quoted path, which go.mod permits",
			body: "module \"example.com/x\"\n",
			want: "example.com/x",
		},
		{
			name: "a file with no module directive",
			body: "go 1.26\n",
			want: "",
		},
		{
			name: "a word that merely starts with module",
			body: "modules are not this\nmodule example.com/x\n",
			want: "example.com/x",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(c.body), 0o600); err != nil {
				t.Fatalf("setup: %v", err)
			}

			if got := coverage.ModulePath(root); got != c.want {
				t.Errorf("ModulePath() = %q, want %q", got, c.want)
			}
		})
	}
}

// A tree with no go.mod is not an error: lcov needs no module path at all, and
// a caller reading a profile out of a Node or Cargo checkout asks for this and
// should get silence rather than a failure it has nothing to do with.
func TestModulePath_noGoMod_isEmptyRatherThanAnError(t *testing.T) {
	if got := coverage.ModulePath(t.TempDir()); got != "" {
		t.Errorf("ModulePath() = %q, want the empty value", got)
	}
}
