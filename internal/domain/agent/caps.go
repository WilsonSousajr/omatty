// Capabilities: what an agent actually offers, and the tier omatty derives
// from them (#520). The tier is computed, never declared, so the support
// matrix can be generated from the profiles rather than written beside them
// and drift from them.

package agent

// Identity is how omatty learns which conversation a session is on, and so
// which transcript is its. The zero value is NoIdentity, the safe default: a
// profile that says nothing claims nothing.
type Identity int

const (
	// NoIdentity means omatty cannot tell which conversation is the session's.
	NoIdentity Identity = iota
	// Assigned means omatty hands the agent the id, as claude's --session-id does.
	Assigned
	// Reported means a startup hook reports the agent's own id (#523).
	Reported
	// Scanned means the id is found in the agent's store by directory and start time (#523).
	Scanned
)

// StatusSource is where a session's status comes from, ordered from least to
// most informative so "at least a transcript" is a comparison.
type StatusSource int

const (
	// StatusProcess means only whether the process runs.
	StatusProcess StatusSource = iota
	// StatusTranscript means the agent's transcript, tailed (invariant 2).
	StatusTranscript
	// StatusHooks means hooks for latency, plus the transcript for truth.
	StatusHooks
)

// Caps is what an agent offers omatty. Every field is a fact about the agent
// binary, found by its spike, never a wish.
//
//	caps := agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks, Waiting: true, Resume: true, TurnBoundary: true}
type Caps struct {
	Identity Identity
	Status   StatusSource
	// Waiting: the agent reports that it waits on the operator (PermissionRequested).
	Waiting bool
	// Resume: the agent has a flag that resumes a conversation by id.
	Resume bool
	// TurnBoundary: the agent reports a turn's end, which #233's gate
	// auto-run, #311's since-turn review and notifications key off.
	TurnBoundary bool
}

// Tier is how much omatty can know about an agent's session.
type Tier int

const (
	// Process means running or exited, and nothing else.
	Process Tier = iota
	// Transcript means status from the transcript, without the hooks' extras.
	Transcript
	// Full means everything claude gives omatty.
	Full
)

// Tier derives the tier from the capabilities. A transcript omatty cannot
// attribute to the session is no transcript at all, so an unknown identity is
// Process whatever else the agent offers.
//
//	agent.Caps{}.Tier() // agent.Process
func (c Caps) Tier() Tier {
	if c.Identity == NoIdentity || c.Status < StatusTranscript {
		return Process
	}
	if c.Status == StatusHooks && c.Waiting && c.Resume {
		return Full
	}
	return Transcript
}

// String is the tier's name as the support matrix and the session detail show it.
//
//	agent.Full.String() // "full"
func (t Tier) String() string {
	return [...]string{"process", "transcript", "full"}[t]
}
