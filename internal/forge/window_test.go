package forge_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// ghByState is a gh stand-in that answers by the --state it is asked for, as
// the real one does, recording each call. A state it has no answer for gets
// an empty list.
type ghByState struct {
	Answers map[string]string // state -> JSON
}

// install writes the script and returns its path and the calls file.
func (g ghByState) install(t *testing.T) (bin, calls string) {
	t.Helper()
	dir := t.TempDir()
	calls = filepath.Join(dir, "calls")
	var cases strings.Builder
	for state, answer := range g.Answers {
		file := filepath.Join(dir, state+".json")
		if err := os.WriteFile(file, []byte(answer), 0o600); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&cases, "  *'--state %s '*) cat '%s' ;;\n", state, file)
	}
	script := "#!/bin/sh\n" +
		`printf '%s|%s\n' "$PWD" "$*" >> '` + calls + "'\n" +
		`case "$* " in` + "\n" + cases.String() + "  *) echo '[]' ;;\nesac\n"
	bin = filepath.Join(dir, "gh")
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin, calls
}

// mergedRange is the JSON for merged PRs from..to, newest first, as gh lists.
func mergedRange(from, to int) string {
	var prs []string
	for n := to; n >= from; n-- {
		prs = append(prs, fmt.Sprintf(`{"number":%d,"headRefName":"b%d","state":"MERGED"}`, n, n))
	}
	return "[" + strings.Join(prs, ",") + "]"
}

// An open pull request older than the newest hundred is still on its card
// (#358). One `--state all --limit 100` call orders by creation, and this
// repository opened about a hundred PRs in ten days: an open PR past that
// window dropped out, and its card fell back to the branch with no "?". The
// open set is now asked for on its own, so its age does not matter.
func TestCLI_anOpenPullRequestOlderThanTheWindowIsStillListed_issue358(t *testing.T) {
	bin, _ := ghByState{Answers: map[string]string{
		"all":    mergedRange(101, 200), // the newest hundred: #5 is not among them
		"open":   `[{"number":5,"headRefName":"old-work","state":"OPEN"}]`,
		"closed": mergedRange(171, 200),
	}}.install(t)

	prs, err := forge.NewCLIWithBin(bin).ListPRs(t.TempDir())

	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range prs {
		if pr.Number == 5 && pr.State == forge.Open {
			return
		}
	}
	t.Errorf("ListPRs() has no open #5 among %d pull requests", len(prs))
}
