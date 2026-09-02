package watcher

import (
	"encoding/json"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// Entry is the slice of a transcript line that status needs. Every other line
// type - attachments, queue operations, titles, snapshots - is dropped at
// parse, so callers only ever hold user and assistant turns.
type Entry struct {
	Type         string // "user" or "assistant"
	At           time.Time
	StopReason   string // assistant
	UserIsPrompt bool   // user: content was a string (a typed prompt)
	ToolUse      bool   // assistant: a tool_use block is present
	ToolResult   bool   // user: a tool_result block is present
	Usage        Tokens // assistant
}

// rawEntry is the on-disk shape. content is deferred so it can be a string
// (a prompt) or a list of blocks (tool results / assistant blocks).
type rawEntry struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
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

func parseUser(r rawEntry) Entry {
	e := Entry{Type: "user", At: r.Timestamp}
	// A string content is a typed prompt; a list holds tool results.
	var s string
	if json.Unmarshal(r.Message.Content, &s) == nil {
		e.UserIsPrompt = true
		return e
	}
	e.ToolResult = hasBlock(r.Message.Content, "tool_result")
	return e
}

func parseAssistant(r rawEntry) Entry {
	return Entry{
		Type:       "assistant",
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

// DeriveFromTail returns the status implied by the most recent relevant entry,
// and its timestamp. It cannot produce Waiting - only a permission hook can
// tell a running tool from one blocked on you.
func DeriveFromTail(entries []Entry) (registry.Status, time.Time) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		switch {
		case e.Type == "user" && e.UserIsPrompt, e.Type == "user" && e.ToolResult:
			return registry.StatusThinking, e.At
		case e.Type == "assistant" && e.ToolUse:
			return registry.StatusTool, e.At
		case e.Type == "assistant" && e.StopReason == "end_turn":
			return registry.StatusDone, e.At
		}
	}
	return registry.StatusIdle, time.Time{}
}

// SumUsage totals the token counters across all assistant entries.
func SumUsage(entries []Entry) Tokens {
	var t Tokens
	for _, e := range entries {
		t.In += e.Usage.In
		t.Out += e.Usage.Out
		t.CacheRead += e.Usage.CacheRead
		t.CacheWrite += e.Usage.CacheWrite
	}
	return t
}
