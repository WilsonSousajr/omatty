package ui

import (
	"errors"
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
	Seq       uint64 // the load that asked; only the latest is drawn (#352)
}

// ListFilesFunc lists a worktree's files. Injected like DiffFunc so ui never
// reaches git itself (invariant 4, #24).
type ListFilesFunc func(dir string) ([]string, error)

// GeneratedFunc reports which of paths nobody wrote (#338). Injected like
// ListFilesFunc, because the detection asks git about .gitattributes and reads
// file headers, and ui does neither.
type GeneratedFunc func(sess registry.Session, paths []string) (map[string]bool, error)

// TallyFunc records one gate run that followed a turn, and whether it passed
// (#332). Injected because it writes state.json, which ui may not touch itself
// (invariant 10).
type TallyFunc func(project string, passed bool) error

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

// The column's views: the diff, the worktree tree, one file's preview, the
// project's gate (#231), or its tracker (#396).
const (
	ViewDiff ReviewView = iota
	ViewTree
	ViewPreview
	ViewGate
	ViewTracker
	ViewTrackerItem
)

// focusTarget is which pane receives a key the router sends "to the terminal":
// the embedded terminal, the review pane, or the note editor.
type focusTarget int

const (
	focusTerminal focusTarget = iota
	focusReview
	focusNote
	focusFilter // the tree's filter line (#198)
)

// ReviewPane is the right-hand column's state. The zero value is closed.
type ReviewPane struct {
	Open      bool
	Focused   bool
	SessionID string // whose diff is shown
	View      ReviewView
	Diff      review.Diff
	Entries   []review.Entry
	// Scope is the whole session or this turn (#311). TurnDiff is what the
	// turn scope draws, TurnErr its last load's error (ErrNoTurn is a notice,
	// not a failure), and TurnReady whether a load has answered since the
	// scope was entered - until it has, the column says it is reading.
	Scope     reviewScope
	TurnDiff  review.Diff
	TurnErr   error
	TurnReady bool
	DiffList  listWindow // the cursor over Entries (#424)
	Err       string
	Note      noteEditor
	// Filter is the tree's type-to-filter line (#198): Active while it has
	// the keys, Query the text in force after enter kept it.
	Filter filterLine
	// GateSearch is the gate's / search over opened output (#429). Its own,
	// not Filter: a tree filter left in place must never become a search of
	// the gate's output, nor a gate search narrow the tree.
	GateSearch filterLine
	// The tree view's state (#24). Tree is nil until the listing arrives,
	// which is what the "listing files..." placeholder means. TreeErr is
	// separate from Err so a failed listing never blanks the diff, and a
	// failed diff never blanks the tree: the two load independently.
	Tree    *review.Tree
	TreeErr string
	Files   listWindow // the cursor over the tree's visible rows (#424)
	// The gate view's state (#231). GateOpen is which steps are folded open,
	// nil until one is - a step's output is hidden by default because four
	// steps of test output would bury the summary the pane exists to show.
	GateCursor int
	GateOffset int
	GateOpen   map[int]bool
	// The tracker view's state (#396), keyed by project rather than by session:
	// it is the one view that belongs to a project, so following the sidebar
	// between two sessions of one project must not throw it away.
	Tracker trackerList
	// The preview view's state: one file at a time, so a new preview
	// replaces the last rather than accumulating.
	Preview       review.Preview
	PreviewOffset int
	// ColOffset is the horizontal counterpart of the three vertical offsets
	// above: how many display cells every content row is scrolled left, so a
	// line wider than the column can still be read (issue #94). One offset
	// serves all three views, so h and l behave the same wherever you are.
	ColOffset int
	// Zoomed widens the column over the session pane (#427). A viewing state
	// on the pane, so every reset of the pane drops it and nothing persists it
	// (invariant 9); it shows only while the column has the keys - see zoomed.
	Zoomed bool
	// Stale marks content loaded before a turn that ended while the column
	// was closed. A hidden pane does not fork git; the reopen does (#124).
	Stale bool
	// Widest memoizes the current view's widest row for the horizontal clamp
	// (#133).
	Widest widthCache
}

// reviewScope is how much of the session the diff view shows (#311).
type reviewScope int

const (
	scopeSession reviewScope = iota // everything the session changed
	scopeTurn                       // what changed since this turn began
)

// TurnLoadedMsg carries a loaded turn diff into Update. Exported so tests
// can send one.
type TurnLoadedMsg struct {
	SessionID string
	Diff      review.Diff
	Err       error
	Seq       uint64 // the load that asked; only the latest is drawn (#352)
}

// shownDiff is the diff the rows are drawn from and indexed into. PruneSent,
// Compose and the tree's markers keep m.review.Diff, the whole session, on
// purpose (#311).
func (m *Model) shownDiff() review.Diff {
	if m.review.Scope == scopeTurn {
		return m.review.TurnDiff
	}
	return m.review.Diff
}

// filterLine is the tree's live filter: / opens it, typing narrows the
// listing, enter keeps the query and hands the keys back, esc clears it.
type filterLine struct {
	Active bool
	Query  string
}

