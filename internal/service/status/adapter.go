// The slice of an agent profile the tailer and the listener need. Declared
// here rather than imported from internal/agent, so watcher stays the
// neutral vocabulary and the dependency runs one way: agent knows what
// claude's JSONL looks like, watcher knows what a Kind is, and only the first
// imports the second (#46).

package status

import (
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"io"
)

// TranscriptPathFunc is where an agent writes a session's transcript. A
// function type rather than the Profile itself, for the reason Adapter is
// declared in internal/domain/status rather than in internal/agent.
type TranscriptPathFunc func(home, dir, sessionID string) string

// WatchDeps is what Start needs beyond the session list.
//
//	w := status.Start(status.WatchDeps{Home: home, Clock: time.Now,
//	        Adapter: profile.Status, TranscriptPath: profile.TranscriptPath}, st.Sessions)
type WatchDeps struct {
	Home           string
	Clock          func() time.Time
	Adapter        Adapter
	TranscriptPath TranscriptPathFunc
	// OpenTranscript reads a transcript at a path. It is injected, because
	// reading files is infra's business (ADR 0001, step 5.2c, #653): cmd passes
	// internal/infra/transcript's NewReader.
	OpenTranscript func(path string) Transcript
	// ListenHooks serves the hook socket at path, offering each payload to
	// sink without waiting. It is injected, because running a socket server is
	// infra's business (ADR 0001, step 5.2d, #653): cmd passes
	// internal/infra/hookserver's Listen.
	ListenHooks func(path string, sink chan<- dstatus.HookPayload) (io.Closer, error)
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
func (claudeStatus) KindOf(p dstatus.HookPayload) (Kind, bool)          { return KindOf(p) }

// ClaudeAdapter is the Adapter for claude's own transcript and hook shapes.
func ClaudeAdapter() Adapter { return claudeStatus{} }
