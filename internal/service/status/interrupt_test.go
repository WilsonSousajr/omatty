package status_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// Regression, issue #737: an interrupt (Esc mid-turn) is written as a user
// entry, "[Request interrupted by user]", and no Stop hook follows it. Read
// as a typed prompt, it left the card spinning until the next prompt. It is
// the turn's end, in either content shape and either wording.
func TestDeriveKind_AnInterruptEndsTheTurn_issue737(t *testing.T) {
	for _, content := range []string{
		`"[Request interrupted by user]"`,
		`[{"type":"text","text":"[Request interrupted by user]"}]`,
		`[{"type":"text","text":"[Request interrupted by user for tool use]"}]`,
	} {
		line := `{"type":"user","timestamp":"2026-09-02T12:00:05Z","message":{"role":"user","content":` + content + `}}`
		e, ok := status.ParseEntry([]byte(line))
		if !ok || e.UserIsPrompt {
			t.Errorf("interrupt %s parsed as %+v, want a user entry that is not a prompt", content, e)
		}
		kind, _, _ := status.DeriveKind([]dstatus.Entry{e})
		if kind != dstatus.TurnEnded {
			t.Errorf("interrupt %s: DeriveKind = %v, want TurnEnded", content, kind)
		}
	}
}

// Regression, issue #737: an interrupt mid-tool, after the tool_result claude
// writes for the refused call, still ends the turn.
func TestDeriveKind_AnInterruptAfterAToolCallEndsTheTurn_issue737(t *testing.T) {
	kind, ts, ok := kindAt(t, "interrupted.jsonl")

	if !ok || kind != dstatus.TurnEnded || !ts.Equal(at("2026-09-02T12:00:03.5Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (TurnEnded, 12:00:03.5, true)", kind, ts, ok)
	}
}

// Regression, issue #737: the summary /compact writes is a user entry marked
// isCompactSummary. It is not a prompt and moves nothing: the turn before it
// stays ended.
func TestDeriveKind_ACompactSummaryIsNotAPrompt_issue737(t *testing.T) {
	kind, ts, ok := kindAt(t, "compacted.jsonl")

	if !ok || kind != dstatus.TurnEnded || !ts.Equal(at("2026-09-02T12:00:04Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (TurnEnded, 12:00:04, true)", kind, ts, ok)
	}
}

// Regression, issue #737: adoption titles a session with its first typed
// prompt, and an interrupt marker is not one.
func TestPromptText_anInterruptIsNotAPrompt_issue737(t *testing.T) {
	for _, content := range []string{
		`"[Request interrupted by user]"`,
		`[{"type":"text","text":"[Request interrupted by user for tool use]"}]`,
	} {
		if got, ok := status.PromptText(json.RawMessage(content)); ok {
			t.Errorf("PromptText(%s) = (%q, true), want not a prompt", content, got)
		}
	}
}

// Regression, issue #737: the tailer, fed a turn interrupted mid-tool, must
// leave the session not working - the spinner runs on thinking and tool.
func TestTailer_AnInterruptLeavesTheSessionNotWorking_issue737(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "transcripts", "interrupted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan dstatus.Event, 8)
	tl := status.Tail("s1", transcript.NewReader(path), sink, time.Now, time.Hour, status.ClaudeAdapter())
	defer tl.Close()
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	tl.Poll()

	got := statusEvents(drain(sink))
	if len(got) == 0 || got[len(got)-1].Kind != dstatus.TurnEnded {
		t.Errorf("events = %+v, want the last to be TurnEnded", got)
	}
}
