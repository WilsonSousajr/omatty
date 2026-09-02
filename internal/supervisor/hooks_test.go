package supervisor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/supervisor"
)

// Regression, issue #31: Command passes --settings at this path, and claude
// refuses to start when the file is absent ("Error: Settings file not found"),
// so every session had a dead PTY behind its sidebar row.
func TestEnsureHooksFile_CreatesValidJSONWhenAbsent_issue31(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "hooks.json")

	if err := supervisor.EnsureHooksFile(path); err != nil {
		t.Fatalf("EnsureHooksFile() error = %v, want nil", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hooks file was not created: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Errorf("hooks file is not valid JSON: %v\n%s", err, b)
	}
	if _, ok := parsed["hooks"]; !ok {
		t.Errorf("hooks file has no hooks key, so #17 has nothing to fill in:\n%s", b)
	}
}

// The file is omatty's, but an operator may still have edited it. Never
// clobber what is already there.
func TestEnsureHooksFile_DoesNotOverwriteAnExistingFile_issue31(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	want := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"true"}]}]}}`
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := supervisor.EnsureHooksFile(path); err != nil {
		t.Fatalf("EnsureHooksFile() error = %v, want nil", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("existing hooks file was overwritten:\ngot  %s\nwant %s", got, want)
	}
}

func TestEnsureHooksFile_UnwritableDirectoryNamesThePath_issue31(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")

	err := supervisor.EnsureHooksFile(path)

	if err == nil {
		t.Fatal("EnsureHooksFile() into a read-only directory returned nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending path", err)
	}
}
