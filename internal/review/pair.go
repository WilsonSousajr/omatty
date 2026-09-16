package review

import (
	"path"
	"strings"
)

// Pairing is whether a diff that changed source also changed tests (#256).
//
//	switch review.Pair(d) {
//	case review.PairingUnpaired: // source changed and no test did
//	}
//
// The cheap half of M10's question, and it needs no coverage data at all. It
// is a remark rather than a verdict: the honest answer to "did this change its
// tests" is often "there is no rule for these files", which is why Unknown is
// one of the four rather than folded into Unpaired.
type Pairing int

// The four outcomes. None is the zero value, so a Pairing nobody computed
// raises no flag.
const (
	// PairingNone is a diff with nothing in it that counts as source - docs,
	// configuration, a lockfile.
	PairingNone Pairing = iota
	// PairingPaired is source and tests both changed.
	PairingPaired
	// PairingUnpaired is source changed and no test did, in a language whose
	// tests this package knows how to recognise.
	PairingUnpaired
	// PairingUnknown is source changed in a language with no rule here - or
	// Rust with no in-file tests added. Not a finding, an absence of one.
	PairingUnknown
)

// String names a pairing for logs and test failures.
func (p Pairing) String() string {
	switch p {
	case PairingNone:
		return "none"
	case PairingPaired:
		return "paired"
	case PairingUnpaired:
		return "unpaired"
	case PairingUnknown:
		return "unknown"
	}
	return "invalid"
}

// Pair reports how d pairs source changes with test changes.
//
// One test file changed is enough to call the whole diff paired: this answers
// "was testing thought about", not "is every line covered", which is what the
// overlay beside it is for (#255).
func Pair(d Diff) Pairing {
	var source, recognised bool
	for _, f := range d.Files {
		if isTestPath(f.Path) || addsInlineTests(f) {
			return PairingPaired
		}
		if !isSource(f.Path) {
			continue
		}
		source = true
		recognised = recognised || hasTestConvention(f.Path)
	}
	switch {
	case recognised:
		return PairingUnpaired
	case source:
		return PairingUnknown
	}
	return PairingNone
}

// testDirs are directory names that mean "the files under here are tests",
// whatever language they are written in.
var testDirs = map[string]bool{"tests": true, "__tests__": true, "spec": true}

// isTestPath reports whether a path names a test file, by the conventions each
// ecosystem actually uses.
func isTestPath(p string) bool {
	for _, seg := range strings.Split(path.Dir(p), "/") {
		if testDirs[seg] {
			return true
		}
	}
	return isTestName(path.Base(p), path.Ext(p))
}

// isTestName reads the filename conventions: Go's _test.go, JavaScript and
// TypeScript's .test./.spec. infixes, Python's test_ prefix and _test suffix,
// and Ruby's _spec.
func isTestName(base, ext string) bool {
	switch ext {
	case ".go":
		return strings.HasSuffix(base, "_test.go")
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx":
		return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
	case ".py":
		return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	case ".rb":
		return strings.HasSuffix(base, "_spec.rb") || strings.HasSuffix(base, "_test.rb")
	}
	return false
}

// conventions are the languages this package can tell a test file from a
// source file in. Deliberately short: a language listed here and guessed wrong
// produces a flag on a diff that did test itself, and one wrong flag costs
// more than the several right ones it sat beside.
var conventions = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".py": true, ".rb": true,
}

// hasTestConvention reports whether "no test file changed" means anything for
// this file's language.
func hasTestConvention(p string) bool { return conventions[path.Ext(p)] }

// sources are what counts as source at all. Anything absent - Markdown, TOML,
// YAML, a lockfile, an image - leaves a diff of only such files reporting
// None, because "you changed the README and no tests" is not a finding.
var sources = map[string]bool{
	".go": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".py": true, ".rb": true, ".rs": true,
	".java": true, ".kt": true, ".swift": true, ".cs": true, ".scala": true,
	".c": true, ".h": true, ".cc": true, ".cpp": true, ".hpp": true,
	".php": true, ".sh": true, ".ex": true, ".exs": true,
}

func isSource(p string) bool { return sources[path.Ext(p)] }

// addsInlineTests is Rust's case, and the reason Unknown exists.
//
// Rust puts tests in the *same file* behind #[cfg(test)], so "no test file
// changed" is simply wrong there. A .rs hunk that adds #[cfg(test)] or #[test]
// tested itself; one that does not is Unknown rather than Unpaired, because
// the tests it has may be three lines further down in a file this diff never
// shows.
//
// Only added lines count. An existing attribute a hunk merely scrolls past is
// context, not evidence - the same rule the coverage overlay follows.
func addsInlineTests(f File) bool {
	if path.Ext(f.Path) != ".rs" {
		return false
	}
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			text := strings.TrimSpace(l.Text)
			if l.Kind != LineAdded {
				continue
			}
			if strings.HasPrefix(text, "#[test]") || strings.HasPrefix(text, "#[cfg(test)]") {
				return true
			}
		}
	}
	return false
}
