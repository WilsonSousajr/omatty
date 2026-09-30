package transcript_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
)

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func lineStrings(lines [][]byte) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = string(l)
	}
	return out
}

// The reader half of the tailer, moved to infra (migration step 5.2c, #653):
// only complete lines, and only the ones appended since the last poll.
func TestReader_returnsOnlyNewCompleteLines_issue653(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	r := transcript.NewReader(path)
	appendTo(t, path, "a\nb\npart")

	lines, truncated, ok := r.Poll()
	if !ok || truncated || strings.Join(lineStrings(lines), ",") != "a,b" {
		t.Fatalf("first Poll = %q, %v, %v; want [a b], false, true", lineStrings(lines), truncated, ok)
	}
	appendTo(t, path, "ial\nc\n")
	lines, _, _ = r.Poll()
	if strings.Join(lineStrings(lines), ",") != "partial,c" {
		t.Errorf("second Poll = %q, want the split line joined, then c", lineStrings(lines))
	}
}

// A missing file has written nothing yet; that is not an error.
func TestReader_aMissingFileReadsNothing_issue653(t *testing.T) {
	r := transcript.NewReader(filepath.Join(t.TempDir(), "never.jsonl"))
	if lines, truncated, ok := r.Poll(); ok || truncated || lines != nil {
		t.Errorf("Poll of a missing file = %v, %v, %v; want nothing", lines, truncated, ok)
	}
}

// A file that shrank (a /clear, a rewrite) is read again from the start, and
// the batch says so - even when the shrink was seen by a poll that read
// nothing, because the interpreter must reset before it reads anew.
func TestReader_aTruncationIsReportedOnTheNextBatch_issue653(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	r := transcript.NewReader(path)
	appendTo(t, path, "one\ntwo\n")
	r.Poll()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := r.Poll(); ok {
		t.Fatal("an emptied file read something")
	}
	appendTo(t, path, "fresh\n")

	lines, truncated, ok := r.Poll()

	if !ok || !truncated || strings.Join(lineStrings(lines), ",") != "fresh" {
		t.Errorf("Poll after truncation = %q, %v, %v; want [fresh], true, true", lineStrings(lines), truncated, ok)
	}
}

// A line over the cap is dropped whole, and reading carries on after it
// (#64): one runaway line must not stall status or cost its size in memory.
func TestReader_aLineOverTheCapIsDroppedAndReadingGoesOn_issue653(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	r := transcript.NewReader(path)
	appendTo(t, path, strings.Repeat("x", transcript.MaxLineBytes+1)+"\nafter\n")

	var got []string
	for {
		lines, _, ok := r.Poll()
		if !ok {
			break
		}
		got = append(got, lineStrings(lines)...)
	}
	if strings.Join(got, ",") != "after" {
		t.Errorf("lines = %d of them, want only \"after\"", len(got))
	}
}
