package review

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
//	term.SendInput(review.BracketedPaste(body))
func BracketedPaste(body string) string {
	return pasteStart + body + pasteEnd + "\r"
}
