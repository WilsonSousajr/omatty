package gate

import (
	"strings"
	"testing"
)

// The peak, not just the result. tail bounded what a step contributed, but
// CombinedOutput had already held the whole of it in memory first - which is
// the very thing bound.go's comment says must not happen. The writer keeps a
// bounded window while the step runs.
func TestBoundedOutput_HoldsAtMostItsWindow(t *testing.T) {
	var b boundedOutput
	chunk := strings.Repeat("x", 8<<10)
	total := 0
	for range 40 { // 320 KiB through a KeptBytes window
		n, err := b.Write([]byte(chunk))
		if err != nil || n != len(chunk) {
			t.Fatalf("Write returned %d, %v; want %d, nil", n, err, len(chunk))
		}
		total += len(chunk)
		if len(b.kept) > KeptBytes {
			t.Fatalf("held %d bytes, want at most %d", len(b.kept), KeptBytes)
		}
	}
	if b.dropped != total-len(b.kept) {
		t.Errorf("dropped = %d, want %d: the elision would misreport what was lost", b.dropped, total-len(b.kept))
	}
}

// The window keeps the end, because the end is where a test runner says what
// broke - the same reason tail keeps the tail.
func TestBoundedOutput_KeepsTheEnd(t *testing.T) {
	var b boundedOutput
	_, _ = b.Write([]byte(strings.Repeat("o", KeptBytes)))
	_, _ = b.Write([]byte("THE END"))

	if got := b.String(); !strings.HasSuffix(got, "THE END") {
		t.Errorf("window ends %q, want it to end in the newest bytes", got[max(0, len(got)-16):])
	}
}

// Output that fits is passed through untouched, dropping nothing.
func TestBoundedOutput_UnderTheWindowIsWholeAndDropsNothing(t *testing.T) {
	var b boundedOutput
	_, _ = b.Write([]byte("one\ntwo\n"))

	if got, want := b.String(), "one\ntwo\n"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if b.dropped != 0 {
		t.Errorf("dropped = %d, want 0", b.dropped)
	}
}