// noteEditor is the one-line comment input opened with c on a diff line. It
// holds the anchor rather than a position so a reload while typing cannot
// leave it pointing past the end of a shorter diff.
type noteEditor struct {
	Active bool
	Anchor review.Anchor
	Quote  string
	Buffer string
	// Fragment is the part of Quote the note is about, and Stage says which of
	// the two things the buffer is collecting (#339). A whole-line note opened
	// with c never leaves stageNote and never sets Fragment, so it behaves
	// exactly as it did.
	Fragment string
	Stage    noteStage
}

// noteStage is which prompt the note editor is showing.
type noteStage int

const (
	// stageNote is the note itself, and the zero value: c opens straight into
	// it, which is the whole-line path this feature must not disturb.
	stageNote noteStage = iota
	// stageFragment collects the part of the line first, for C (#339).
	stageFragment
)

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
// closes it when it already shows v - from either focus state, which is what
// the help text has promised since #103 and what esc-then-leader needs to
// terminate (#124). Content survives a close: it is keyed by SessionID and
// only a different session throws it away. Comments survive too: they live
// on the model, keyed by session (#21). The tree's collapse state does not -
// it is rebuilt from the listing, which is cheap and always current (#24).
func (m *Model) toggleView(v ReviewView) tea.Cmd {
	// The preview is the tree's child (keptView), so f over a preview closes
	// the column rather than switching it back to the listing (#124).
	if m.review.Open && keptView(m.review.View) == v {
		return m.closeColumn()
	}
	id := m.Selected()
	if id == "" {
		return nil
	}
	wasOpen, fresh := m.review.Open, m.review.SessionID != id
	if fresh {
		m.review = ReviewPane{SessionID: id}
	}
	// Each view is a different shape of text, so a pan that made sense in one
	// is meaningless in the next: switching starts at the left edge (#94).
	m.review.Open, m.review.View, m.review.Focused, m.review.ColOffset = true, v, true, 0
	if v == ViewGate {
		// Opening the gate asks for a fresh one. A pane that only showed the
		// last result would be a report with no way to ask for a new one.
		m.runGate(id)
	}
	return tea.Batch(m.resizeIfWidthChanged(wasOpen), m.reloadIfNeeded(id, fresh), m.loadFilesIfMissing(id))
}

// resizeIfWidthChanged resizes the terminal only when the column appeared:
// switching views changes no width. An identical Resize would send claude
// nothing at all - the kernel skips SIGWINCH for an unchanged window (#191)
// - but it would still reflow the emulator's grid and mark it damaged for
// no reason (#95).
func (m *Model) resizeIfWidthChanged(wasOpen bool) tea.Cmd {
	if wasOpen {
		return nil
	}
	return m.resizeSelected()
}

// reloadIfNeeded fetches the diff for a session the column has not loaded,
// or one whose cached diff went stale behind a closed column (#124), and
// re-lists a stale tree beside it (#195). A column already open on this
// session keeps what it loaded: re-forking git for a diff already in memory
// is the stall loadDiff's own comment exists to avoid.
func (m *Model) reloadIfNeeded(id string, fresh bool) tea.Cmd {
	if !fresh && !m.review.Stale {
		return nil
	}
	m.review.Stale = false
	return tea.Batch(m.loadDiff(id), m.relistFiles(id))
}

// closeColumn hides the column and gives the keys back, keeping what it
// loaded so reopening on the same session asks git nothing. #90 made the
// leader refocus instead of close, to spare that reload; the cache answers
// the reload, and the refocus made esc-then-leader an endless loop (#124).
func (m *Model) closeColumn() tea.Cmd {
	m.review.Open, m.review.Focused, m.review.Zoomed = false, false, false
	return m.resizeSelected()
}

// loadFullDiff fetches the whole-session diff off the Update goroutine: git on
// a large tree takes long enough to stall the frame.
func (m *Model) loadFullDiff(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	m.diffSeq++
	root, load, seq := m.projectRoot(sess.Project), m.diff, m.diffSeq
	return func() tea.Msg {
		d, err := load(sess, root)
		return DiffLoadedMsg{SessionID: id, Diff: d, Err: err, Seq: seq}
	}
}

// loadDiff reloads what the column shows: always the whole session, which
// the tree's markers and PruneSent read, and the turn as well while the turn
// scope is on, so it refreshes on every trigger the full diff has.
func (m *Model) loadDiff(id string) tea.Cmd {
	return tea.Batch(m.loadFullDiff(id), m.loadTurn(id))
}

// loadTurn fetches the turn diff when the column is showing that session's
// turn, and nothing otherwise.
func (m *Model) loadTurn(id string) tea.Cmd {
	if !m.review.Open || m.review.Scope != scopeTurn || id != m.review.SessionID || m.hooksDown {
		return nil
	}
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	m.turnSeq++
	root, load, seq := m.projectRoot(sess.Project), m.turn.Diff, m.turnSeq
	return func() tea.Msg {
		d, err := load(sess, root)
		return TurnLoadedMsg{SessionID: id, Diff: d, Err: err, Seq: seq}
	}
}

