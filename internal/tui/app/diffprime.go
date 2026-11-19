// Priming the diff's lexer from the file's start (#800).

package app

import (
	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	dsession "github.com/WilsonSousajr/omatty/internal/domain/session"

	tea "charm.land/bubbletea/v2"
)

// lexedFile is one file of the shown diff with each side lexed whole: the
// colours by line, and the plain text they were lexed from, so a line is
// only taken from the file while the file still reads as the diff does.
type lexedFile struct {
	old, oldPlain []string
	new, newPlain []string
}

// prime gives each line of a hunk its colours from its side lexed whole, so
// a hunk that opens inside a raw string or a block comment is coloured as the
// file reads (#800). A line the file no longer has - edited again since git
// answered - keeps the hunk's own lexing. A nil lexedFile primes nothing.
func (lf *lexedFile) prime(styled, plain []string, lines []dreview.Line) {
	if lf == nil {
		return
	}
	for i, l := range lines {
		if s, ok := lf.lineFor(l, plain[i]); ok {
			styled[i] = s
		}
	}
}

// lineFor is a line's colours from its side: a removed line is the old
// file's, an added or a context line the new file's, as lexSides draws them.
func (lf *lexedFile) lineFor(l dreview.Line, plain string) (string, bool) {
	if l.Kind == dreview.LineRemoved {
		return lineAtNo(lf.old, lf.oldPlain, l.OldNo, plain)
	}
	return lineAtNo(lf.new, lf.newPlain, l.NewNo, plain)
}

// lineAtNo is line no (1-based) of a side lexed whole, if it reads want.
func lineAtNo(styled, plain []string, no int, want string) (string, bool) {
	if no < 1 || no > len(plain) || no > len(styled) || plain[no-1] != want {
		return "", false
	}
	return styled[no-1], true
}

// OldFileFunc reads path as it was at rev in sess's checkout: the diff's old
// side (#800). Injected, because the TUI may not touch git (invariant 4).
type OldFileFunc func(sess dsession.Session, rev, path string) (dreview.Preview, error)

// SidesLoadedMsg carries a diff's files lexed whole into Update, numbered by
// the diff load it was read for so a late one cannot colour a newer diff.
// Exported so tests can send one.
type SidesLoadedMsg struct {
	SessionID string
	Turn      bool   // the turn's diff, not the session's
	Seq       uint64 // the diffSeq or turnSeq the diff was loaded under
	Files     map[string]*lexedFile
}

// primedDiffs is the files of each of the column's two diffs, lexed whole:
// the session's and the turn's, which load apart and are shown by scope.
type primedDiffs struct{ full, turn map[string]*lexedFile }

// of is one diff's files.
func (p primedDiffs) of(turn bool) map[string]*lexedFile {
	if turn {
		return p.turn
	}
	return p.full
}

// set replaces one diff's files.
func (p *primedDiffs) set(turn bool, files map[string]*lexedFile) {
	if turn {
		p.turn = files
		return
	}
	p.full = files
}

// primeFiles bounds how many of a diff's files are read whole per load: each
// is a git call and a lex, off the Update goroutine but not free. The rest
// are lexed by hunk, as before.
const primeFiles = 64

// loadSides reads and lexes the changed files of a diff that just landed, off
// the Update goroutine: two file reads and two lexes a file, which a frame
// cannot wait on (#197's budget). Until they land the diff is drawn from its
// hunks, and the last diff's sides are dropped, since its lines may have moved.
func (m *Model) loadSides(id string, d dreview.Diff, turn bool, seq uint64) tea.Cmd {
	m.primed.set(turn, nil)
	sess, ok := m.reviewSession(id)
	if !ok || m.oldFile == nil || d.Base == "" {
		return nil
	}
	r := sideReader{hl: m.highlighter, preview: m.preview, old: m.oldFile, sess: sess, base: d.Base}
	return func() tea.Msg {
		return SidesLoadedMsg{SessionID: id, Turn: turn, Seq: seq, Files: r.lexAll(d)}
	}
}

// onSidesLoaded keeps a diff's lexed files if that diff is still the one
// shown, and redraws its hunks with them.
func (m *Model) onSidesLoaded(msg SidesLoadedMsg) tea.Cmd {
	current := m.diffSeq
	if msg.Turn {
		current = m.turnSeq
	}
	if !m.review.Open || msg.SessionID != m.review.SessionID || msg.Seq != current {
		return nil
	}
	m.primed.set(msg.Turn, msg.Files)
	m.review.HunkStyles = nil // drawn without them; drawn again with
	return nil
}

// sideReader is what loadSides needs off the Update goroutine, copied out of
// the model so the goroutine reads none of it.
type sideReader struct {
	hl      Highlighter
	preview PreviewFunc
	old     OldFileFunc
	sess    dsession.Session
	base    string
}

// lexAll is the first primeFiles text files of d with a hunk, lexed whole.
func (r sideReader) lexAll(d dreview.Diff) map[string]*lexedFile {
	out := map[string]*lexedFile{}
	for _, f := range d.Files {
		if len(out) == primeFiles {
			break
		}
		if lf := r.lexFile(f); lf != nil {
			out[f.Path] = lf
		}
	}
	return out
}

// lexFile is f's two sides lexed whole: the old one at the diff's base, under
// the name it had there, and the new one as the worktree holds it. A side
// that does not exist - an added or a deleted file - or cannot be read is
// left empty, and its lines are lexed by hunk.
func (r sideReader) lexFile(f dreview.File) *lexedFile {
	if f.Binary || len(f.Hunks) == 0 {
		return nil
	}
	lf := &lexedFile{}
	lf.old, lf.oldPlain = r.oldSide(f)
	lf.new, lf.newPlain = r.newSide(f)
	if lf.old == nil && lf.new == nil {
		return nil
	}
	return lf
}

// oldSide is f at the diff's base, lexed whole; nothing for an added file.
func (r sideReader) oldSide(f dreview.File) (styled, plain []string) {
	if f.Status == dreview.FileAdded {
		return nil, nil
	}
	p, err := r.old(r.sess, r.base, oldPathOf(f))
	return lexWhole(r.hl, f.Path, p, err)
}

// newSide is f in the worktree, lexed whole; nothing for a deleted file.
func (r sideReader) newSide(f dreview.File) (styled, plain []string) {
	if f.Status == dreview.FileDeleted {
		return nil, nil
	}
	p, err := r.preview(r.sess.Dir, f.Path)
	return lexWhole(r.hl, f.Path, p, err)
}

// oldPathOf is the name f had at the diff's base: its old path when renamed.
func oldPathOf(f dreview.File) string {
	if f.OldPath != "" {
		return f.OldPath
	}
	return f.Path
}

// lexWhole is one side read whole and lexed from its first line, tabs
// expanded as the diff draws them; nothing when it could not be read, is not
// text, was cut at the read bound, or is over the budget a lex may take
// (stylePreview's, #197).
func lexWhole(hl Highlighter, path string, p dreview.Preview, err error) (styled, plain []string) {
	if err != nil || p.Binary || p.Truncated || len(p.Lines) == 0 || previewBytes(p.Lines) > highlightBudget {
		return nil, nil
	}
	plain = make([]string, len(p.Lines))
	for i, line := range p.Lines {
		plain[i] = expandTabs(line)
	}
	if styled = hl.Lines(path, plain); len(styled) != len(plain) {
		return nil, nil
	}
	return styled, plain
}
