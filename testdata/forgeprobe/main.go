// Command forgeprobe proves the thing internal/forge's unit tests cannot: that
// the real gh on this machine answers with the field names the fold reads, in
// a real repository, using the operator's own authentication.
//
//	go run ./testdata/forgeprobe
//	go run ./testdata/forgeprobe /path/to/a/checkout
//	go run ./testdata/forgeprobe -forge gitlab /path/to/a/checkout
//	go run ./testdata/forgeprobe -transport rest /path/to/a/checkout
//
// It reads through the same Router the TUI builds (#452): the checkout's
// origin names its forge, and -forge names it instead, as a [forge.hosts] line
// for origin's host would.
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
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

func main() {
	kind := flag.String("forge", "", "name origin's host as this forge, as [forge.hosts] would")
	transport := flag.String("transport", "", "cli or rest: read that way alone (default: CLI first, REST fallback)")
	flag.Parse()
	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	r := forge.NewRouter(forge.Options{Remote: vcs.NewCLI().RemoteURL, Hosts: hostsFor(root, *kind), Transport: transportOf(*transport)})
	fmt.Println("transport:", map[string]string{"": "auto (CLI first)", "cli": "cli", "rest": "rest"}[*transport])
	fmt.Println("repository:", root)
	issues := probeIssues(r, root)
	probePRs(r, root)
	probeDetail(r, root, issues)
	fmt.Printf("\nforge: %+v\n", r.Label(root))
	fmt.Println("\nRead it: every title non-empty, every date this decade, labels present.")
}

// transportOf reads -transport.
func transportOf(s string) forge.Transport {
	switch s {
	case "":
		return forge.TransportAuto
	case "cli":
		return forge.TransportCLI
	case "rest":
		return forge.TransportREST
	}
	exitOn(fmt.Errorf("-transport %q, want cli or rest", s))
	return forge.TransportAuto
}

// hostsFor is the one-line [forge.hosts] -forge stands for, or none.
func hostsFor(root, kind string) forge.Hosts {
	if kind == "" {
		return nil
	}
	k, err := forge.ParseKind(kind)
	exitOn(err)
	raw, err := vcs.NewCLI().RemoteURL(root)
	exitOn(err)
	remote, err := forge.ParseRemote(raw)
	exitOn(err)
	return forge.Hosts{remote.Host: k}
}

func probeIssues(r *forge.Router, root string) []forge.Issue {
	issues, err := r.ListIssues(root)
	if err != nil {
		fmt.Println("ListIssues:", err)
	}
	fmt.Printf("\n--- %d open issues ---\n", len(issues))
	for _, is := range issues {
		fmt.Printf("#%-4d %-11s %-52s %s %s\n", is.Number, first(is.Labels), clip(is.Title, 52),
			is.Updated.Format("2006-01-02"), who(is))
	}
	return issues
}

func probePRs(r *forge.Router, root string) {
	prs, err := r.ListPRs(root)
	if err != nil {
		fmt.Println("ListPRs:", err)
	}
	fmt.Printf("\n--- open changes of %d read ---\n", len(prs))
	for _, pr := range prs {
		if pr.State != forge.Open {
			continue
		}
		fmt.Printf("%-5s %-52s %s ci=%v draft=%v conflict=%v fork=%v head=%.8s\n", r.Label(root).Ref(pr.Number),
			clip(pr.Title, 52), pr.Updated.Format("2006-01-02"), pr.CI, pr.Draft, pr.Conflict, pr.Fork, pr.Head)
	}
}

// probeDetail reads the first issue in full: the body and the comments are a
// third call with its own field names, and the lists prove nothing about it.
func probeDetail(r *forge.Router, root string, issues []forge.Issue) {
	if len(issues) == 0 {
		return
	}
	d, err := r.ViewIssue(root, issues[0].Number)
	if err != nil {
		fmt.Println("ViewIssue:", err)
		return
	}
	fmt.Printf("\n--- #%d in full ---\n%s by %s, %s, %d comment(s), %d bytes of body, truncated=%v\n",
		d.Number, clip(d.Title, 52), d.Author, d.Created.Format("2006-01-02"), len(d.Comments), len(d.Body), d.Truncated)
}

func exitOn(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "forgeprobe:", err)
		os.Exit(2)
	}
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
