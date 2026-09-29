// Command forgeprobe proves the thing internal/forge's unit tests cannot: that
// the real gh on this machine answers with the field names the fold reads, in
// a real repository, using the operator's own authentication.
//
//	go run ./testdata/forgeprobe
//	go run ./testdata/forgeprobe /path/to/a/checkout
//	go run ./testdata/forgeprobe -forge gitlab /path/to/a/checkout
//	go run ./testdata/forgeprobe -transport rest /path/to/a/checkout
//	go run ./testdata/forgeprobe -open feat/x:main /path/to/a/scratch/checkout
//	go run ./testdata/forgeprobe -protected main -merge 7 /path/to/a/scratch/checkout
//
// -open and -merge WRITE to the forge - #331's ship actions, run for real
// (#464) - so point them only at a scratch repository. -protected only reads.
// Any of the three runs instead of the reads, never beside them.
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
	open := flag.String("open", "", "head:base - WRITES: open a change for head against base")
	protected := flag.String("protected", "", "branch - read whether the forge protects it")
	merge := flag.Int("merge", 0, "number - WRITES: merge that change now, with the repository's own method")
	flag.Parse()
	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	r := forge.NewRouter(forge.Options{Remote: vcs.NewCLI().RemoteURL, Hosts: hostsFor(root, *kind), Transport: transportOf(*transport)})
	fmt.Println("transport:", map[string]string{"": "auto (CLI first)", "cli": "cli", "rest": "rest"}[*transport])
	fmt.Println("repository:", root)
	if probeShip(r, root, *open, *protected, *merge) {
		return
	}
	issues := probeIssues(r, root)
	prs := probePRs(r, root)
	probeDetail(r, root, issues, prs)
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

func probePRs(r *forge.Router, root string) []forge.PR {
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
	return prs
}

// probeDetail reads the first two issues and the first open change in full:
// the body and the comments are a third call with its own field names, and
// the lists prove nothing about it.
func probeDetail(r *forge.Router, root string, issues []forge.Issue, prs []forge.PR) {
	for i, is := range issues {
		if i == 2 {
			break
		}
		d, err := r.ViewIssue(root, is.Number)
		printDetail("#", d, err)
	}
	for _, pr := range prs {
		if pr.State == forge.Open {
			d, err := r.ViewPR(root, pr.Number)
			printDetail(r.Label(root).Sigil, d, err)
			return
		}
	}
}

func printDetail(sigil string, d forge.Detail, err error) {
	if err != nil {
		fmt.Println("\nview:", err)
		return
	}
	fmt.Printf("\n--- %s%d in full ---\n%s by %s, %s, %d comment(s), %d check(s), %d bytes of body, truncated=%v\n",
		sigil, d.Number, clip(d.Title, 52), d.Author, d.Created.Format("2006-01-02"), len(d.Comments), len(d.Checks), len(d.Body), d.Truncated)
	for _, c := range d.Comments {
		fmt.Printf("    comment by %s: %s\n", c.Author, clip(c.Body, 60))
	}
}

// probeShip runs the ship actions asked for, and reports whether any were:
// open, then the protection read, then merge, each printed as it answers.
func probeShip(r *forge.Router, root, open, protected string, merge int) bool {
	if head, base, ok := strings.Cut(open, ":"); ok {
		n, err := r.CreatePR(root, head, base, "Opened by omatty's forge probe (#464)")
		fmt.Printf("\nCreatePR(%s -> %s) = %s, %v\n", head, base, r.Label(root).Ref(n), err)
	}
	if protected != "" {
		p, err := r.BranchProtected(root, protected)
		fmt.Printf("\nBranchProtected(%s) = %v, %v\n", protected, p, err)
	}
	if merge > 0 {
		probeMerge(r, root, merge)
	}
	return open != "" || protected != "" || merge > 0
}

// probeMerge merges number at the head the forge lists for it now, as
// ctrl+o p merges the head its card showed (#599), and says whether the
// forge merged it or only accepted the merge.
func probeMerge(r *forge.Router, root string, number int) {
	prs, err := r.ListPRs(root)
	exitOn(err)
	head := ""
	for _, pr := range prs {
		if pr.Number == number {
			head = pr.Head
		}
	}
	merged, err := r.MergePR(root, number, head)
	fmt.Printf("\nMergePR(%s at %.12s) = merged %v, %v\n", r.Label(root).Ref(number), head, merged, err)
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
