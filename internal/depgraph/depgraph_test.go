package depgraph_test

import (
	"math"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/depgraph"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

const module = "m"

// A three-package chain: ui depends on registry depends on paths. Instability
// falls along it, which is what the Stable Dependencies Principle asks for.
func chain() []golist.Package {
	return []golist.Package{
		{ImportPath: "m/internal/ui", Imports: []string{"m/internal/registry", "fmt"}},
		{ImportPath: "m/internal/registry", Imports: []string{"m/internal/paths"}},
		{ImportPath: "m/internal/paths", Imports: []string{"os"}},
	}
}

// Ca counts packages that import you, Ce packages you import - and only within
// the module. Counting fmt and os as efferent coupling would make every leaf
// look unstable and the metric would say nothing.
func TestBuild_countsCouplingWithinTheModuleOnly(t *testing.T) {
	g := depgraph.Build(module, chain())

	for _, c := range []struct {
		pkg    string
		ca, ce int
	}{
		{"m/internal/ui", 0, 1},
		{"m/internal/registry", 1, 1},
		{"m/internal/paths", 1, 0},
	} {
		node := g.Nodes[c.pkg]
		if node.Afferent != c.ca || node.Efferent != c.ce {
			t.Errorf("%s: Ca=%d Ce=%d, want Ca=%d Ce=%d",
				c.pkg, node.Afferent, node.Efferent, c.ca, c.ce)
		}
	}
}

// I = Ce / (Ca + Ce). A package nothing imports and which imports others is
// maximally unstable; a pure leaf everything depends on is maximally stable.
func TestInstability_isEfferentOverTotalCoupling(t *testing.T) {
	g := depgraph.Build(module, chain())

	for pkg, want := range map[string]float64{
		"m/internal/ui": 1, "m/internal/registry": 0.5, "m/internal/paths": 0,
	} {
		if got := g.Nodes[pkg].Instability(); math.Abs(got-want) > 0.001 {
			t.Errorf("%s instability = %.3f, want %.3f", pkg, got, want)
		}
	}
}

// A package with no coupling at all is stable, not a division by zero.
func TestInstability_anIsolatedPackageIsStable(t *testing.T) {
	g := depgraph.Build(module, []golist.Package{{ImportPath: "m/internal/alone"}})

	if got := g.Nodes["m/internal/alone"].Instability(); got != 0 {
		t.Errorf("instability = %v, want 0 for a package with no coupling", got)
	}
}

// The chain obeys the principle, so nothing is reported. A check that cannot
// come back clean is not a check.
func TestViolations_aDescendingChainIsClean(t *testing.T) {
	g := depgraph.Build(module, chain())

	if got := g.Violations(); len(got) != 0 {
		t.Errorf("Violations() = %v, want none on a descending chain", got)
	}
}

// And the reverse is caught: a stable package must not depend on an unstable
// one, because everything that depends on the stable package is then pinned by
// something built to change.
//
// core is depended on by three packages and imports one, so I = 1/4. churn is
// depended on by core alone and imports two, so I = 2/3. The edge core -> churn
// therefore runs uphill. It cannot be built as a cycle - a cycle equalises the
// instability of everything in it, and the Go compiler refuses one anyway.
func TestViolations_catchesAStablePackageDependingOnAnUnstableOne(t *testing.T) {
	pkgs := []golist.Package{
		{ImportPath: "m/internal/x", Imports: []string{"m/internal/core"}},
		{ImportPath: "m/internal/y", Imports: []string{"m/internal/core"}},
		{ImportPath: "m/internal/z", Imports: []string{"m/internal/core"}},
		{ImportPath: "m/internal/core", Imports: []string{"m/internal/churn"}},
		{ImportPath: "m/internal/churn", Imports: []string{"m/internal/p", "m/internal/q"}},
		{ImportPath: "m/internal/p"},
		{ImportPath: "m/internal/q"},
	}

	got := depgraph.Build(module, pkgs).Violations()

	if len(got) != 1 {
		t.Fatalf("Violations() = %+v, want exactly one", got)
	}
	if got[0].From.ImportPath != "m/internal/core" || got[0].To.ImportPath != "m/internal/churn" {
		t.Errorf("violation = %s -> %s, want core -> churn",
			got[0].From.ImportPath, got[0].To.ImportPath)
	}
}

// A violation's report must carry both endpoints' coupling, so a reader can
// tell a broken architecture from an unrelated import that shifted a ratio.
func TestReport_namesBothEndpointsCouplingOnAViolation(t *testing.T) {
	pkgs := []golist.Package{
		{ImportPath: "m/internal/x", Imports: []string{"m/internal/core"}},
		{ImportPath: "m/internal/y", Imports: []string{"m/internal/core"}},
		{ImportPath: "m/internal/z", Imports: []string{"m/internal/core"}},
		{ImportPath: "m/internal/core", Imports: []string{"m/internal/churn"}},
		{ImportPath: "m/internal/churn", Imports: []string{"m/internal/p", "m/internal/q"}},
		{ImportPath: "m/internal/p"},
		{ImportPath: "m/internal/q"},
	}
	var out strings.Builder

	depgraph.Report(&out, depgraph.Build(module, pkgs))

	text := out.String()
	if !strings.Contains(text, "SDP:") {
		t.Fatalf("report does not flag the violation:\n%s", text)
	}
	for _, want := range []string{"Ca=3", "Ce=1", "Ca=1", "Ce=2"} {
		if !strings.Contains(text, want) {
			t.Errorf("report omits %q, so the reader cannot judge it:\n%s", want, text)
		}
	}
}

// An import naming a package outside the listed set is ignored rather than
// invented as a node: scoring ./internal/... must not sprout a phantom entry
// for cmd/omatty just because something mentions it.
func TestBuild_ignoresImportsOfPackagesNotListed(t *testing.T) {
	g := depgraph.Build(module, []golist.Package{
		{ImportPath: "m/internal/a", Imports: []string{"m/cmd/omatty"}},
	})

	if len(g.Nodes) != 1 {
		t.Errorf("Nodes = %v, want only the listed package", g.Nodes)
	}
	if len(g.Edges) != 0 {
		t.Errorf("Edges = %v, want none to an unlisted package", g.Edges)
	}
}

// An empty graph has no tightest edge, and must say so rather than divide by
// nothing.
func TestMargin_anEmptyGraphHasNoEdge(t *testing.T) {
	edge, margin := depgraph.Build(module, nil).Margin()

	if edge.From != "" || margin != 0 {
		t.Errorf("Margin() = %+v %v, want the zero edge", edge, margin)
	}
}

// The margin is what makes the gate readable before it fails. SDP is a ratio of
// small integers, so it moves in jumps; a run that only says "clean" gives no
// warning that the next import will break it.
func TestMargin_reportsTheTightestEdge(t *testing.T) {
	g := depgraph.Build(module, chain())

	edge, margin := g.Margin()

	if math.Abs(margin-0.5) > 0.001 {
		t.Errorf("Margin() = %.3f, want 0.5 - both edges drop by half", margin)
	}
	if edge.From == "" || edge.To == "" {
		t.Errorf("Margin() edge = %+v, want it to name the tightest edge", edge)
	}
}

// The compiler refuses a cycle in production imports, so a check there could
// never fire. The test graph is where a Go repository can actually have one.
func TestTestCycles_findsACycleThroughAnExternalTestPackage(t *testing.T) {
	pkgs := []golist.Package{
		{ImportPath: "m/internal/a", XTestImports: []string{"m/internal/b"}},
		{ImportPath: "m/internal/b", Imports: []string{"m/internal/a"}},
	}

	got := depgraph.TestCycles(module, pkgs)

	if len(got) == 0 {
		t.Fatal("TestCycles() found nothing, but a_test -> b -> a is a cycle")
	}
	if !strings.Contains(strings.Join(got[0], " "), "m/internal/a") {
		t.Errorf("cycle = %v, want it to name m/internal/a", got[0])
	}
}

// And it must not invent one where the tests merely depend downward.
func TestTestCycles_isQuietWhenTestsDependDownward(t *testing.T) {
	pkgs := []golist.Package{
		{ImportPath: "m/internal/a", TestImports: []string{"m/internal/b"}},
		{ImportPath: "m/internal/b"},
	}

	if got := depgraph.TestCycles(module, pkgs); len(got) != 0 {
		t.Errorf("TestCycles() = %v, want none", got)
	}
}

// The report is the deliverable when the gate is clean: the table is what goes
// into docs/ARCHITECTURE.md, and the margin is what a reader watches.
func TestReport_printsTheTableAndTheMargin(t *testing.T) {
	var out strings.Builder

	depgraph.Report(&out, depgraph.Build(module, chain()))

	text := out.String()
	for _, want := range []string{"Ca", "Ce", "paths", "margin"} {
		if !strings.Contains(text, want) {
			t.Errorf("report does not mention %q:\n%s", want, text)
		}
	}
}

// Regression, issue #263: `package foo_test` importing `foo` is the standard
// external-test-package idiom, and go list reports it in XTestImports. Counted
// naively it makes every tested package in the repository look like a one-node
// cycle - which is what the first run of this check reported, 23 times.
func TestTestCycles_aPackagesOwnExternalTestIsNotACycle(t *testing.T) {
	pkgs := []golist.Package{
		{ImportPath: "m/internal/a", XTestImports: []string{"m/internal/a"}},
	}

	if got := depgraph.TestCycles(module, pkgs); len(got) != 0 {
		t.Errorf("TestCycles() = %v, want none; a_test importing a is the idiom", got)
	}
}

// Short is what makes both the table and the cycle report readable.
func TestShort_dropsTheModulePrefix(t *testing.T) {
	got := depgraph.Short("m/internal/ui", "m/cmd/omatty")

	if got[0] != "internal/ui" {
		t.Errorf("Short()[0] = %q, want internal/ui", got[0])
	}
	if got[1] != "m/cmd/omatty" {
		t.Errorf("Short()[1] = %q, want the path left alone outside internal/", got[1])
	}
}
