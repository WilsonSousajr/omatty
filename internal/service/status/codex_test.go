package status_test

import (
	"bufio"
	"os"
	"slices"
	"testing"
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// codexEntries parses a fixture's lines through the codex adapter.
func codexEntries(t *testing.T, name string) []dstatus.Entry {
	t.Helper()
	f, err := os.Open("testdata/transcripts/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []dstatus.Entry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if e, ok := status.CodexAdapter().ParseEntry(sc.Bytes()); ok {
			out = append(out, e)
		}
	}
	return out
}

// task_started is busy and task_complete is a turn's end: the pairing the
// #528 spike read off a real rollout. session_meta and response items do
// not move the status.
func TestCodex_ATurnStartsAndEnds_issue152(t *testing.T) {
	entries := codexEntries(t, "codex-turn.jsonl")
	kind, at, ok := status.CodexAdapter().DeriveKind(entries)
	want := time.Date(2026, 10, 4, 9, 50, 25, 60e6, time.UTC)
	if !ok || kind != dstatus.TurnEnded || !at.Equal(want) {
		t.Errorf("DeriveKind = %v at %v, %v; want TurnEnded at %v", kind, at, ok, want)
	}
	kind, _, _ = status.CodexAdapter().DeriveKind(entries[:1])
	if kind != dstatus.PromptSubmitted {
		t.Errorf("after task_started alone = %v, want PromptSubmitted", kind)
	}
}

// Esc ends a turn with turn_aborted and no task_complete. It is idle, not
// a turn's end: a gate must not auto-run on a turn the operator cut short.
func TestCodex_AnInterruptedTurnIsIdle_issue152(t *testing.T) {
	kind, _, ok := status.CodexAdapter().DeriveKind(codexEntries(t, "codex-aborted.jsonl"))
	if !ok || kind != dstatus.Idle {
		t.Errorf("DeriveKind = %v, %v; want Idle", kind, ok)
	}
}

// Usage is each response's last_token_usage, in claude's split: codex's
// input_tokens include the cached ones, which omatty counts as cache reads.
// A token_count with info null (a rate-limit refresh) carries none.
func TestCodex_UsageIsTheResponsesOwn_issue152(t *testing.T) {
	var usage []dstatus.Tokens
	for _, e := range codexEntries(t, "codex-turn.jsonl") {
		if e.Usage != (dstatus.Tokens{}) {
			usage = append(usage, e.Usage)
		}
	}
	want := []dstatus.Tokens{{In: 11044, Out: 6, CacheRead: 4480}}
	if !slices.Equal(usage, want) {
		t.Errorf("usage = %+v, want %+v", usage, want)
	}
	for _, e := range codexEntries(t, "codex-aborted.jsonl") {
		if e.Usage != (dstatus.Tokens{}) {
			t.Errorf("a null-info token_count carried usage %+v", e.Usage)
		}
	}
}

// A token_count written twice for one response - same cumulative total -
// is counted once by the tailer, as claude's repeated message id is (#59).
func TestCodex_ARepeatedTokenCountCountsOnce_issue152(t *testing.T) {
	line := []byte(`{"timestamp":"2026-10-04T09:50:25.044Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":15530},"last_token_usage":{"input_tokens":10,"output_tokens":6,"total_tokens":16}}}}`)
	end := []byte(`{"timestamp":"2026-10-04T09:50:25.060Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"t"}}`)
	src := &onceTranscript{lines: [][]byte{line, end, line}}
	sink := make(chan dstatus.Event, 4)
	tl := status.Tail("s", src, sink, time.Now, time.Hour, status.CodexAdapter())
	defer tl.Close()
	tl.Poll()
	var got dstatus.Tokens
	for len(sink) > 0 {
		if e := <-sink; e.Kind == dstatus.UsageUpdated {
			got = e.Tokens
		}
	}
	if got != (dstatus.Tokens{In: 10, Out: 6}) {
		t.Errorf("usage = %+v, want one response's {In:10 Out:6}", got)
	}
}

// codex's hooks map as claude's do, with two differences the spike found:
// a new conversation in the pane - the first prompt's startup, or /clear -
// re-binds the row, since codex takes no id from omatty (Reported, #523);
// and Interrupt, which codex fires in place of Stop on Esc, is idle.
func TestCodex_HookKinds_issue152(t *testing.T) {
	cases := []struct {
		event, source string
		want          dstatus.Kind
	}{
		{"SessionStart", "startup", dstatus.SessionRebound},
		{"SessionStart", "clear", dstatus.SessionRebound},
		{"SessionStart", "resume", dstatus.SessionStarted},
		{"UserPromptSubmit", "", dstatus.PromptSubmitted},
		{"PermissionRequest", "", dstatus.PermissionRequested},
		{"Stop", "", dstatus.TurnEnded},
		{"Interrupt", "", dstatus.Idle},
	}
	for _, c := range cases {
		got, ok := status.CodexAdapter().KindOf(dstatus.HookPayload{HookEventName: c.event, Source: c.source})
		if !ok || got != c.want {
			t.Errorf("KindOf(%s %s) = %v, %v; want %v", c.event, c.source, got, ok, c.want)
		}
	}
	if _, ok := status.CodexAdapter().KindOf(dstatus.HookPayload{HookEventName: "PreCompact"}); ok {
		t.Error("an event omatty does not subscribe to was mapped")
	}
}

// The events codex's -c hooks subscribe to are exactly the ones KindOf
// maps, so the declaration and the listener cannot drift (#78).
func TestCodexHookEventNames_AreTheMappedOnes_issue152(t *testing.T) {
	want := []string{"Interrupt", "PermissionRequest", "SessionStart", "Stop", "UserPromptSubmit"}
	if got := status.CodexHookEventNames(); !slices.Equal(got, want) {
		t.Errorf("CodexHookEventNames = %v, want %v", got, want)
	}
}
