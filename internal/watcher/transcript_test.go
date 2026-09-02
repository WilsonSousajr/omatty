package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
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

func TestDeriveFromTail_Fixtures_issue19(t *testing.T) {
	at := func(s string) time.Time { tm, _ := time.Parse(time.RFC3339, s); return tm }
	tests := []struct {
		fixture string
		want    registry.Status
		wantAt  time.Time
	}{
		{"prompt-sent.jsonl", registry.StatusThinking, at("2026-09-02T12:00:01Z")},
		{"tool-running.jsonl", registry.StatusTool, at("2026-09-02T12:00:02Z")},
		{"tool-returned.jsonl", registry.StatusThinking, at("2026-09-02T12:00:03Z")},
		{"turn-ended.jsonl", registry.StatusDone, at("2026-09-02T12:00:04Z")},
		{"noise-only.jsonl", registry.StatusIdle, time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			status, ts := watcher.DeriveFromTail(loadFixture(t, tt.fixture))
			if status != tt.want {
				t.Errorf("status = %q, want %q", status, tt.want)
			}
			if !ts.Equal(tt.wantAt) {
				t.Errorf("timestamp = %v, want %v", ts, tt.wantAt)
			}
		})
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
