package status_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// onceTranscript is a transcript read whole, the way a one-shot command sees
// it: every line at once, then nothing.
type onceTranscript struct{ lines [][]byte }

func (o *onceTranscript) Poll() ([][]byte, bool, bool) {
	lines := o.lines
	o.lines = nil
	return lines, false, true
}

// ADR 0001's transcript Read (migration step 7.1, #653): one read of the whole
// transcript says what the tailer's first poll would - the status its last
// turn implies, and the usage it adds up to - with no timer and no socket.
func TestRead_isWhatTheTailersFirstPollSays_issue653(t *testing.T) {
	src := &onceTranscript{lines: [][]byte{
		[]byte(`{"type":"assistant","timestamp":"2026-09-02T12:00:00Z","message":{"id":"m1","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":10,"output_tokens":5}}}`),
		[]byte(`{"type":"user","timestamp":"2026-09-02T12:00:05Z","message":{"role":"user","content":"next"}}`),
	}}

	st := status.Read("s1", src, status.ClaudeAdapter(), func() time.Time { return time.Unix(0, 0) })

	if st.Status != status.StatusThinking {
		t.Errorf("Status = %q, want thinking: the last line is a prompt not yet answered", st.Status)
	}
	if st.Tokens.In != 10 || st.Tokens.Out != 5 {
		t.Errorf("Tokens = %+v, want 10 in and 5 out", st.Tokens)
	}
}

// A session that has not spoken has no transcript yet; it reads as nothing
// rather than as an error.
func TestRead_aSessionThatHasNotSpokenIsZero_issue653(t *testing.T) {
	st := status.Read("s1", &onceTranscript{}, status.ClaudeAdapter(), time.Now)
	if st.Status != "" || !st.At.IsZero() {
		t.Errorf("Read of an empty transcript = %+v, want the zero state", st)
	}
}
