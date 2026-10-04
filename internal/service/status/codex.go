// Codex's transcript and hook shapes (#152), read off a real rollout in the
// #528 spike (docs/research/agents/codex.md). A rollout line is
// {"timestamp","type","payload"}; status needs four event_msg payloads:
// task_started (busy), task_complete (the turn's end), turn_aborted (Esc:
// idle, not a turn's end) and token_count (usage).

package status

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// The Entry types the codex adapter writes. "user" and "assistant" keep the
// tailer's meaning - only an assistant entry's usage is counted - and
// codexTurnType is a turn's end, which carries no usage. Were the end an
// assistant entry, its empty id would reset the tailer's dedupe and a
// token_count repeated after it would count twice.
const (
	codexTurnType = "turn"

	codexStartedStop = "task_started"
	codexEndedStop   = "task_complete"
	codexAbortedStop = "turn_aborted"
)

type codexLine struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Payload   struct {
		Type string          `json:"type"`
		Info *codexUsageInfo `json:"info"`
	} `json:"payload"`
}

type codexUsageInfo struct {
	Total codexTokens `json:"total_token_usage"`
	Last  codexTokens `json:"last_token_usage"`
}

type codexTokens struct {
	Input      int `json:"input_tokens"`
	Cached     int `json:"cached_input_tokens"`
	CacheWrite int `json:"cache_write_input_tokens"`
	Output     int `json:"output_tokens"`
	Total      int `json:"total_tokens"`
}

// codexStatus is the Adapter for codex's rollout and hooks.
type codexStatus struct{}

// CodexAdapter is the Adapter for codex's rollout JSONL and hook payloads.
//
//	tl := status.Tail(id, src, sink, time.Now, time.Second, status.CodexAdapter())
func CodexAdapter() dstatus.Adapter { return codexStatus{} }

func (codexStatus) ParseEntry(line []byte) (dstatus.Entry, bool) {
	var l codexLine
	if json.Unmarshal(line, &l) != nil || l.Type != "event_msg" {
		return dstatus.Entry{}, false
	}
	switch l.Payload.Type {
	case codexStartedStop:
		return dstatus.Entry{Type: "user", At: l.Timestamp, UserIsPrompt: true}, true
	case codexEndedStop, codexAbortedStop:
		return dstatus.Entry{Type: codexTurnType, At: l.Timestamp, StopReason: l.Payload.Type}, true
	case "token_count":
		return codexUsage(l)
	}
	return dstatus.Entry{}, false
}

// codexUsage is one response's usage in claude's split: codex's input
// includes the cached tokens, which omatty counts as cache reads. The
// cumulative total names the response, so a token_count written twice for
// it is counted once (#59's dedupe). info is null on a rate-limit refresh.
func codexUsage(l codexLine) (dstatus.Entry, bool) {
	if l.Payload.Info == nil {
		return dstatus.Entry{}, false
	}
	u := l.Payload.Info.Last
	return dstatus.Entry{Type: "assistant", At: l.Timestamp,
		MessageID: "codex-total-" + strconv.Itoa(l.Payload.Info.Total.Total),
		Usage:     dstatus.Tokens{In: u.Input - u.Cached, Out: u.Output, CacheRead: u.Cached, CacheWrite: u.CacheWrite}}, true
}

func (codexStatus) DeriveKind(entries []dstatus.Entry) (dstatus.Kind, time.Time, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		switch {
		case e.Type == "user":
			return dstatus.PromptSubmitted, e.At, true
		case e.StopReason == codexEndedStop:
			return dstatus.TurnEnded, e.At, true
		case e.StopReason == codexAbortedStop:
			return dstatus.Idle, e.At, true
		}
	}
	return 0, time.Time{}, false
}

// codexKindByEvent is every hook event codex's -c hooks subscribe to.
// Interrupt is what codex fires on Esc in place of Stop: idle, not a turn's
// end, as turn_aborted is.
var codexKindByEvent = map[string]dstatus.Kind{
	"SessionStart":      dstatus.SessionStarted,
	"UserPromptSubmit":  dstatus.PromptSubmitted,
	"PermissionRequest": dstatus.PermissionRequested,
	"Stop":              dstatus.TurnEnded,
	"Interrupt":         dstatus.Idle,
}

// KindOf maps a codex hook payload. A SessionStart that begins a
// conversation - the first prompt's startup, or /clear - re-binds the pane,
// because codex takes no id from omatty and reports its own (Reported,
// #523). A resume keeps the id the row already holds.
func (codexStatus) KindOf(p dstatus.HookPayload) (dstatus.Kind, bool) {
	if p.HookEventName == "SessionStart" && (p.Source == "startup" || p.Source == "clear") {
		return dstatus.SessionRebound, true
	}
	kind, ok := codexKindByEvent[p.HookEventName]
	return kind, ok
}

// CodexHookEventNames is every event codex's hooks are declared for: the
// ones KindOf maps, so the declaration and the listener cannot drift (#78).
// Sorted, so the rendered arguments are stable.
//
//	args, _ := hooks.RenderCodexArgs(bin, status.CodexHookEventNames())
func CodexHookEventNames() []string {
	names := make([]string, 0, len(codexKindByEvent))
	for name := range codexKindByEvent {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
