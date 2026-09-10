// The slice of an agent profile the tailer and the listener need. Declared
// here rather than imported from internal/agent, so watcher stays the
// neutral vocabulary and the dependency runs one way: agent knows what
// claude's JSONL looks like, watcher knows what a Kind is, and only the first
// imports the second (#46).

package watcher

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// Adapter turns one agent's transcript lines and hook payloads into events.
//
//	tl := watcher.Tail(id, path, events, time.Now, time.Second, watcher.ClaudeAdapter())
type Adapter interface {
	// ParseEntry parses one transcript line. ok is false for a line status
	// does not need, including malformed JSON.
	ParseEntry(line []byte) (Entry, bool)
	// DeriveKind is the event implied by the most recent relevant entry in a
	// tail.
	DeriveKind(entries []Entry) (Kind, time.Time, bool)
	// KindOf maps a hook payload to the event it represents.
	KindOf(p hooks.Payload) (Kind, bool)
}

// TranscriptPathFunc is where an agent writes a session's transcript. A
// function type rather than the Profile itself, for the reason Adapter is
// declared here.
type TranscriptPathFunc func(home, dir, sessionID string) string

// WatchDeps is what Start needs beyond the session list.
//
//	w := watcher.Start(watcher.WatchDeps{Home: home, Clock: time.Now,
//	        Adapter: profile.Status, TranscriptPath: profile.TranscriptPath}, st.Sessions)
type WatchDeps struct {
	Home           string
	Clock          func() time.Time
	Adapter        Adapter
	TranscriptPath TranscriptPathFunc
}

// claudeStatus reads claude's transcript through the functions this package
// already exports. The parsing stays here for now: that is where its tests
// and fixtures live, and discover imports PromptText from the same file.
// What the type buys is the call-site indirection - the tailer and the
// listener no longer call those functions directly, so a second agent
// supplies its own without touching either (#46, #61, #62, #122).
type claudeStatus struct{}

func (claudeStatus) ParseEntry(line []byte) (Entry, bool)               { return ParseEntry(line) }
func (claudeStatus) DeriveKind(entries []Entry) (Kind, time.Time, bool) { return DeriveKind(entries) }
func (claudeStatus) KindOf(p hooks.Payload) (Kind, bool)                { return KindOf(p) }

// ClaudeAdapter is the Adapter for claude's own transcript and hook shapes.
func ClaudeAdapter() Adapter { return claudeStatus{} }
