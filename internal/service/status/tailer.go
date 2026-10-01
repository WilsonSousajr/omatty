package status

import (
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"sync"
	"time"
)

// ringSize bounds how many recent entries a tailer keeps to derive status.
// Only the tail matters, so there is no reason to grow without limit.
const ringSize = 32

// Transcript is one transcript file, read incrementally: ADR 0001's
// Transcripts port, declared here by its consumer. Poll returns the complete
// lines appended since the last call, whether the file was cut short since the
// last batch, and whether anything was read at all. internal/infra/transcript
// implements it; reading bytes - offsets, split lines, the line cap - is its
// business, and what the lines mean is this package's (step 5.2c, #653).
//
//	tl := status.Tail(sess.ID, transcript.NewReader(path), events, time.Now, time.Second, adapter)
type Transcript interface {
	Poll() (lines [][]byte, truncated, ok bool)
}

// Tailer polls one session's transcript and emits status and usage events as
// the file grows. It is the source of truth on attach (omatty may start after
// a session is mid-turn), the self-heal when a hook was missed, and the only
// source of age and tokens.
type Tailer struct {
	sessionID string
	src       Transcript
	sink      chan<- dstatus.Event
	clock     func() time.Time
	adapter   dstatus.Adapter // the agent's parser (#46)

	ring        []dstatus.Entry // last ringSize relevant entries
	usage       dstatus.Tokens  // cumulative across the whole file
	lastUsageID string          // the response whose usage was last counted (issue #59)
	last        dstatus.Event   // the status event most recently sent, to skip repeats (issue #66)
	usageDirty  bool            // usage changed since it was last sent
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
}

// Tail starts polling src every `every` and returns the Tailer. Close stops
// it. clock is injected so a test can prove the event carries the entry's own
// timestamp, not now.
//
//	tl := status.Tail(sess.ID, transcript.NewReader(path), events, time.Now, time.Second, status.ClaudeAdapter())
//	defer tl.Close()
//
// adapter is the agent's own parser: which lines matter and what they mean is
// the agent's business, not the tailer's (#46).
func Tail(
	sessionID string, src Transcript, sink chan<- dstatus.Event, clock func() time.Time, every time.Duration, adapter dstatus.Adapter,
) *Tailer {
	tl := &Tailer{sessionID: sessionID, src: src, sink: sink, clock: clock, adapter: adapter,
		stop: make(chan struct{}), done: make(chan struct{})}
	go tl.loop(every)
	return tl
}

// Close stops the polling goroutine. It is idempotent.
func (tl *Tailer) Close() { tl.once.Do(func() { close(tl.stop) }) }

// Done is closed once the polling goroutine has exited, so a caller can prove
// Close actually stopped it (issue #65).
func (tl *Tailer) Done() <-chan struct{} { return tl.done }

func (tl *Tailer) loop(every time.Duration) {
	defer close(tl.done)
	defer recoverLoop("tailer", tl.sessionID)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-tl.stop:
			return
		case <-t.C:
			tl.Poll()
		}
	}
}

// Poll reads whatever has been appended since the last call and emits at most
// one status event and one usage event. It is exported so tests drive it
// directly rather than waiting on a ticker. A missing file reads nothing - the
// session has simply not spoken yet.
func (tl *Tailer) Poll() {
	lines, truncated, ok := tl.src.Poll()
	if !ok {
		return
	}
	if truncated {
		tl.startOver()
	}
	for _, line := range lines {
		tl.ingest(line)
	}
	tl.emit()
}

// startOver forgets everything read before a truncation - a /clear or a
// rewrite - and marks the zeroed usage dirty, so it reaches the sidebar.
func (tl *Tailer) startOver() {
	tl.usage, tl.ring, tl.lastUsageID = dstatus.Tokens{}, nil, ""
	tl.last, tl.usageDirty = dstatus.Event{}, true
}

func (tl *Tailer) ingest(line []byte) {
	e, ok := tl.adapter.ParseEntry(line)
	if !ok {
		return
	}
	// One API response is written as one line per content block, each
	// repeating the same usage under the same message id; count it once. A
	// line without an id (older transcripts, fixtures) still counts (issue #59).
	if e.Type == "assistant" && (e.MessageID == "" || e.MessageID != tl.lastUsageID) {
		tl.usage.Add(e.Usage)
		tl.lastUsageID = e.MessageID
		tl.usageDirty = true
	}
	tl.ring = append(tl.ring, e)
	if len(tl.ring) > ringSize {
		tl.ring = tl.ring[len(tl.ring)-ringSize:]
	}
}

// emit sends the derived status if it changed and the usage total if it
// changed. Any append used to re-send both (issue #66).
func (tl *Tailer) emit() {
	kind, at, ok := tl.adapter.DeriveKind(tl.ring)
	if ok && (kind != tl.last.Kind || !at.Equal(tl.last.At)) {
		tl.last = dstatus.Event{Kind: kind, At: at}
		tl.send(dstatus.Event{SessionID: tl.sessionID, Kind: kind, At: at})
	}
	if tl.usageDirty {
		tl.usageDirty = false
		tl.send(dstatus.Event{SessionID: tl.sessionID, Kind: dstatus.UsageUpdated, At: tl.clock(), Tokens: tl.usage})
	}
}

// send delivers ev unless the tailer is closed, so Close never leaves a
// goroutine parked on a full sink (issue #65).
func (tl *Tailer) send(ev dstatus.Event) {
	select {
	case tl.sink <- ev:
	case <-tl.stop:
	}
}
