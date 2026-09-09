package highlight

import (
	"sync"

	"github.com/alecthomas/chroma/v2"
)

// The style is omatty's own rather than a stock chroma theme, because of
// the colour rule internal/ui/style.go states: one hue, one meaning. The
// accent (75) means focus alone, and amber (214), green (78) and red (203)
// mean a queued comment, an added line and a removed line. Every stock theme
// spends those on keywords and strings, so a highlighted preview would read
// as a diff full of the operator's own comments.
//
// The indices below are xterm-256 colours written as the hex the terminal
// formatter maps back to that index. They sit off the reserved four and off
// each other: keyword 111 (a soft blue, quieter than the accent), type 117
// (cyan), string 180 (tan), number 176 (mauve), function 152 (grey-cyan),
// comment 245 (the muted text hue). Plain names and punctuation carry no
// style at all, so unstyled text stays the terminal's own foreground, as the
// preview always drew it. At 16 colours the keyword blue and the accent both
// quantise to bright blue, which is tolerable: the accent never appears
// inside preview text, only on the chrome around it (#197).
const (
	hexKeyword  = "#87afff" // 111
	hexType     = "#87d7ff" // 117
	hexString   = "#d7af87" // 180
	hexNumber   = "#d75fd7" // 176
	hexFunction = "#afd7d7" // 152
	hexComment  = "#8a8a8a" // 245
)

// style is built on first use rather than at package init: a style that
// fails to parse should fail where the preview is opened, not before main
// runs (AGENTS.md's rule against init-time state).
var style = sync.OnceValue(func() *chroma.Style {
	return chroma.MustNewStyle("omatty", chroma.StyleEntries{
		chroma.Keyword:       hexKeyword,
		chroma.KeywordType:   hexType,
		chroma.NameClass:     hexType,
		chroma.NameBuiltin:   hexType,
		chroma.NameFunction:  hexFunction,
		chroma.LiteralString: hexString,
		chroma.LiteralNumber: hexNumber,
		chroma.Comment:       hexComment,
	})
})
