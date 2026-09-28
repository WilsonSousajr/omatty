package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Every probe in testdata/ compiles. testdata/ is outside ./... by Go's own
// convention, so vet, lint and the tests never build it, and a probe that
// calls an API the code no longer has breaks in silence until a person next
// runs it - which is when it is needed. #452 removed forge.NewCLI, the one
// thing testdata/forgeprobe called; this is what would have said so.
func TestProbes_EveryTestdataProbeBuilds_issue452(t *testing.T) {
	root := repoRoot(t)
	probes, err := filepath.Glob(filepath.Join(root, "testdata", "*", "main.go"))
	if err != nil || len(probes) == 0 {
		t.Fatalf("found no probes under testdata/ (%v)", err)
	}
	for _, main := range probes {
		dir := "./" + filepath.ToSlash(mustRel(t, root, filepath.Dir(main)))
		cmd := exec.Command("go", "build", "-o", os.DevNull, dir)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("go build %s: %v\n%s", dir, err, out)
		}
	}
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}
