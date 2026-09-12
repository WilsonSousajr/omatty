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
)

// tail bounds a step's output, keeping the end. The end is where a test runner
// puts the summary that says what broke; the head is usually the package list.
func tail(out string) string {
	kept, droppedLines := lastLines(out, MaxOutputLines)
	kept, droppedBytes := lastBytes(kept, MaxOutputBytes)
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
