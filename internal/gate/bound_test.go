package gate_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// go test on a broken build writes unbounded output, and the whole of it would
// reach the sidebar, the gate pane and eventually claude's context. Keep the
// tail: Go puts the FAIL lines that say what went wrong at the end.
func TestRun_floodOfOutput_keepsTheTailAndSaysWhatItDropped(t *testing.T) {
	steps := []gate.Step{{Name: "test", Run: "seq 1 50000; echo FAIL; exit 1"}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	out := got[0].Output
	if lines := strings.Count(out, "\n"); lines > gate.MaxOutputLines+2 {
		t.Errorf("kept %d lines, want at most %d plus the elision notice", lines, gate.MaxOutputLines)
	}
	if !strings.Contains(out, "FAIL") {
		t.Error("output lost the trailing FAIL line; the tail is the part that says why")
	}
	if !strings.Contains(out, "elided") {
		t.Error("output was truncated without saying so")
	}
	if strings.Contains(out, "\n1\n") {
		t.Error("output kept the head; it should keep the tail")
	}
}

func TestRun_outputUnderTheCap_isKeptWhole(t *testing.T) {
	steps := []gate.Step{{Name: "test", Run: "printf 'one\ntwo\nthree\n'"}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	if want := "one\ntwo\nthree\n"; got[0].Output != want {
		t.Errorf("Output = %q, want %q untouched", got[0].Output, want)
	}
	if strings.Contains(got[0].Output, "elided") {
		t.Error("short output was marked as truncated")
	}
}

// A single line longer than the byte cap must not slip through the line cap.
func TestRun_oneEnormousLine_isStillBounded(t *testing.T) {
	steps := []gate.Step{{Name: "test", Run: fmt.Sprintf("head -c %d /dev/zero | tr '\\0' 'x'", gate.MaxOutputBytes*3)}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	if len(got[0].Output) > gate.MaxOutputBytes*2 {
		t.Errorf("Output is %d bytes, want it bounded near %d", len(got[0].Output), gate.MaxOutputBytes)
	}
}
