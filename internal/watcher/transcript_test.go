package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func loadFixture(t *testing.T, name string) []watcher.Entry {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "transcripts", name))
	if err != nil {
		t.Fatal(err)
	}
	var out []watcher.Entry
	for _, line := range splitLines(b) {
		if e, ok := watcher.ParseEntry(line); ok {
			out = append(out, e)
		}
	}
	return out
}

func splitLines(b []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				lines = append(lines, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		lines = append(lines, b[start:])
	}
	return lines
}

func kindAt(t *testing.T, fixture string) (watcher.Kind, time.Time, bool) {
	t.Helper()
	return watcher.DeriveKind(loadFixture(t, fixture))
}

func at(s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return tm
}

func TestDeriveKind_Fixtures_issue19(t *testing.T) {
	tests := []struct {
		fixture string
		want    watcher.Kind
		wantAt  time.Time
		ok      bool
	}{
		{"prompt-sent.jsonl", watcher.PromptSubmitted, at("2026-09-02T12:00:01Z"), true},
		{"tool-running.jsonl", watcher.ToolStarted, at("2026-09-02T12:00:02Z"), true},
		{"tool-returned.jsonl", watcher.PromptSubmitted, at("2026-09-02T12:00:03Z"), true},
		{"turn-ended.jsonl", watcher.TurnEnded, at("2026-09-02T12:00:04Z"), true},
		{"noise-only.jsonl", 0, time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			kind, ts, ok := watcher.DeriveKind(loadFixture(t, tt.fixture))
			if ok != tt.ok || (ok && (kind != tt.want || !ts.Equal(tt.wantAt))) {
				t.Errorf("DeriveKind = (%v, %v, %v), want (%v, %v, %v)", kind, ts, ok, tt.want, tt.wantAt, tt.ok)
			}
		})
	}
}

// Regression, issue #61: entries claude injects as the user - a finished
// background task, a local command and its output, isMeta context - were
// read as typed prompts and flipped a finished session back to thinking.
func TestDeriveKind_IgnoresInjectedUserEntries_issue61(t *testing.T) {
	kind, ts, ok := kindAt(t, "injected-after-done.jsonl")

	if !ok || kind != watcher.TurnEnded || !ts.Equal(at("2026-09-02T12:00:04Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (TurnEnded, 12:00:04, true): injected entries are not prompts", kind, ts, ok)
	}
}

// Regression, issue #62: a prompt sent as a list of text (and image) blocks
// set neither flag, so the tail skipped it and status and age stayed on the
// previous turn.
func TestDeriveKind_ListOfTextIsAPrompt_issue62(t *testing.T) {
	kind, ts, ok := kindAt(t, "list-text-prompt.jsonl")

	if !ok || kind != watcher.PromptSubmitted || !ts.Equal(at("2026-09-02T12:05:00Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (PromptSubmitted, 12:05:00, true)", kind, ts, ok)
	}
}

// Regression, issue #63: only end_turn counted as a finished turn, so a
// response stopped at max_tokens left the session at thinking forever.
func TestDeriveKind_AnyStopReasonButToolUseEndsTheTurn_issue63(t *testing.T) {
	kind, ts, ok := kindAt(t, "stopped-at-max-tokens.jsonl")

	if !ok || kind != watcher.TurnEnded || !ts.Equal(at("2026-09-02T12:00:05Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (TurnEnded, 12:00:05, true)", kind, ts, ok)
	}
}

func TestParseEntry_CarriesTheMessageID_issue59(t *testing.T) {
	e, ok := watcher.ParseEntry([]byte(
		`{"type":"assistant","timestamp":"2026-09-02T12:00:00Z","message":{"id":"msg_x","role":"assistant","content":[{"type":"text","text":"hi"}]}}`))
	if !ok || e.MessageID != "msg_x" {
		t.Errorf("entry = %+v, want MessageID msg_x", e)
	}
}

func TestSumUsage_AddsAllFourCountersAcrossEntries_issue39(t *testing.T) {
	got := watcher.SumUsage(loadFixture(t, "usage.jsonl"))

	want := watcher.Tokens{In: 1110, Out: 221, CacheRead: 332, CacheWrite: 443}
	if got != want {
		t.Errorf("SumUsage = %+v, want %+v", got, want)
	}
}

func TestParseEntry_DropsNoise_issue19(t *testing.T) {
	for _, line := range []string{
		`{"type":"attachment","attachment":{"type":"skill_listing"}}`,
		`{"type":"queue-operation","operation":"enqueue"}`,
		`{"type":"ai-title"}`,
		`{"type":"file-history-snapshot"}`,
		`not json at all`,
	} {
		if _, ok := watcher.ParseEntry([]byte(line)); ok {
			t.Errorf("ParseEntry kept a line status does not need: %s", line)
		}
	}
}

func TestParseEntry_UserStringContentIsAPrompt_issue19(t *testing.T) {
	e, ok := watcher.ParseEntry([]byte(
		`{"type":"user","timestamp":"2026-09-02T12:00:00Z","message":{"role":"user","content":"hello"}}`))
	if !ok || !e.UserIsPrompt {
		t.Errorf("a user string message = %+v, want a prompt", e)
	}
	if e.ToolResult {
		t.Error("a typed prompt was misread as a tool result")
	}
}

func TestParseEntry_UserListContentWithToolResult_issue19(t *testing.T) {
	e, ok := watcher.ParseEntry([]byte(
		`{"type":"user","timestamp":"2026-09-02T12:00:00Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":"ok"}]}}`))
	if !ok || e.UserIsPrompt || !e.ToolResult {
		t.Errorf("a tool-result message = %+v, want ToolResult and not a prompt", e)
	}
}

func TestParseEntry_AssistantToolUseSetsTheFlagAndUsage_issue19(t *testing.T) {
	e, ok := watcher.ParseEntry([]byte(
		`{"type":"assistant","timestamp":"2026-09-02T12:00:00Z","message":{"role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"t","name":"Bash","input":{}}],"usage":{"input_tokens":5,"output_tokens":1}}}`))
	if !ok || !e.ToolUse || e.StopReason != "tool_use" {
		t.Errorf("assistant tool_use = %+v, want ToolUse and stop_reason tool_use", e)
	}
	if e.Usage.In != 5 || e.Usage.Out != 1 {
		t.Errorf("usage = %+v, want In 5 Out 1", e.Usage)
	}
}
