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

// BracketedText wraps body the same way and submits nothing: the operator
// keeps typing after it. Attaching a file reference to the prompt is the
// case, where a carriage return would send a prompt of one path (#199,
// invariant 8).
//
//	term.SendInput(review.BracketedText("@internal/ui/tree.go "))
func BracketedText(body string) string {
	return pasteStart + body + pasteEnd
}
