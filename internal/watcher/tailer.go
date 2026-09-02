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
	stop        chan struct{}
	once        sync.Once
}

// Tail starts polling path every `every` and returns the Tailer. Close stops
// it. clock is injected so a test can prove the event carries the entry's own
// timestamp, not now.
//
//	tl := watcher.Tail(sess.ID, paths.Transcript(home, sess.Dir, sess.ID), events, time.Now, time.Second)
//	defer tl.Close()
func Tail(sessionID, path string, sink chan<- Event, clock func() time.Time, every time.Duration) *Tailer {
	tl := &Tailer{sessionID: sessionID, path: path, sink: sink, clock: clock, stop: make(chan struct{})}
	go tl.loop(every)
	return tl
}

// Close stops the polling goroutine. It is idempotent.
func (tl *Tailer) Close() { tl.once.Do(func() { close(tl.stop) }) }

func (tl *Tailer) loop(every time.Duration) {
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
	fresh, err := readFrom(f, tl.offset)
	if err != nil || len(fresh) == 0 {
		return
	}
	tl.offset += int64(len(fresh))
	tl.consume(fresh)
	tl.emit()
}

// reconcileTruncation resets the read offset when the file shrank, which
// happens on a /clear or a rewrite.
func (tl *Tailer) reconcileTruncation(f *os.File) {
	if info, err := f.Stat(); err == nil && info.Size() < tl.offset {
		tl.offset, tl.usage, tl.ring, tl.partial = 0, Tokens{}, nil, nil
		tl.lastUsageID = ""
	}
}

// consume parses complete lines out of the fresh bytes, carrying any trailing
// partial line to the next poll so a line split across reads is not lost.
func (tl *Tailer) consume(fresh []byte) {
	buf := append(tl.partial, fresh...)
	for {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			break
		}
		tl.ingest(buf[:i])
		buf = buf[i+1:]
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
	}
	tl.ring = append(tl.ring, e)
	if len(tl.ring) > ringSize {
		tl.ring = tl.ring[len(tl.ring)-ringSize:]
	}
}

// emit sends the derived status (with the entry's own timestamp) and the
// running usage total.
func (tl *Tailer) emit() {
	kind, at, ok := DeriveKind(tl.ring)
	if ok {
		tl.sink <- Event{SessionID: tl.sessionID, Kind: kind, At: at}
	}
	tl.sink <- Event{SessionID: tl.sessionID, Kind: UsageUpdated, At: tl.clock(), Tokens: tl.usage}
}

func readFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}
