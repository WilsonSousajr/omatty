package termwrap_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// dingbatTitle is what claude sends: an OSC 0 whose text starts with ✳️
// (U+2733, bytes E2 9C B3) - and 0x9C is the byte x/ansi's parser takes for
// the 8-bit string terminator even inside UTF-8 (#192).
const dingbatTitle = "\x1b]0;\xe2\x9c\xb3\xef\xb8\x8f Claude Code\x07"

// readAll drains r through the guard in chunks of n bytes.
func readAll(t *testing.T, r io.Reader, n int) string {
	t.Helper()
	var out bytes.Buffer
	buf := make([]byte, n)
	for {
		k, err := r.Read(buf)
		out.Write(buf[:k])
		if err == io.EOF {
			return out.String()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestC1Guard_ProtectsSTInsideAnOSCPayload_issue192(t *testing.T) {
	got := readAll(t, termwrap.GuardC1(strings.NewReader(dingbatTitle+"BODY")), 4096)

	if strings.Contains(got[:len(dingbatTitle)], "\x9c") {
		t.Errorf("0x9C survived inside the OSC payload: %q", got)
	}
	if !strings.HasSuffix(got, "\x07BODY") {
		t.Errorf("the terminator and the text after it were disturbed: %q", got)
	}
	if !strings.HasPrefix(got, "\x1b]0;") {
		t.Errorf("the OSC introducer was disturbed: %q", got)
	}
}

func TestC1Guard_ProtectsADCSPayload_issue192(t *testing.T) {
	in := "\x1bPq\xe2\x9c\xb3\x1b\\BODY"
	got := readAll(t, termwrap.GuardC1(strings.NewReader(in)), 4096)

	if strings.Contains(got, "\x9c") {
		t.Errorf("0x9C survived inside the DCS payload: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b\\BODY") {
		t.Errorf("the ST and the text after it were disturbed: %q", got)
	}
}

// Outside a string payload 0x9C is a legitimate continuation byte: ✳ drawn
// as text must arrive byte for byte, and so must an ESC sequence that is not
// a string at all.
func TestC1Guard_LeavesGroundBytesAlone_issue192(t *testing.T) {
	in := "\xe2\x9c\xb3 text \x1b[31m\xe2\x9c\x85\x1b[0m \x1b]0;t\x07\xe2\x9c\xa8"
	got := readAll(t, termwrap.GuardC1(strings.NewReader(in)), 4096)

	if got != in {
		t.Errorf("ground bytes changed:\n got %q\nwant %q", got, in)
	}
}

// A title can straddle two reads: the guard's state must carry across Read
// calls, so a one-byte reader gives the same bytes as a one-shot one.
func TestC1Guard_SurvivesEveryChunkBoundary_issue192(t *testing.T) {
	in := "x" + dingbatTitle + "BODY" + "\x1bPq\xe2\x9c\xb3\x1b\\tail"
	want := readAll(t, termwrap.GuardC1(strings.NewReader(in)), 4096)
	for n := 1; n < 8; n++ {
		if got := readAll(t, termwrap.GuardC1(strings.NewReader(in)), n); got != want {
			t.Errorf("chunks of %d bytes: %q, want %q", n, got, want)
		}
	}
}

// CAN and SUB abort a string the way the parser's table says; the byte after
// them is ground again.
func TestC1Guard_CANAndSUBEndAPayload_issue192(t *testing.T) {
	in := "\x1b]0;a\x18\xe2\x9c\xb3\x1b]0;b\x1a\xe2\x9c\xb3"
	got := readAll(t, termwrap.GuardC1(strings.NewReader(in)), 4096)

	if got != in {
		t.Errorf("bytes after CAN/SUB were treated as payload:\n got %q\nwant %q", got, in)
	}
}
