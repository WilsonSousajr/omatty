// Untrusted text in, plain text out (#483).
//
// Everything this package folds from gh was written by someone else: on a
// public repository, anyone. The renderers downstream are ANSI-aware - they
// measure around an escape sequence and keep it - so a control character left
// in a title or a body reached the terminal as written: an issue body could
// write the operator's clipboard with OSC 52, clear the screen, or draw over
// omatty's own panes the moment it was opened. AGENTS.md says forge text is
// data, never instructions, and a control sequence is an instruction to the
// terminal. So it is stripped here, at the edge, once: every later reader,
// and any added later, gets plain text.

package forge

import "strings"

// clean drops every control character from s but its newlines and tabs, which
// are a body's own layout: C0, DEL, and C1 - whose 0x9b is a whole CSI on a
// terminal that honours 8-bit controls.
//
//	forge.clean("hi\x1b]52;c;ZXZpbA==\x07") // "hi]52;c;ZXZpbA=="
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !isControl(r) {
			return r
		}
		return -1
	}, s)
}

// cleanLine is clean for a one-line field - a title, a label, a name, a
// branch - where a newline or a tab would break the row it is drawn in.
func cleanLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return ' '
		}
		return r
	}, clean(s))
}

// isControl is C0, DEL or C1.
func isControl(r rune) bool {
	return r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}
