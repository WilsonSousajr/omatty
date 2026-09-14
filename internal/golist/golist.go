// Package golist is omatty's only route to the `go list` command.
//
//	pkgs, err := golist.List(root, "./internal/...")
//
// It exists for the reason internal/vcs exists: a tool invoked as a subprocess
// is a seam, and the blast radius of its output format changing belongs inside
// one package we own. The gate tools that score complexity and map the import
// graph both need the same answer from it, and neither should be parsing
// command output itself.
//
// Asking go list rather than walking the tree is not a stylistic choice. It is
// the only way to get the file set the compiler agrees to: internal/gate holds
// procgroup_other.go behind //go:build !unix, which neither CI runner ever
// compiles, and a filepath.Walk would hand a scorer a file that has no coverage
// data because it has no coverage to have.
package golist

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Package is what `go list -json` says about one package, narrowed to the
// fields omatty's gate tools read. The names match go list's own JSON so the
// mapping stays checkable against `go help list`.
type Package struct {
	// ImportPath is the full path, e.g. github.com/you/repo/internal/gate.
	ImportPath string
	// Dir is the absolute directory holding the package.
	Dir string
	// GoFiles are the non-test .go files this platform compiles, without a
	// directory. Test files arrive in separate fields go list keeps apart, so
	// there is nothing here to filter out.
	GoFiles []string
	// IgnoredGoFiles are the .go files a build constraint excluded here.
	// internal/gate/procgroup_other.go is //go:build !unix and so is in this
	// list on both CI runners. Carried only so a report can say how many files
	// it did not look at: silence about them is how a metric quietly stops
	// covering half a package.
	IgnoredGoFiles []string
	// Imports are this package's *direct* imports, stdlib and third party
	// included, so a caller measuring the module's own structure filters by
	// the module prefix. Direct and not transitive: `go list -deps` would
	// answer a different question, and efferent coupling computed from it
	// would count the whole reachable graph as one package's dependencies.
	Imports []string
	// TestImports and XTestImports are what the package's own tests and its
	// _test package import. Kept apart from Imports because production
	// structure and test structure are different questions - and because the
	// only import cycle a Go repository can actually have lives here: the
	// compiler refuses one in Imports, but a_test -> b -> a is legal.
	TestImports  []string
	XTestImports []string
}

// syntheticModule is what `go list -m` answers outside a module: not an error,
// not a module path, and exit 0.
const syntheticModule = "command-line-arguments"

// Module returns the module path of the module rooted at or above dir.
//
//	module, err := golist.Module(".")   // github.com/you/repo
//
// A coverage profile keys its records by import path, so anything translating
// between a profile and a file name needs this prefix.
//
// Run outside a module, `go list -m` prints "command-line-arguments" and exits
// 0 rather than failing. Passing that on would be the worst kind of wrong: no
// profile record would match the prefix, every function would score zero
// coverage, and the report would look exactly like a genuinely untested tree.
// So it is rejected here, where the reason can still be explained.
func Module(dir string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("golist: go list -m: %w%s", err, said(err))
	}
	module := strings.TrimSpace(string(out))
	if module == "" || module == syntheticModule {
		return "", fmt.Errorf("golist: %s names no module (go list -m said %q)", dir, module)
	}
	return module, nil
}

// List runs `go list -json` for patterns, resolved relative to dir.
//
// A pattern that matches nothing yields no packages and no error: the caller
// asked a question and "none" is an answer. A pattern go list rejects is an
// error naming what it said, because a gate step that only reports failure
// without the reason sends a session looking in the wrong place.
func List(dir string, patterns ...string) ([]Package, error) {
	cmd := exec.Command("go", append([]string{"list", "-json"}, patterns...)...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("golist: go list %s: %w%s",
			strings.Join(patterns, " "), err, said(err))
	}
	return decode(bytes.NewReader(out))
}

// decode reads go list's output, which is a stream of concatenated JSON
// objects rather than an array - so json.Unmarshal would return only the first
// package and no error to say the rest were dropped.
func decode(r io.Reader) ([]Package, error) {
	var pkgs []Package
	dec := json.NewDecoder(r)
	for {
		var pkg Package
		switch err := dec.Decode(&pkg); {
		case errors.Is(err, io.EOF):
			return pkgs, nil
		case err != nil:
			return nil, fmt.Errorf("golist: reading go list output: %w", err)
		}
		pkgs = append(pkgs, pkg)
	}
}

// said returns what the command wrote to stderr, which is where go list puts
// the sentence a person needs.
func said(err error) string {
	var exit *exec.ExitError
	if !errors.As(err, &exit) || len(exit.Stderr) == 0 {
		return ""
	}
	return ": " + strings.TrimSpace(string(exit.Stderr))
}
