// Package transcript reads an agent's JSONL transcript as it grows: the
// complete lines appended since the last read, and whether the file was cut
// short in between. It knows nothing of what the lines mean; that is
// internal/service/status's business (ADR 0001, migration step 5.2c, #653).
//
//	r := transcript.NewReader(path)
//	lines, truncated, ok := r.Poll()
package transcript

import (
	"bytes"
	"io"
	"os"
)

// maxPollBytes caps one read, so a large delta costs a bounded buffer rather
// than an allocation its own size (issue #64).
const maxPollBytes = 1 << 20

// MaxLineBytes is the longest line kept. A longer one is dropped whole, and
// reading carries on after it (issue #64).
const MaxLineBytes = 1 << 20

// Reader follows one transcript file across polls.
type Reader struct {
	path      string
	offset    int64  // bytes already consumed
	partial   []byte // a trailing line not yet terminated by \n
	skipping  bool   // inside a line over MaxLineBytes; discard to the next newline
	truncated bool   // the file shrank since the last batch was returned
}

// NewReader returns a Reader for path. The file need not exist yet.
//
//	r := transcript.NewReader(paths.Transcript(home, dir, id))
func NewReader(path string) *Reader { return &Reader{path: path} }

// Poll returns the complete lines appended since the last call. truncated
// says the file shrank since the last batch - reported on the next batch
// that reads anything, so the reader's user resets before it reads anew. ok
// is false when nothing was read; a missing file is not an error, the
// session has simply not spoken yet.
func (r *Reader) Poll() (lines [][]byte, truncated, ok bool) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, false, false
	}
	defer func() { _ = f.Close() }()
	r.reconcileTruncation(f)
	lines, ok = r.drain(f)
	if !ok {
		return nil, false, false
	}
	truncated, r.truncated = r.truncated, false
	return lines, truncated, true
}

// drain reads everything appended since the last poll in chunks of at most
// maxPollBytes. It reports whether anything was read.
func (r *Reader) drain(f *os.File) ([][]byte, bool) {
	var lines [][]byte
	read := false
	for {
		fresh, err := readFrom(f, r.offset)
		if err != nil || len(fresh) == 0 {
			return lines, read
		}
		read = true
		r.offset += int64(len(fresh))
		lines = append(lines, r.consume(fresh)...)
		if len(fresh) < maxPollBytes {
			return lines, true
		}
	}
}

// reconcileTruncation starts again from the top when the file shrank, which
// happens on a /clear or a rewrite.
func (r *Reader) reconcileTruncation(f *os.File) {
	if info, err := f.Stat(); err == nil && info.Size() < r.offset {
		r.offset, r.partial, r.skipping, r.truncated = 0, nil, false, true
	}
}

// consume splits complete lines out of the fresh bytes, carrying any trailing
// partial line to the next poll so a line split across reads is not lost. A
// line over MaxLineBytes, complete or not, is dropped (issue #64).
func (r *Reader) consume(fresh []byte) [][]byte {
	var lines [][]byte
	buf := append(r.partial, fresh...)
	for {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			break
		}
		if !r.skipping && i <= MaxLineBytes {
			lines = append(lines, buf[:i])
		}
		r.skipping = false
		buf = buf[i+1:]
	}
	if len(buf) > MaxLineBytes {
		r.skipping, buf = true, nil
	}
	r.partial = append([]byte(nil), buf...)
	return lines
}

func readFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, maxPollBytes))
}
