package termwrap_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// osc52Hello is what a child sends to put "hello" on the clipboard: the
// sequence named in issue #212.
const osc52Hello = "\x1b]52;c;aGVsbG8=\x07"

// lift drains in through the lifting reader in chunks of n bytes, returning
// what reached the emulator and every write pulled out of the stream.
//
// The reader does its scanning inside Read, on the caller's goroutine, so by
// the time readAll returns every write is already queued and len() on the
// channel is exact.
func lift(t *testing.T, in string, n int) (string, []termwrap.ClipboardWrite) {
	t.Helper()
	r, writes := termwrap.LiftClipboard(strings.NewReader(in))
	got := readAll(t, r, n)
	lifted := make([]termwrap.ClipboardWrite, 0, len(writes))
	for len(writes) > 0 {
		lifted = append(lifted, <-writes)
	}
	return got, lifted
}

// The issue's own case: the sequence reaches the host and nothing lands on
// the grid.
func TestClipLift_osc52ReachesTheHostAndNotTheGrid_issue212(t *testing.T) {
	got, lifted := lift(t, "before"+osc52Hello+"after", 4096)

	if got != "beforeafter" {
		t.Errorf("the sequence was left in the stream: %q", got)
	}
	if len(lifted) != 1 {
		t.Fatalf("lifted %d writes, want 1: %+v", len(lifted), lifted)
	}
	if lifted[0].Selection != 'c' || lifted[0].Text != "hello" {
		t.Errorf("lifted %+v, want {c hello}", lifted[0])
	}
}

// A copy can straddle two of the emulator's reads, so the state must carry
// across Read calls: every chunk size must give the same answer.
func TestClipLift_survivesEveryChunkBoundary_issue212(t *testing.T) {
	in := "a" + osc52Hello + "b\x1b]0;t\x07c" + "\x1b]52;p;d29ybGQ=\x1b\\d"
	wantOut, wantLifted := lift(t, in, 4096)
	for n := 1; n < 9; n++ {
		got, lifted := lift(t, in, n)
		if got != wantOut {
			t.Errorf("chunks of %d bytes: %q, want %q", n, got, wantOut)
		}
		if len(lifted) != len(wantLifted) {
			t.Errorf("chunks of %d bytes: lifted %+v, want %+v", n, lifted, wantLifted)
		}
	}
}

// Every other sequence is somebody else's: a title, a hyperlink, a colour
// query, a DCS and plain text all arrive byte for byte. This is the guard
// against a filter that eats more than it was asked to.
func TestClipLift_leavesOtherSequencesAlone_issue212(t *testing.T) {
	in := "\x1b]0;title\x07 text \x1b[31mred\x1b[0m \x1b]8;;http://x\x07link" +
		"\x1b]11;?\x07\x1bPq~~\x1b\\ tail \x1b]520;notfiftytwo\x07"
	got, lifted := lift(t, in, 4096)

	if got != in {
		t.Errorf("bytes were disturbed:\n got %q\nwant %q", got, in)
	}
	if len(lifted) != 0 {
		t.Errorf("lifted %+v from a stream with no OSC 52 in it", lifted)
	}
}

// The selection field says which clipboard. 'p' is the primary selection;
// naming none means the system clipboard.
func TestClipLift_readsTheSelectionField_issue212(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want byte
	}{
		{"system", "\x1b]52;c;aGVsbG8=\x07", 'c'},
		{"primary", "\x1b]52;p;aGVsbG8=\x07", 'p'},
		{"unnamed", "\x1b]52;;aGVsbG8=\x07", 'c'},
		{"a set takes the first", "\x1b]52;pc;aGVsbG8=\x07", 'p'},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, lifted := lift(t, tc.in, 4096)
			if got != "" {
				t.Errorf("the sequence was left in the stream: %q", got)
			}
			if len(lifted) != 1 {
				t.Fatalf("lifted %d writes, want 1", len(lifted))
			}
			if lifted[0].Selection != tc.want {
				t.Errorf("selection %q, want %q", lifted[0].Selection, tc.want)
			}
		})
	}
}

// Three terminators end an OSC: BEL, the two-byte ST, and the 8-bit 0x9C.
// The last one is why this reader sits before c1Guard, which would have
// rewritten it to '?' (#192).
func TestClipLift_everyTerminatorEndsThePayload_issue212(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"BEL", "\x1b]52;c;aGVsbG8=\x07"},
		{"ST", "\x1b]52;c;aGVsbG8=\x1b\\"},
		{"8-bit ST", "\x1b]52;c;aGVsbG8=\x9c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, lifted := lift(t, tc.in+"tail", 4096)
			if got != "tail" {
				t.Errorf("the sequence was left in the stream: %q", got)
			}
			if len(lifted) != 1 || lifted[0].Text != "hello" {
				t.Fatalf("lifted %+v, want one {c hello}", lifted)
			}
		})
	}
}

