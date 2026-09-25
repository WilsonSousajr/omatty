// Command forgeprobe proves the thing internal/forge's unit tests cannot: that
// the real gh on this machine answers with the field names the fold reads, in
// a real repository, using the operator's own authentication.
//
//	go run ./testdata/forgeprobe
//	go run ./testdata/forgeprobe /path/to/a/checkout
//
// It is the same argument as dtachprobe and gateprobe. The tests fold recorded
// JSON, which proves the fold and nothing about gh: a field renamed upstream,
// or one this gh does not have, folds to a zero value in silence - an empty
// title, an epoch age - and every test stays green. #43 is what shipped the
// last time a package's tests substituted a fake for the thing they tested.
//
// It reaches the network by design, which is why it lives in testdata/,
// outside ./... and outside the gate. A person reads the output (roadmap rule
// 2). One question it already settled: `gh issue list --json comments` answers
// with the comment objects, not a count, so ListIssues does not ask for it -
// the count comes from the item call instead (#397).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	cli := forge.NewCLI()
	fmt.Println("repository:", root)

	issues, err := cli.ListIssues(root)
	if err != nil {
		fmt.Println("ListIssues:", err)
	}
	fmt.Printf("\n--- %d open issues ---\n", len(issues))
	for _, is := range issues {
		fmt.Printf("#%-4d %-11s %-52s %s %s\n", is.Number, first(is.Labels), clip(is.Title, 52),
			is.Updated.Format("2006-01-02"), who(is))
	}

	prs, err := cli.ListPRs(root)
	if err != nil {
		fmt.Println("ListPRs:", err)
	}
	fmt.Printf("\n--- open pull requests of %d read ---\n", len(prs))
	for _, pr := range prs {
		if pr.State != forge.Open {
			continue
		}
		fmt.Printf("#%-4d %-52s %s ci=%v draft=%v\n", pr.Number, clip(pr.Title, 52),
			pr.Updated.Format("2006-01-02"), pr.CI, pr.Draft)
	}
	fmt.Println("\nRead it: every title non-empty, every date this decade, labels present.")
}

// who names the assignee, or says nobody is on it.
func who(is forge.Issue) string {
	if is.Assignee == "" {
		return "(unassigned, by " + is.Author + ")"
	}
	return is.Assignee
}

func first(labels []string) string {
	if len(labels) == 0 {
		return "-"
	}
	return labels[0]
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}
