package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
)

// The JSON is the only format these print, so the flag is asked for rather
// than assumed: without it the command says what it wants (#653).
func TestReadCommand_wantsJSON_issue653(t *testing.T) {
	err := readCommand("sessions", nil, t.TempDir(), statestore.NewStore(filepath.Join(t.TempDir(), "state.json")), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--json") {
		t.Errorf("err = %v, want one naming --json", err)
	}
}

// `omatty status --json` end to end through cmd's wiring: the real store, the
// real transcript reader, the claude profile's path and parser. A session
// whose transcript ends in an unanswered prompt reads thinking (#653).
func TestReadCommand_statusReadsARealTranscript_issue653(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	store := statestore.NewStore(filepath.Join(home, "state.json"))
	st := session.State{Version: session.Version,
		Projects: []session.Project{{Name: "p", Root: dir}},
		Sessions: []session.Session{{ID: "s1", Project: "p", Title: "one", Dir: dir}}}
	if err := store.Save(context.Background(), st); err != nil {
		t.Fatal(err)
	}
	path := paths.Transcript(home, mustResolve(t, dir), "s1")
	line := `{"type":"user","timestamp":"2026-09-02T12:00:05Z","message":{"role":"user","content":"hi"}}`
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := readCommand("status", []string{"--json"}, home, store, &out); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0]["status"] != "thinking" {
		t.Errorf("status --json = %s (%v), want s1 thinking", out.String(), err)
	}
}

// mustResolve is dir with its symlinks resolved, the way claude names its
// transcript directory (#564): t.TempDir is behind one on macOS.
func mustResolve(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
