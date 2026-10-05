package status_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// Every transcript line carries the directory claude was working in; status
// ignored it, so omatty never knew claude had moved into a worktree (#659).
func TestParseEntry_readsTheWorkingDirectory_issue659(t *testing.T) {
	line := `{"type":"assistant","cwd":"/r/.worktrees/x","timestamp":"2026-10-01T00:00:00Z",` +
		`"message":{"id":"m1","stop_reason":"end_turn","content":[{"type":"text","text":"done"}]}}`

	e, ok := status.ParseEntry([]byte(line))

	if !ok || e.Cwd != "/r/.worktrees/x" {
		t.Errorf("ParseEntry = %+v, %v; want Cwd /r/.worktrees/x", e, ok)
	}
}

// A status event carries the directory of the last line read, so the TUI
// learns it at the turn's end with no second reader (#659).
func TestTailer_aStatusEventCarriesTheLastWorkingDirectory_issue659(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan dstatus.Event, 8)
	tl := status.Tail("s1", transcript.NewReader(path), sink, time.Now, time.Hour, status.ClaudeAdapter())
	defer tl.Close()
	lines := `{"type":"user","cwd":"/r","timestamp":"2026-10-01T00:00:01Z","message":{"role":"user","content":"go"}}` + "\n" +
		`{"type":"assistant","cwd":"/r/.worktrees/x","timestamp":"2026-10-01T00:00:02Z",` +
		`"message":{"id":"m1","stop_reason":"end_turn","content":[{"type":"text","text":"done"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	tl.Poll()

	got := statusEvents(drain(sink))
	if len(got) == 0 || got[len(got)-1].Cwd != "/r/.worktrees/x" {
		t.Errorf("status events = %+v, want the last to carry Cwd /r/.worktrees/x", got)
	}
}