// onTurnLoaded paints a turn diff, unless the column moved on or went back
// to the whole session while git ran.
func (m *Model) onTurnLoaded(msg TurnLoadedMsg) tea.Cmd {
	if !m.review.Open || msg.SessionID != m.review.SessionID || m.review.Scope != scopeTurn || msg.Seq != m.turnSeq {
		return nil
	}
	if msg.Err != nil && !errors.Is(msg.Err, review.ErrNoTurn) {
		slog.Warn("loading a turn diff", "session", msg.SessionID, "err", msg.Err)
	}
	m.review.TurnDiff, m.review.TurnErr, m.review.TurnReady = msg.Diff, msg.Err, true
	m.rebuildEntries()
	return nil
}

// onDiffLoaded paints a freshly loaded diff, unless the pane closed or moved
// to another session while git was running.
func (m *Model) onDiffLoaded(msg DiffLoadedMsg) tea.Cmd {
	if !m.review.Open || msg.SessionID != m.review.SessionID || msg.Seq != m.diffSeq {
		return nil
	}
	if msg.Err != nil {
		slog.Warn("loading diff", "session", msg.SessionID, "err", msg.Err)
		m.review.Err = causeOf(msg.Err)
		return nil
	}
	m.review.Err = ""
	m.review.Diff = msg.Diff
	// Only a diff that actually loaded may prune: rebuildEntries also runs
	// before the first load, against an empty diff every comment misses.
	m.commentsFor(msg.SessionID).PruneSent(msg.Diff)
	m.rebuildEntries()
	m.retouchTree()
	return m.classifyAfterDiff(msg.SessionID)
}

// rebuildEntries re-places the comments against the current diff and keeps the
// cursor on a valid row.
func (m *Model) rebuildEntries() {
	d, comments := m.shownDiff(), m.commentsFor(m.review.SessionID).All()
	placed := review.Place(d, comments)
	if m.review.Scope == scopeTurn {
		// Through the session diff, by line rather than first match; what
		// does not place is elsewhere in the session, not moved (#311).
		placed = review.PlaceIn(m.review.Diff, d, comments)
	}
	m.review.Entries = review.Flatten(d, placed)
	m.contentChanged()
	if m.review.DiffList.Cursor >= len(m.review.Entries) {
		m.review.DiffList.Cursor = max(len(m.review.Entries)-1, 0)
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
	id := m.Selected()
	if !m.review.Open || id == "" || id == m.review.SessionID {
		return nil
	}
	sess, _ := m.session(id)
	m.review = ReviewPane{
		Open: true, Focused: m.review.Focused, SessionID: id, View: keptView(m.review.View),
		// The tracker is the project's, not the session's: moving along the
		// sidebar inside one project keeps its list and its cursor, and moving
		// to another project re-points it (#396).
		Tracker: keptTracker(m.review.Tracker, sess.Project),
	}
	if m.review.View == ViewTracker {
		return m.readTracker(m.review.Tracker.Project)
	}
	return tea.Batch(m.loadDiff(id), m.loadFiles(id))
}

// keptView is the view a column carries to another session, and the view its
// own leader key toggles: a child view degrades to its parent, so ctrl+o f over
// a preview and ctrl+o i over an item both close the column (#124, #397).
func keptView(v ReviewView) ReviewView {
	switch v {
	case ViewPreview:
		return ViewTree
	case ViewTrackerItem:
		return ViewTracker
	}
	return v
}

func (m *Model) session(id string) (registry.Session, bool) {
	i, ok := m.sessionIndex(id)
	if !ok {
		return registry.Session{}, false
	}
	return m.state.Sessions[i], true
}

// sessionIndex is where id sits in m.state.Sessions, if it is there at all.
//
// One scan for the whole package: this loop had been hand-written five times
// (session, knownSession, sessionTitle, retitle, forgetSession) and each copy
// had invented its own answer for a miss - the zero value, false, the id
// itself, or a silent return. Callers that need to mutate take the index;
// callers that only read go through session (#40, #41).
func (m *Model) sessionIndex(id string) (int, bool) {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			return i, true
		}
	}
	return 0, false
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
// and a diff from before the turn would be stale on arrival (#21). The
// listing goes with it, so a file claude created appears without r (#195).
func (m *Model) refreshReview(id string, before, after watcher.Status) tea.Cmd {
	if id != m.review.SessionID || before == after {
		return nil
	}
	if after != watcher.StatusDone && after != watcher.StatusWaiting {
		return nil
	}
	// A closed column keeps its content for the reopen (#124); forking git
	// for a pane nobody can see would be waste, so the reopen pays instead.
	if !m.review.Open {
		m.review.Stale = true
		return nil
	}
	return tea.Batch(m.loadDiff(id), m.relistFiles(id))
}

// reviewOwnsKeys reports whether a plain keystroke would reach the review
// column right now.
//
// m.review.Focused alone is not that question: a modal takes the keyboard
// without clearing the flag, so the footer went on advertising j/k, c, d, r, S
// and esc while every one of them typed a character into the prompt instead.
// One flag was answering two questions, which is the confusion #95 came from.
func (m *Model) reviewOwnsKeys() bool {
	return m.review.Focused && !m.modalOpen()
}
