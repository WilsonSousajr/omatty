package fsread_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/fsread"
)

// codex names a rollout after its launch time, under date directories, so
// the path cannot be derived from the id; it is found by the id's suffix
// (#152, docs/research/agents/codex.md).
func TestCodexRollout_FindsTheConversationsFileByItsId_issue152(t *testing.T) {
	store := t.TempDir()
	day := filepath.Join(store, "sessions", "2026", "10", "04")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(day, "rollout-2026-10-04T06-50-16-01a10652-4044-7521-927f-fe1fdaed5e18.jsonl")
	other := filepath.Join(day, "rollout-2026-10-04T06-51-09-01a10653-1210-7910-b53c-987893f66767.jsonl")
	mustWrite(t, want, "{}\n")
	mustWrite(t, other, "{}\n")

	got, ok := fsread.CodexRollout(store, "01a10652-4044-7521-927f-fe1fdaed5e18")

	if !ok || got != want {
		t.Errorf("CodexRollout = %q, %v; want %q", got, ok, want)
	}
}

// A conversation codex has not written yet - a pane nobody has typed into -
// is not found; nor is an id that would match as a glob (#152).
func TestCodexRollout_NoFileIsNotFound_issue152(t *testing.T) {
	store := t.TempDir()
	if got, ok := fsread.CodexRollout(store, "01a10652-4044-7521-927f-fe1fdaed5e18"); ok {
		t.Errorf("CodexRollout found %q in an empty store", got)
	}
	day := filepath.Join(store, "sessions", "2026", "10", "04")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(day, "rollout-x-abc.jsonl"), "{}\n")
	if got, ok := fsread.CodexRollout(store, "*"); ok {
		t.Errorf("CodexRollout(*) matched %q, want a glob character refused", got)
	}
}
