package supervisor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/supervisor"
)

func TestWriteHooksFile_CreatesTheFileAndParentDir_issue17(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "hooks.json")

	if err := supervisor.WriteHooksFile(path, []byte(`{"hooks":{}}`)); err != nil {
		t.Fatalf("WriteHooksFile() error = %v, want nil", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != `{"hooks":{}}` {
		t.Errorf("file = %q, err = %v; want the content written", got, err)
	}
}

// Replaces TestEnsureHooksFile_DoesNotOverwriteAnExistingFile_issue31. That
// test asserted the file was never rewritten, which was correct only for the
// #31 stub. The file now names the omatty binary by absolute path, which moves
// with `go install`, so a stale path must be replaced (invariant 11 depends on
// the hook actually reaching a running omatty).
func TestWriteHooksFile_OverwritesEveryTime_issue17(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":"OLD BINARY PATH"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := supervisor.WriteHooksFile(path, []byte(`{"hooks":{}}`)); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "OLD BINARY PATH") {
		t.Errorf("the stale hooks file was not overwritten:\n%s", got)
	}
}

func TestWriteHooksFile_UnwritableDirectoryNamesThePath_issue17(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")

	err := supervisor.WriteHooksFile(path, []byte("{}"))

	if err == nil {
		t.Fatal("WriteHooksFile() into a read-only dir returned nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending path", err)
	}
}
