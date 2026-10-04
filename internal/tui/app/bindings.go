// The review column's keys, one key.Binding per action carrying every
// spelling that action accepts (ADR 0001, "Keys use key.Binding"; migration
// step 6.3a, #653). The handlers match on these and the help modal's tables
// are built from them, so a key cannot be handled without being documented -
// the drift #103 and #422 each caught after it shipped. A binding with no help
// text is the second half of the row before it, the k of "j / k".

package app

import (
	"slices"

	"charm.land/bubbles/v2/key"
)

// bind is a binding for keys, documented as the row label for what it does.
func bind(label, does string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(label, does))
}

// also is the second half of the row before it: handled, listed under that row.
func also(keys ...string) key.Binding { return key.NewBinding(key.WithKeys(keys...)) }

// note is a help row with no key of its own: a gesture, or a legend.
func note(label, does string) key.Binding { return key.NewBinding(key.WithHelp(label, does)) }

// is reports whether k is one of b's spellings.
func is(k string, b key.Binding) bool { return slices.Contains(b.Keys(), k) }

// rowsOf is the help modal's rows for bs, in order, skipping the bindings
// that are the second half of a row.
func rowsOf(bs ...key.Binding) []keyHelp {
	out := make([]keyHelp, 0, len(bs))
	for _, b := range bs {
		if h := b.Help(); h.Key != "" {
			out = append(out, keyHelp{Key: h.Key, Does: h.Desc})
		}
	}
	return out
}

// columnBind works the same on every face.
var columnBind = struct {
	Down, Up, Top, Bottom, HalfDown, HalfUp, Left, Right, Wheel, Home, Back, Interrupt key.Binding
}{
	Down:     bind("j / k", "move the cursor, or scroll a preview or an item", "j", "down"),
	Up:       also("k", "up"),
	Top:      bind("g / G", "the first row, or the last", "g"),
	Bottom:   also("G", "shift+g", "shift+G"),
	HalfDown: bind("ctrl+d / ctrl+u", "half a page down, or up", "ctrl+d"),
	HalfUp:   also("ctrl+u"),
	Left:     bind("h / l", "pan sideways", "h", "left"),
	Right:    also("l", "right"),
	Wheel:    note("wheel sideways", "pan too - shift+wheel where the terminal sends it"),
	Home:     bind("0", "jump back to the left edge", "0"),
	Back:     bind("esc", "back a step: a filter, a preview or an item, then the column", "esc"),
	// ctrl+c leaves the column too, and does not quit here: it is the
	// reflex for interrupting claude, and a reviewer's hand is still on it
	// (#28). Undocumented in the column's own table because it is the same
	// key everywhere.
	Interrupt: also("ctrl+c"),
}

// diffBind is the diff's own. S and C each have two spellings, because a
// terminal reporting the shift modifier gives "shift+s" while a legacy one
// gives the bare "S" (#87).
var diffBind = struct {
	Comment, CommentPart, Delete, Submit, Scope, Open, NextFile, PrevFile, NextHunk, PrevHunk, Fold, Reload key.Binding
}{
	Comment:     bind("c", "comment on the line under the cursor", "c"),
	CommentPart: bind("C", "comment on part of the line: type the words, then the note", "shift+c", "C"),
	Delete:      bind("d", "delete the comment under the cursor", "d"),
	Submit:      bind("S", "submit the queued comments", "shift+s", "S"),
	Scope:       bind("t", "the whole session, or only this turn", "t"),
	Open:        bind("o", "open the file at the line under the cursor", "o"),
	NextFile:    bind("] / [", "the next file, or the one before", "]"),
	PrevFile:    also("["),
	NextHunk:    bind("n / N", "the next hunk, or the one before", "n"),
	PrevHunk:    also("N", "shift+n", "shift+N"),
	Fold:        bind("enter", "on a file's header: fold the file to it, or open it", "enter"),
	Reload:      bind("r", "reload the diff", "r"),
}

// treeBind is the file tree's and its preview's.
var treeBind = struct {
	Enter, Read, Generated, Filter, Attach, Open, Reload, Changed, Legend key.Binding
}{
	Enter:     bind("enter", "fold a directory, or preview a file", "enter"),
	Read:      bind("v", "mark the file read; ✓ stays until its diff changes, then ~", "v"),
	Generated: bind(".", "show the generated files it folded away, or fold them again", "."),
	Filter:    bind("/", "filter the tree as you type; enter keeps it, esc clears it", "/"),
	Attach:    bind("a", "attach the row or previewed file to the prompt as @path", "a"),
	Open:      bind("o", "from a preview, jump to that file in the diff", "o"),
	Reload:    bind("r", "re-list the tree", "r"),
	Changed:   bind("c", "only the files the session changed, or all of them again", "c"),
	Legend:    note("M A D R", "a file the session modified, added, deleted or renamed; a folder, the strongest beneath it"),
}

// gateBind is the gate's.
var gateBind = struct {
	Fold, Send, Rerun, Search, NextMatch, PrevMatch key.Binding
}{
	Fold:      bind("enter", "fold a step's output open or shut", "enter"),
	Send:      bind("S", "send the failures to the session; S twice more resends", "S", "shift+s", "shift+S"),
	Rerun:     bind("r", "run the gate again", "r"),
	Search:    bind("/", "search the opened output; enter keeps it, esc clears it", "/"),
	NextMatch: bind("n / N", "the next search match, or the one before", "n"),
	PrevMatch: also("N", "shift+n", "shift+N"),
}

// trackerBind is the tracker's and its open item's.
var trackerBind = struct {
	Read, Filter, Start, Attach, Browse, Reload, NextSection, PrevSection, Fold key.Binding
}{
	Read:        bind("enter", "read the issue or pull request in full", "enter"),
	Filter:      bind("/", "filter by number, title or label", "/"),
	Start:       bind("n", "start a session named and branched after it", "n"),
	Attach:      bind("a", "attach its reference to the prompt", "a"),
	Browse:      bind("b", "open it in the browser", "b"),
	Reload:      bind("r", "read the list, or the open item, again", "r"),
	NextSection: bind("] / [", "the first pull request, or the first issue", "]"),
	PrevSection: also("["),
	Fold:        bind("tab", "fold or unfold the list the cursor is in", "tab"),
}
