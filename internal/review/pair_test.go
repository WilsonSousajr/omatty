package review_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// changed is a diff that touched these paths and nothing else, which is all
// the pairing question needs for every language but Rust.
func changed(paths ...string) review.Diff {
	files := make([]review.File, 0, len(paths))
	for _, p := range paths {
		files = append(files, review.File{Path: p})
	}
	return review.Diff{Files: files}
}

// rustFile is a .rs file whose hunk adds the given lines, since Rust's tests
// live in the file they test and the answer is in the added text.
func rustFile(path string, added ...string) review.File {
	lines := make([]review.Line, 0, len(added))
	for i, text := range added {
		lines = append(lines, review.Line{Kind: review.LineAdded, Text: text, NewNo: i + 1})
	}
	return review.File{Path: path, Hunks: []review.Hunk{{Header: "@@ -1 +1 @@", Lines: lines}}}
}

func TestPair(t *testing.T) {
	cases := []struct {
		name string
		diff review.Diff
		want review.Pairing
	}{
		{
			name: "Go source with its test",
			diff: changed("internal/gate/run.go", "internal/gate/run_test.go"),
			want: review.PairingPaired,
		},
		{
			name: "Go source alone",
			diff: changed("internal/gate/run.go"),
			want: review.PairingUnpaired,
		},
		{
			name: "docs and config only",
			diff: changed("README.md", "config.toml", ".github/workflows/ci.yml"),
			want: review.PairingNone,
		},
		{
			name: "source beside docs is still source",
			diff: changed("README.md", "internal/gate/run.go"),
			want: review.PairingUnpaired,
		},
		{
			name: "TypeScript with a .test. sibling",
			diff: changed("src/cart.ts", "src/cart.test.ts"),
			want: review.PairingPaired,
		},
		{
			name: "TypeScript with a .spec. sibling",
			diff: changed("src/cart.ts", "src/cart.spec.ts"),
			want: review.PairingPaired,
		},
		{
			name: "JavaScript under __tests__",
			diff: changed("src/cart.js", "src/__tests__/cart.js"),
			want: review.PairingPaired,
		},
		{
			name: "TypeScript alone",
			diff: changed("src/cart.tsx"),
			want: review.PairingUnpaired,
		},
		{
			name: "Python with a test_ prefix",
			diff: changed("app/cart.py", "app/test_cart.py"),
			want: review.PairingPaired,
		},
		{
			name: "Python under tests/",
			diff: changed("app/cart.py", "tests/cart.py"),
			want: review.PairingPaired,
		},
		{
			name: "Python alone",
			diff: changed("app/cart.py"),
			want: review.PairingUnpaired,
		},
		{
			name: "Ruby with its spec",
			diff: changed("lib/cart.rb", "spec/cart_spec.rb"),
			want: review.PairingPaired,
		},
		{
			name: "Ruby alone",
			diff: changed("lib/cart.rb"),
			want: review.PairingUnpaired,
		},
		{
			name: "a language with no rule for its tests",
			diff: changed("src/Cart.java"),
			want: review.PairingUnknown,
		},
		{
			name: "nothing changed at all",
			diff: review.Diff{},
			want: review.PairingNone,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := review.Pair(c.diff); got != c.want {
				t.Errorf("Pair() = %v, want %v", got, c.want)
			}
		})
	}
}

// Rust is the interesting case: tests live in the file they test, behind
// #[cfg(test)], so "no test file changed" is simply wrong there. A flag that
// cried wolf on every Rust change would teach the eye to skip it, and then it
// would be worth nothing on the languages where it was right.
func TestPair_rust(t *testing.T) {
	cases := []struct {
		name  string
		files []review.File
		want  review.Pairing
	}{
		{
			name:  "a hunk that adds a #[cfg(test)] module",
			files: []review.File{rustFile("src/cart.rs", "fn total() -> u32 { 1 }", "#[cfg(test)]", "mod tests {")},
			want:  review.PairingPaired,
		},
		{
			name:  "a hunk that adds a single #[test]",
			files: []review.File{rustFile("src/cart.rs", "    #[test]", "    fn totals() { }")},
			want:  review.PairingPaired,
		},
		{
			name:  "a hunk that adds neither",
			files: []review.File{rustFile("src/cart.rs", "fn total() -> u32 { 1 }")},
			want:  review.PairingUnknown,
		},
		{
			name:  "a Rust change beside a tests/ file",
			files: []review.File{rustFile("src/cart.rs", "fn total() -> u32 { 1 }"), {Path: "tests/cart.rs"}},
			want:  review.PairingPaired,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := review.Pair(review.Diff{Files: c.files}); got != c.want {
				t.Errorf("Pair() = %v, want %v", got, c.want)
			}
		})
	}
}

// An existing #[test] that a hunk merely moves past is not evidence: only
// added lines are read, the same rule the coverage overlay follows.
func TestPair_rustContextIsNotEvidence(t *testing.T) {
	f := review.File{Path: "src/cart.rs", Hunks: []review.Hunk{{Lines: []review.Line{
		{Kind: review.LineContext, Text: "#[test]"},
		{Kind: review.LineAdded, Text: "fn total() -> u32 { 2 }", NewNo: 2},
	}}}}

	if got := review.Pair(review.Diff{Files: []review.File{f}}); got != review.PairingUnknown {
		t.Errorf("Pair() = %v, want PairingUnknown from a context line alone", got)
	}
}

// A pairing names itself, so a test failure and a log line say which of the
// four it was rather than an integer.
func TestPairing_String(t *testing.T) {
	for p, want := range map[review.Pairing]string{
		review.PairingNone:     "none",
		review.PairingPaired:   "paired",
		review.PairingUnpaired: "unpaired",
		review.PairingUnknown:  "unknown",
	} {
		if got := p.String(); got != want {
			t.Errorf("Pairing(%d).String() = %q, want %q", int(p), got, want)
		}
	}
}
