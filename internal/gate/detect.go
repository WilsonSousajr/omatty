package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Detect proposes a gate for the checkout at root, or nil when it recognises
// nothing there.
//
//	steps := gate.Detect(proj.Root)   // then show them, and write only on confirm
//
// It only ever *proposes*: nothing here writes, and nothing here runs. That is
// the posture discover.Propose already takes for projects, and it is load
// bearing for the same reason plus one more.
//
// The extra reason is that some markers are repository-controlled.
// package.json's scripts are written by whoever wrote the repository, so a
// clone must never be able to get a command of its choosing proposed - let
// alone run - merely because omatty registered it. So the Node branch reads
// package.json to learn *whether* a script exists and never what it contains:
// the proposed line is `npm run test`, composed here, and npm resolves the
// script itself, after the operator has confirmed the gate. That is the same
// thing as the operator typing `npm test`, and deliberately no more.
//
// Steps come out cheapest first, because a gate stops at the first step that
// does not pass and there is no reason to spend a test suite discovering that
// the tree is unformatted.
func Detect(root string) []Step {
	for _, detector := range []func(string) []Step{detectGo, detectCargo, detectNode, detectPython} {
		if steps := detector(root); steps != nil {
			return append(steps, coverageStep(root)...)
		}
	}
	return nil
}

// detectGo proposes the line a Go project almost always has, adding lint only
// where the repository is configured for it - proposing a tool the project
// does not use would report Missing forever.
func detectGo(root string) []Step {
	if !exists(root, "go.mod") {
		return nil
	}
	steps := []Step{
		{Name: "fmt", Run: "gofmt -l ."},
		{Name: "vet", Run: "go vet ./..."},
	}
	if exists(root, ".golangci.yml") || exists(root, ".golangci.yaml") {
		steps = append(steps, Step{Name: "lint", Run: "golangci-lint run"})
	}
	return append(steps, Step{Name: "test", Run: "go test ./... -race"})
}

func detectCargo(root string) []Step {
	if !exists(root, "Cargo.toml") {
		return nil
	}
	return []Step{
		{Name: "fmt", Run: "cargo fmt --check"},
		{Name: "lint", Run: "cargo clippy -- -D warnings"},
		{Name: "test", Run: "cargo test"},
	}
}

func detectPython(root string) []Step {
	if !exists(root, "pyproject.toml") {
		return nil
	}
	return []Step{
		{Name: "lint", Run: "ruff check ."},
		{Name: "test", Run: "pytest"},
	}
}

// detectNode reads package.json for the *names* of scripts it defines, never
// their bodies. See Detect's doc comment: this is the one detector whose
// marker file is written by the repository rather than by a tool.
func detectNode(root string) []Step {
	scripts, ok := packageScripts(root)
	if !ok {
		return nil
	}
	runner := nodeRunner(root)
	var steps []Step
	for _, name := range []string{"lint", "test"} {
		if _, defined := scripts[name]; defined {
			steps = append(steps, Step{Name: name, Run: runner + " run " + name})
		}
	}
	return steps
}

// packageScripts returns the script names package.json defines. A file that
// will not parse proposes nothing rather than failing the picker.
func packageScripts(root string) (map[string]json.RawMessage, bool) {
	b, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil, false
	}
	// RawMessage, so a script's body is never decoded into anything this
	// package could accidentally put in a command line.
	var pkg struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return nil, false
	}
	return pkg.Scripts, true
}

// nodeRunner picks the package manager from the lockfile that is present, so
// the proposal matches how the project is actually built.
func nodeRunner(root string) string {
	for _, m := range []struct{ lockfile, runner string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
	} {
		if exists(root, m.lockfile) {
			return m.runner
		}
	}
	return "npm"
}

// coverageStep proposes a project's own coverage script when it ships one, so
// its percentage reaches the card (#225). Last, because it is the most
// expensive thing in the gate and the least likely to be the reason a change
// is wrong.
func coverageStep(root string) []Step {
	const script = "scripts/check-coverage.sh"
	if !exists(root, script) {
		return nil
	}
	return []Step{{Name: "cov", Run: "./" + script, Kind: KindCoverage}}
}

// exists reports whether root holds name.
func exists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}
