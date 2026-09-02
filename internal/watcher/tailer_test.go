package watcher_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func drain(sink chan watcher.Event) []watcher.Event {
	var out []watcher.Event
	for {
		select {
		case e := <-sink:
			out = append(out, e)
		default:
			return out
		}
	}
}

func statusEvents(evs []watcher.Event) []watcher.Event {
	var out []watcher.Event
	for _, e := range evs {
		if e.Kind != watcher.UsageUpdated {
			out = append(out, e)
		}
	}
	return out
}

func lastUsage(evs []watcher.Event) (watcher.Tokens, bool) {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Kind == watcher.UsageUpdated {
			return evs[i].Tokens, true
		}
	}
	return watcher.Tokens{}, false
}

const promptLine = `{"type":"user","timestamp":"2026-09-02T12:00:01Z","message":{"role":"user","content":"hi"}}` + "\n"

func TestTailer_EmitsStatusWhenTheFileGrows_issue19(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()

	if err := os.WriteFile(path, []byte(promptLine), 0o600); err != nil {
		t.Fatal(err)
	}
	tl.Poll()

	got := statusEvents(drain(sink))
	if len(got) == 0 {
		t.Fatal("no status event after the file grew")
	}
	last := got[len(got)-1]
	if last.SessionID != "s1" || last.Kind != watcher.PromptSubmitted {
		t.Errorf("event = %+v, want session s1 PromptSubmitted", last)
	}
}

func TestTailer_EventTimeIsTheEntryTimestampNotNow_issue19(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	// A clock far from the entry's own timestamp: the status event must carry
	// the entry's time so newer-wins compares like with like.
	future := func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	tl := watcher.Tail("s1", path, sink, future, time.Hour)
	defer tl.Close()
	_ = os.WriteFile(path, []byte(promptLine), 0o600)

	tl.Poll()

	want, _ := time.Parse(time.RFC3339, "2026-09-02T12:00:01Z")
	for _, e := range statusEvents(drain(sink)) {
		if !e.At.Equal(want) {
			t.Errorf("status event time = %v, want the entry's %v", e.At, want)
		}
	}
}

func TestTailer_EmitsCumulativeUsage_issue39(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()
	line := `{"type":"assistant","timestamp":"2026-09-02T12:00:02Z","message":{"stop_reason":"end_turn","content":[{"type":"text","text":"x"}],"usage":{"input_tokens":40,"output_tokens":8}}}` + "\n"
	_ = os.WriteFile(path, []byte(line+line), 0o600)

	tl.Poll()

	tok, ok := lastUsage(drain(sink))
	if !ok || tok.In != 80 || tok.Out != 16 {
		t.Errorf("cumulative usage = %+v (found=%v), want In 80 Out 16", tok, ok)
	}
}

// Regression, issue #59: one response is written as one line per content
// block, each repeating the response's usage; summing per line doubled the
// counts in the header. usage.jsonl has msg_a on three lines.
func TestTailer_CountsUsageOncePerResponse_issue59(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "transcripts", "usage.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	_ = os.WriteFile(path, fixture, 0o600)
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()

	tl.Poll()

	tok, ok := lastUsage(drain(sink))
	want := watcher.Tokens{In: 1010, Out: 201, CacheRead: 302, CacheWrite: 403}
	if !ok || tok != want {
		t.Errorf("usage = %+v (found=%v), want %+v: msg_a's three lines must count once", tok, ok, want)
	}
}

func TestTailer_MissingFileIsNotAnError_issue19(t *testing.T) {
	sink := make(chan watcher.Event, 1)
	tl := watcher.Tail("s1", filepath.Join(t.TempDir(), "never"), sink, time.Now, time.Hour)
	defer tl.Close()

	tl.Poll() // must not panic

	if len(drain(sink)) != 0 {
		t.Error("a missing transcript produced events")
	}
}

func TestTailer_ReadsOnlyNewBytes_issue19(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 16)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()
	first := `{"type":"assistant","timestamp":"2026-09-02T12:00:01Z","message":{"stop_reason":"tool_use","content":[{"type":"tool_use","id":"a","name":"Read","input":{}}],"usage":{"input_tokens":10,"output_tokens":1}}}` + "\n"
	_ = os.WriteFile(path, []byte(first), 0o600)
	tl.Poll()
	_ = drain(sink)

	second := `{"type":"user","timestamp":"2026-09-02T12:00:05Z","message":{"content":"more"}}` + "\n"
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString(second)
	_ = f.Close()
	tl.Poll()

	got := statusEvents(drain(sink))
	if len(got) == 0 {
		t.Fatal("no event after the append")
	}
	if last := got[len(got)-1]; last.Kind != watcher.PromptSubmitted {
		t.Errorf("second poll derived %v, want PromptSubmitted from only the new line", last.Kind)
	}
	// The cumulative usage must still reflect only the one assistant entry:
	// re-reading the whole file would double it.
	if tok, ok := lastUsage(append(got, drain(sink)...)); ok && tok.In > 10 {
		t.Errorf("usage In = %d, want 10; the file was re-read from the start", tok.In)
	}
}

func TestTailer_HandlesTruncation_issue19(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 16)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()
	long := promptLine + promptLine + promptLine
	_ = os.WriteFile(path, []byte(long), 0o600)
	tl.Poll()
	_ = drain(sink)

	// Truncate to something shorter, then write a fresh line.
	tool := `{"type":"assistant","timestamp":"2026-09-02T12:00:09Z","message":{"stop_reason":"tool_use","content":[{"type":"tool_use","id":"z","name":"Edit","input":{}}]}}` + "\n"
	_ = os.WriteFile(path, []byte(tool), 0o600)
	tl.Poll()

	got := statusEvents(drain(sink))
	if len(got) == 0 || got[len(got)-1].Kind != watcher.ToolStarted {
		t.Errorf("after truncation, derived %v, want ToolStarted from the rewritten file", got)
	}
}

// Regression, issue #64: one unterminated or oversized line was read and
// buffered whole, so a giant tool result could take the process down with
// every session in it. A line past the cap is dropped; the lines around it
// still count.
func TestTailer_DropsALineOverTheCapAndKeepsGoing_issue64(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()
	huge := `{"type":"user","timestamp":"2026-09-02T12:00:09Z","message":{"content":"` +
		strings.Repeat("x", 2<<20) + `"}}` + "\n"
	_ = os.WriteFile(path, []byte(promptLine+huge), 0o600)

	tl.Poll()

	got := statusEvents(drain(sink))
	want, _ := time.Parse(time.RFC3339, "2026-09-02T12:00:01Z")
	if len(got) == 0 || !got[len(got)-1].At.Equal(want) {
		t.Errorf("derived %+v, want PromptSubmitted at %v: the 2 MiB line must be dropped, not read", got, want)
	}
}
