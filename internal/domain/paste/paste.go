// Package paste owns invariant 8: text omatty sends into a session's PTY on
// the operator's behalf travels between bracketed-paste delimiters, and only a
// deliberate carriage return submits it.
//
// It is its own package because two callers need it and neither should import
// the other: review sends a batch of comments (#23), and gate sends a batch of
// step failures (#232). Both are "one message, not N", which is the whole of
// the invariant.
package paste

// Bracketed-paste delimiters (xterm): between them a terminal application
// treats newlines as text rather than as enter (invariant 8).
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// BracketedPaste wraps body so a multi-line message reaches claude as one
// prompt, then appends the single carriage return that submits it. Written
// raw, every newline would submit a fragment (invariant 8).
//
//	term.SendInput(paste.BracketedPaste(body))
func BracketedPaste(body string) string {
	return pasteStart + body + pasteEnd + "\r"
}

// BracketedText wraps body the same way and submits nothing: the operator
// keeps typing after it. Attaching a file reference to the prompt is the
// case, where a carriage return would send a prompt of one path (#199,
// invariant 8).
//
//	term.SendInput(paste.BracketedText("@internal/ui/tree.go "))
func BracketedText(body string) string {
	return pasteStart + body + pasteEnd
}
