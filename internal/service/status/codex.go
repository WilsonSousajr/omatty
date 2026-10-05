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
	// codexToolType is a tool call or its output, from the rollout's
	// response items: claude's ToolUse and ToolResult, without usage.
	codexToolType = "tool"

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

// ParseEntry reads one rollout line: an event_msg that moves the status or
// carries usage, or a response_item that is a tool call or its output.
//
//	e, ok := status.CodexAdapter().ParseEntry(line)
func (codexStatus) ParseEntry(line []byte) (dstatus.Entry, bool) {
	var l codexLine
	if json.Unmarshal(line, &l) != nil {
		return dstatus.Entry{}, false
	}
	switch l.Type {
	case "event_msg":
		return codexEvent(l)
	case "response_item":
		return codexTool(l)
	}
	return dstatus.Entry{}, false
}

// codexEvent reads the event_msg payloads status needs.
func codexEvent(l codexLine) (dstatus.Entry, bool) {
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

// codexTool reads a tool call or its output. Calls and their outputs come in
// two spellings, function_call and custom_tool_call.
func codexTool(l codexLine) (dstatus.Entry, bool) {
	switch l.Payload.Type {
	case "function_call", "custom_tool_call":
		return dstatus.Entry{Type: codexToolType, At: l.Timestamp, ToolUse: true}, true
	case "function_call_output", "custom_tool_call_output":
		return dstatus.Entry{Type: codexToolType, At: l.Timestamp, ToolResult: true}, true
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

// DeriveKind is the status the newest relevant entry implies. A tail of
// usage alone is a long turn whose start the tailer's ring has dropped:
// busy, never "no status" (#152's review).
//
//	kind, at, ok := status.CodexAdapter().DeriveKind(entries)
func (codexStatus) DeriveKind(entries []dstatus.Entry) (dstatus.Kind, time.Time, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		if kind, ok := codexKindOf(entries[i]); ok {
			return kind, entries[i].At, true
		}
	}
	if n := len(entries); n > 0 {
		return dstatus.PromptSubmitted, entries[n-1].At, true
	}
	return 0, time.Time{}, false
}

// codexKindOf is the status one entry implies, if any: a tool's output is
// the model thinking again, as a tool_result is for claude.
func codexKindOf(e dstatus.Entry) (dstatus.Kind, bool) {
	switch {
	case e.Type == "user" || e.ToolResult:
		return dstatus.PromptSubmitted, true
	case e.ToolUse:
		return dstatus.ToolStarted, true
	case e.StopReason == codexEndedStop:
		return dstatus.TurnEnded, true
	case e.StopReason == codexAbortedStop:
		return dstatus.Idle, true
	}
	return 0, false
}

// codexKindByEvent is every hook event codex's -c hooks subscribe to.
// Interrupt is what codex fires on Esc in place of Stop: idle, not a turn's
// end, as turn_aborted is. PostToolUse is what moves a card off "waiting"
// once the operator approves a command (#152's review).
var codexKindByEvent = map[string]dstatus.Kind{
	"SessionStart":      dstatus.SessionStarted,
	"UserPromptSubmit":  dstatus.PromptSubmitted,
	"PreToolUse":        dstatus.ToolStarted,
	"PostToolUse":       dstatus.ToolFinished,
	"PermissionRequest": dstatus.PermissionRequested,
	"Stop":              dstatus.TurnEnded,
	"Interrupt":         dstatus.Idle,
}

// KindOf maps a codex hook payload. A SessionStart that begins a
// conversation - the first prompt's startup, or /clear - re-binds the pane,
// because codex takes no id from omatty and reports its own (Reported,
// #523). A resume keeps the id the row already holds. Only the pane's main
// thread reaches here with a startup: codex runs no SessionStart for a
// subagent (SubagentStart instead, or nothing for an internal one -
// core/src/hook_runtime.rs), and the one background thread that does, the
// memories writer, carries no transcript_path and is dropped by
// hooks.ParseCodexPayload.
//
//	kind, ok := status.CodexAdapter().KindOf(payload)
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
