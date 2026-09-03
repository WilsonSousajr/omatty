package watcher

import (
	"bytes"
	"io"
	"os"
	"sync"
	"time"
)

// ringSize bounds how many recent entries a tailer keeps to derive status.
// Only the tail matters, so there is no reason to grow without limit.
const ringSize = 32

// maxPollBytes bounds one read, so a large delta is consumed in chunks
// rather than allocated at once (issue #64).
const maxPollBytes = 1 << 20

// maxLineBytes bounds a single JSONL line. A longer one - a tool returning a
// huge file - is discarded whole rather than buffered without limit.
const maxLineBytes = 1 << 20

// Tailer polls one session's transcript and emits status and usage events as
// the file grows. It is the source of truth on attach (omatty may start after
// a session is mid-turn), the self-heal when a hook was missed, and the only
// source of age and tokens.
type Tailer struct {
	sessionID string
	path      string
	sink      chan<- Event
	clock     func() time.Time

	offset      int64   // bytes already consumed
	ring        []Entry // last ringSize relevant entries
	usage       Tokens  // cumulative across the whole file
	lastUsageID string  // the response whose usage was last counted (issue #59)
	partial     []byte  // a trailing line not yet terminated by \n
	skipping    bool    // inside a line over maxLineBytes; discard to the next newline
	last        Event   // the status event most recently sent, to skip repeats (issue #66)
	usageDirty  bool    // usage changed since it was last sent
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
}

// Tail starts polling path every `every` and returns the Tailer. Close stops
// it. clock is injected so a test can prove the event carries the entry's own
// timestamp, not now.
//
//	tl := watcher.Tail(sess.ID, paths.Transcript(home, sess.Dir, sess.ID), events, time.Now, time.Second)
//	defer tl.Close()
func Tail(sessionID, path string, sink chan<- Event, clock func() time.Time, every time.Duration) *Tailer {
	tl := &Tailer{sessionID: sessionID, path: path, sink: sink, clock: clock,
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
// directly rather than waiting on a ticker. A missing file is not an error -
// the session has simply not spoken yet.
func (tl *Tailer) Poll() {
	f, err := os.Open(tl.path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	tl.reconcileTruncation(f)
	if tl.drain(f) {
		tl.emit()
	}
}

// drain reads everything appended since the last poll in chunks of at most
// maxPollBytes, so a large delta costs a bounded buffer rather than an
// allocation its own size (issue #64). It reports whether anything was read.
func (tl *Tailer) drain(f *os.File) bool {
	read := false
	for {
		fresh, err := readFrom(f, tl.offset)
		if err != nil || len(fresh) == 0 {
			return read
		}
		read = true
		tl.offset += int64(len(fresh))
		tl.consume(fresh)
		if len(fresh) < maxPollBytes {
			return true
		}
	}
}

// reconcileTruncation resets the read offset when the file shrank, which
// happens on a /clear or a rewrite. The zeroed usage must reach the sidebar,
// so it is marked dirty.
func (tl *Tailer) reconcileTruncation(f *os.File) {
	if info, err := f.Stat(); err == nil && info.Size() < tl.offset {
		tl.offset, tl.usage, tl.ring, tl.partial = 0, Tokens{}, nil, nil
		tl.lastUsageID, tl.skipping = "", false
		tl.last, tl.usageDirty = Event{}, true
	}
}

// consume parses complete lines out of the fresh bytes, carrying any trailing
// partial line to the next poll so a line split across reads is not lost. A
// line over maxLineBytes, complete or not, is dropped (issue #64).
func (tl *Tailer) consume(fresh []byte) {
	buf := append(tl.partial, fresh...)
	for {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			break
		}
		if !tl.skipping && i <= maxLineBytes {
			tl.ingest(buf[:i])
		}
		tl.skipping = false
		buf = buf[i+1:]
	}
	if len(buf) > maxLineBytes {
		tl.skipping, buf = true, nil
	}
	tl.partial = append([]byte(nil), buf...)
}

func (tl *Tailer) ingest(line []byte) {
	e, ok := ParseEntry(line)
	if !ok {
		return
	}
	// One API response is written as one line per content block, each
	// repeating the same usage under the same message id; count it once. A
	// line without an id (older transcripts, fixtures) still counts (issue #59).
	if e.Type == "assistant" && (e.MessageID == "" || e.MessageID != tl.lastUsageID) {
		tl.usage.add(e.Usage)
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
	kind, at, ok := DeriveKind(tl.ring)
	if ok && (kind != tl.last.Kind || !at.Equal(tl.last.At)) {
		tl.last = Event{Kind: kind, At: at}
		tl.send(Event{SessionID: tl.sessionID, Kind: kind, At: at})
	}
	if tl.usageDirty {
		tl.usageDirty = false
		tl.send(Event{SessionID: tl.sessionID, Kind: UsageUpdated, At: tl.clock(), Tokens: tl.usage})
	}
}

// send delivers ev unless the tailer is closed, so Close never leaves a
// goroutine parked on a full sink (issue #65).
func (tl *Tailer) send(ev Event) {
	select {
	case tl.sink <- ev:
	case <-tl.stop:
	}
}

func readFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, maxPollBytes))
}
