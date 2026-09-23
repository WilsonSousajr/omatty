package gate

import (
	"fmt"
	"strings"
)

// Caps on what one step may contribute. `go test ./... -race` on a build that
// does not compile writes without limit, and every byte would otherwise reach
// the sidebar, the gate pane, and from there claude's context.
const (
	// MaxOutputLines is the per-step line cap.
	MaxOutputLines = 200
	// MaxOutputBytes bounds a step that writes few lines but enormous ones.
	MaxOutputBytes = 16 << 10
	// KeptBytes is how much of a step's output is held while it is still
	// running. Four times MaxOutputBytes, so everything tail could keep is
	// still in hand when the step ends, while the peak stays bounded.
	//
	// The caps above bound what a step *contributes*; this one bounds what it
	// *costs to collect*. exec.CombinedOutput held the whole of a runaway
	// step in memory and only then handed it to tail, which is the failure
	// this file's comment describes, one layer earlier than it was fixed.
	KeptBytes = MaxOutputBytes * 4
)

// boundedOutput collects a step's output while holding at most KeptBytes of
// it, keeping the newest bytes and counting what it discarded so the elision
// can still say how much was lost.
//
// Stdout and Stderr are both set to one of these, which os/exec serves from
// a single pipe and a single goroutine when the two are the same writer -
// the same arrangement CombinedOutput makes, so nothing here needs a lock.
type boundedOutput struct {
	kept    []byte
	dropped int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.kept = append(b.kept, p...)
	if excess := len(b.kept) - KeptBytes; excess > 0 {
		b.dropped += excess
		b.kept = b.kept[:copy(b.kept, b.kept[excess:])]
	}
	return len(p), nil
}

// String is everything still inside the window.
func (b *boundedOutput) String() string { return string(b.kept) }

// tail bounds a step's output, keeping the end. The end is where a test runner
// puts the summary that says what broke; the head is usually the package list.
// dropped is what the collecting window already threw away before tail saw
// anything, so a runaway step's elision counts those bytes too rather than
// reporting only the part that reached here.
func tail(out string, dropped int) string {
	kept, droppedLines := lastLines(out, MaxOutputLines)
	kept, droppedBytes := lastBytes(kept, MaxOutputBytes)
	droppedBytes += dropped
	if droppedLines == 0 && droppedBytes == 0 {
		return out
	}
	return elision(droppedLines, droppedBytes) + kept
}

// lastLines keeps at most limit lines from the end, reporting how many it cut.
func lastLines(out string, limit int) (string, int) {
	lines := strings.SplitAfter(out, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	if len(lines) <= limit {
		return out, 0
	}
	dropped := len(lines) - limit
	return strings.Join(lines[dropped:], ""), dropped
}

// lastBytes keeps at most limit bytes from the end, reporting how many it cut.
func lastBytes(out string, limit int) (string, int) {
	if len(out) <= limit {
		return out, 0
	}
	return out[len(out)-limit:], len(out) - limit
}

// elision says what was dropped, so a reader is never shown a silently
// truncated failure and told it is the whole of it.
func elision(lines, bytes int) string {
	if lines > 0 {
		return fmt.Sprintf("… %d earlier lines elided …\n", lines)
	}
	return fmt.Sprintf("… %d earlier bytes elided …\n", bytes)
}
