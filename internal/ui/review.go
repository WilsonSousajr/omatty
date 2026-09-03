package ui

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// DiffFunc loads a session's diff. Injected so ui never touches git
// (invariant 4); projectRoot is the session's project's main checkout, the
// fallback base for worktrees that recorded none.
type DiffFunc func(sess registry.Session, projectRoot string) (review.Diff, error)

// DiffLoadedMsg carries a loaded diff into Update. Exported so tests can send
// one.
type DiffLoadedMsg struct {
	SessionID string
	Diff      review.Diff
	Err       error
}

// ListFilesFunc lists a worktree's files. Injected like DiffFunc so ui never
// reaches git itself (invariant 4, #24).
type ListFilesFunc func(dir string) ([]string, error)

// PreviewFunc reads one file for the preview view, so a test never touches
// the filesystem.
type PreviewFunc func(dir, rel string) (review.Preview, error)

// FilesLoadedMsg carries a worktree listing into Update. Exported so tests
// can send one.
type FilesLoadedMsg struct {
	SessionID string
	Paths     []string
	Err       error
}

// ReviewView is which face the review column shows.
type ReviewView int

// The column's views: the diff, the worktree tree, or one file's preview.
const (
	ViewDiff ReviewView = iota
	ViewTree
	ViewPreview
)

// focusTarget is which pane receives a key the router sends "to the terminal":
// the embedded terminal, the review pane, or the note editor.
type focusTarget int

const (
	focusTerminal focusTarget = iota
	focusReview
	focusNote
)

// ReviewPane is the right-hand column's state. The zero value is closed.
type ReviewPane struct {
	Open      bool
	Focused   bool
	SessionID string // whose diff is shown
	View      ReviewView
	Diff      review.Diff
	Entries   []review.Entry
	Cursor    int
	Offset    int // first visible entry
	Err       string
	Note      noteEditor
	// The tree view's state (#24). Tree is nil until the listing arrives,
	// which is what the "listing files..." placeholder means. TreeErr is
	// separate from Err so a failed listing never blanks the diff, and a
	// failed diff never blanks the tree: the two load independently.
	Tree       *review.Tree
	TreeErr    string
	TreeCursor int
	TreeOffset int
	// The preview view's state: one file at a time, so a new preview
	// replaces the last rather than accumulating.
	Preview       review.Preview
	PreviewOffset int
}

// noteEditor is the one-line comment input opened with c on a diff line. It
// holds the anchor rather than a position so a reload while typing cannot
// leave it pointing past the end of a shorter diff.
type noteEditor struct {
	Active bool
	Anchor review.Anchor
	Quote  string
	Buffer string
}

// noDiff is the Deps.Diff default: it names the missing wiring rather than
// showing an empty diff, which would read as "this session changed nothing".
func noDiff(sess registry.Session, _ string) (review.Diff, error) {
	return review.Diff{}, fmt.Errorf("ui: no diff source configured for session %s", sess.ID)
}

// noFiles is the Deps.Files default, for the same reason as noDiff: an empty
// tree would read as "this worktree is empty" (#24).
func noFiles(dir string) ([]string, error) {
	return nil, fmt.Errorf("ui: no file lister configured for %q", dir)
}

// ReviewOpen reports whether the review column is shown.
func (m *Model) ReviewOpen() bool { return m.review.Open }

// ReviewFocused reports whether plain keys go to the review column.
func (m *Model) ReviewFocused() bool { return m.review.Focused }

// ReviewView reports which face the column shows.
func (m *Model) ReviewView() ReviewView { return m.review.View }

// toggleView opens the review column on v, switches an open column to v, or
// closes it when it already shows v. Either way the terminal is resized to
// what is left (#21). Comments survive a close: they live on the model, keyed
// by session. The tree's collapse state does not - it is rebuilt from the
// listing, which is cheap and always current (#24).
func (m *Model) toggleView(v ReviewView) tea.Cmd {
	if m.review.Open && m.review.View == v {
		m.review = ReviewPane{}
		return m.resizeFocused()
	}
	id := m.Focused()
	if id == "" {
		return nil
	}
	if !m.review.Open || m.review.SessionID != id {
		m.review = ReviewPane{Open: true, SessionID: id}
	}
	m.review.View, m.review.Focused = v, true
	return tea.Batch(m.resizeFocused(), m.loadDiff(id), m.loadFiles(id))
}

// loadDiff fetches the diff off the Update goroutine: git on a large tree
// takes long enough to stall the frame.
func (m *Model) loadDiff(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	root, load := m.projectRoot(sess.Project), m.diff
	return func() tea.Msg {
		d, err := load(sess, root)
		return DiffLoadedMsg{SessionID: id, Diff: d, Err: err}
	}
}

// onDiffLoaded paints a freshly loaded diff, unless the pane closed or moved
// to another session while git was running.
func (m *Model) onDiffLoaded(msg DiffLoadedMsg) tea.Cmd {
	if !m.review.Open || msg.SessionID != m.review.SessionID {
		return nil
	}
	if msg.Err != nil {
		slog.Warn("loading diff", "session", msg.SessionID, "err", msg.Err)
		m.review.Err = msg.Err.Error()
		return nil
	}
	m.review.Err = ""
	m.review.Diff = msg.Diff
	m.rebuildEntries()
	m.retouchTree()
	return nil
}

// rebuildEntries re-places the comments against the current diff and keeps the
// cursor on a valid row.
func (m *Model) rebuildEntries() {
	placed := review.Place(m.review.Diff, m.commentsFor(m.review.SessionID).All())
	m.review.Entries = review.Flatten(m.review.Diff, placed)
	if m.review.Cursor >= len(m.review.Entries) {
		m.review.Cursor = max(len(m.review.Entries)-1, 0)
	}
}

// commentsFor returns the session's queue, creating it on first use.
func (m *Model) commentsFor(id string) *review.Comments {
	if m.comments[id] == nil {
		m.comments[id] = review.NewComments()
	}
	return m.comments[id]
}

// followSession moves an open review column to the newly focused session,
// keeping the view it was showing: an operator who moves along the sidebar
// with the tree open wants the next session's tree, not its diff (#24). A
// preview belongs to the file it read, so it degrades to the tree.
func (m *Model) followSession() tea.Cmd {
	id := m.Focused()
	if !m.review.Open || id == "" || id == m.review.SessionID {
		return nil
	}
	m.review = ReviewPane{
		Open: true, Focused: m.review.Focused, SessionID: id, View: keptView(m.review.View),
	}
	return tea.Batch(m.loadDiff(id), m.loadFiles(id))
}

// keptView is the view a column carries to another session.
func keptView(v ReviewView) ReviewView {
	if v == ViewPreview {
		return ViewTree
	}
	return v
}

func (m *Model) session(id string) (registry.Session, bool) {
	for _, s := range m.state.Sessions {
		if s.ID == id {
			return s, true
		}
	}
	return registry.Session{}, false
}

func (m *Model) projectRoot(name string) string {
	for _, p := range m.state.Projects {
		if p.Name == name {
			return p.Root
		}
	}
	return ""
}

// refreshReview reloads the open diff when its session finishes a turn or
// stops for a question: that is the moment the operator looks at what changed,
// and a diff from before the turn would be stale on arrival (#21).
func (m *Model) refreshReview(id string, before, after watcher.Status) tea.Cmd {
	if !m.review.Open || id != m.review.SessionID || before == after {
		return nil
	}
	if after != watcher.StatusDone && after != watcher.StatusWaiting {
		return nil
	}
	return m.loadDiff(id)
}
