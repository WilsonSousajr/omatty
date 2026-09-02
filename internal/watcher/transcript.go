package watcher

import (
	"encoding/json"
	"strings"
	"time"
)

// Entry is the slice of a transcript line that status needs. Every other line
// type - attachments, queue operations, titles, snapshots - is dropped at
// parse, so callers only ever hold user and assistant turns.
type Entry struct {
	Type         string // "user" or "assistant"
	MessageID    string // assistant: one API response spans several lines under one id
	At           time.Time
	StopReason   string // assistant
	UserIsPrompt bool   // user: a typed prompt (a string, or text/image blocks)
	ToolUse      bool   // assistant: a tool_use block is present
	ToolResult   bool   // user: a tool_result block is present
	Usage        Tokens // assistant
}

// rawEntry is the on-disk shape. content is deferred so it can be a string
// (a prompt) or a list of blocks (tool results / assistant blocks).
type rawEntry struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	IsMeta    bool      `json:"isMeta"`
	Message   struct {
		ID         string          `json:"id"`
		StopReason string          `json:"stop_reason"`
		Content    json.RawMessage `json:"content"`
		Usage      struct {
			In        int `json:"input_tokens"`
			Out       int `json:"output_tokens"`
			CacheRead int `json:"cache_read_input_tokens"`
			CacheMake int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

type block struct {
	Type string `json:"type"`
}

// ParseEntry parses one transcript line. ok is false for a line status does
// not need, including malformed JSON.
func ParseEntry(line []byte) (Entry, bool) {
	var r rawEntry
	if json.Unmarshal(line, &r) != nil {
		return Entry{}, false
	}
	switch r.Type {
	case "user":
		return parseUser(r), true
	case "assistant":
		return parseAssistant(r), true
	default:
		return Entry{}, false
	}
}

// injectedPrefixes open the user-role entries Claude Code writes itself: a
// finished background task, a local slash command and its output. None is a
// typed prompt, so none may move the status to thinking (issue #61).
var injectedPrefixes = []string{"<task-notification", "<command-", "<local-command-"}

func parseUser(r rawEntry) Entry {
	e := Entry{Type: "user", At: r.Timestamp}
	if r.IsMeta {
		return e // context claude injected, not something the operator typed
	}
	var s string
	if json.Unmarshal(r.Message.Content, &s) == nil {
		e.UserIsPrompt = !isInjected(s)
		return e
	}
	e.ToolResult = hasBlock(r.Message.Content, "tool_result")
	// A prompt with an attachment is a list of text and image blocks, not a
	// string (issue #62).
	e.UserIsPrompt = !e.ToolResult &&
		(hasBlock(r.Message.Content, "text") || hasBlock(r.Message.Content, "image"))
	return e
}

func isInjected(s string) bool {
	for _, p := range injectedPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func parseAssistant(r rawEntry) Entry {
	return Entry{
		Type:       "assistant",
		MessageID:  r.Message.ID,
		At:         r.Timestamp,
		StopReason: r.Message.StopReason,
		ToolUse:    hasBlock(r.Message.Content, "tool_use"),
		Usage: Tokens{
			In: r.Message.Usage.In, Out: r.Message.Usage.Out,
			CacheRead: r.Message.Usage.CacheRead, CacheWrite: r.Message.Usage.CacheMake,
		},
	}
}

func hasBlock(content json.RawMessage, kind string) bool {
	var blocks []block
	if json.Unmarshal(content, &blocks) != nil {
		return false
	}
	for _, b := range blocks {
		if b.Type == kind {
			return true
		}
	}
	return false
}

// turnEnded reports an assistant entry that closed its turn. Claude stops at
// end_turn normally, but also at max_tokens, stop_sequence, refusal and
// pause_turn; only tool_use means the turn continues, and a null stop_reason
// is a mid-response line (issue #63).
func turnEnded(e Entry) bool {
	return e.Type == "assistant" && e.StopReason != "" && e.StopReason != "tool_use"
}

// DeriveKind returns the status event implied by the most recent relevant
// entry, and its timestamp, for the tailer to feed through Apply alongside
// hook events. It cannot produce Waiting - only a permission hook can tell a
// running tool from one blocked on you. ok is false when the tail says
// nothing (only noise so far).
func DeriveKind(entries []Entry) (Kind, time.Time, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		switch {
		case e.Type == "user" && (e.UserIsPrompt || e.ToolResult):
			return PromptSubmitted, e.At, true
		case e.Type == "assistant" && e.ToolUse:
			return ToolStarted, e.At, true
		case turnEnded(e):
			return TurnEnded, e.At, true
		}
	}
	return 0, time.Time{}, false
}