// A '?' payload is the read direction - the child asking the host for the
// clipboard. omatty does not answer it (#212 forwards writes only), so it is
// dropped exactly as the emulator dropped it, and nothing is put on the host
// clipboard on the strength of a question.
func TestClipLift_dropsAClipboardQuery_issue212(t *testing.T) {
	got, lifted := lift(t, "\x1b]52;c;?\x07tail", 4096)

	if got != "tail" {
		t.Errorf("the query was left in the stream: %q", got)
	}
	if len(lifted) != 0 {
		t.Errorf("a read request was forwarded as a write: %+v", lifted)
	}
}

// A payload that is not base64 is a broken sequence, not text to paste.
func TestClipLift_dropsAnUndecodablePayload_issue212(t *testing.T) {
	got, lifted := lift(t, "\x1b]52;c;!!!not base64!!!\x07tail", 4096)

	if got != "tail" {
		t.Errorf("the sequence was left in the stream: %q", got)
	}
	if len(lifted) != 0 {
		t.Errorf("undecodable bytes reached the host clipboard: %+v", lifted)
	}
}

// Padding is optional in the wild; both spellings decode.
func TestClipLift_acceptsUnpaddedBase64_issue212(t *testing.T) {
	_, lifted := lift(t, "\x1b]52;c;aGVsbG8\x07", 4096)

	if len(lifted) != 1 || lifted[0].Text != "hello" {
		t.Fatalf("lifted %+v, want one {c hello}", lifted)
	}
}

// A child that opens a write and never terminates it must not make the
// reader buffer without limit: past the cap the write is dropped and the
// stream carries on.
func TestClipLift_dropsAnOversizedPayload_issue212(t *testing.T) {
	huge := strings.Repeat("QUJD", termwrap.MaxClipPayload/4+16)
	got, lifted := lift(t, "\x1b]52;c;"+huge+"\x07tail", 4096)

	if got != "tail" {
		t.Errorf("an oversized payload reached the emulator: %d bytes", len(got))
	}
	if len(lifted) != 0 {
		t.Errorf("an oversized payload reached the host clipboard: %d writes", len(lifted))
	}
}

// A stream that ends before the introducer is complete has not made an OSC
// 52 of anything, so the bytes held back on the chance that it would are
// given up at EOF rather than swallowed.
func TestClipLift_releasesAnIncompleteIntroducer_issue212(t *testing.T) {
	for _, in := range []string{"text\x1b", "text\x1b]", "text\x1b]5", "text\x1b]52"} {
		got, lifted := lift(t, in, 4096)
		if got != in {
			t.Errorf("held bytes were swallowed at EOF:\n got %q\nwant %q", got, in)
		}
		if len(lifted) != 0 {
			t.Errorf("%q lifted %+v", in, lifted)
		}
	}
}

// Past the introducer the sequence is a clipboard write, and one the child
// never terminated carries no payload. It is dropped rather than painted -
// which is what the emulator did with every OSC 52 before this reader
// existed, so nothing on the grid changes.
func TestClipLift_dropsAnUnterminatedWrite_issue212(t *testing.T) {
	got, lifted := lift(t, "text\x1b]52;c;aGVsbG8=", 4096)

	if got != "text" {
		t.Errorf("an unterminated write reached the emulator: %q", got)
	}
	if len(lifted) != 0 {
		t.Errorf("an unterminated write reached the host clipboard: %+v", lifted)
	}
}

// The PTY read loop is behind this reader. If a full queue blocked it, the
// pane would freeze (issue #33), so writes past the queue are dropped and
// the stream keeps moving.
func TestClipLift_neverBlocksOnAFullQueue_issue212(t *testing.T) {
	in := strings.Repeat(osc52Hello, 64) + "tail"
	done := make(chan string, 1)
	go func() {
		got, _ := lift(t, in, 4096)
		done <- got
	}()
	select {
	case got := <-done:
		if got != "tail" {
			t.Errorf("stream came through as %q, want %q", got, "tail")
		}
	case <-t.Context().Done():
		t.Fatal("the reader blocked on a full clipboard queue")
	}
}
