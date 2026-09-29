package ui

import (
	tea "charm.land/bubbletea/v2"
)

// Update routes messages to one handler per type, so it stays a router and
// stays inside the 20-line function limit.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Any message may change what is on screen, so the memoised frame is
	// dropped unless the message took the one path that provably cannot
	// touch model state - onWindowFocus's broadcast, which sets paneOnly.
	// Invalidating by default is what keeps a message type added later from
	// silently leaving a stale screen behind: the cost of forgetting is a
	// rebuild, never a lie.
	m.paneOnly = false
	cmd := tea.Batch(m.routeMsg(msg), m.armSpin(), m.armPreview())
	if !m.paneOnly {
		m.frameMemo.valid = false
	}
	return m, cmd
}

// routeMsg hands one message to the table that owns it.
func (m *Model) routeMsg(msg tea.Msg) tea.Cmd {
	if cmd, ok := m.onInput(msg); ok {
		return cmd
	}
	if cmd, ok := m.onHeartbeat(msg); ok {
		return cmd
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		return m.onResize(size)
	}
	return m.onDataMsg(msg)
}

// onHeartbeat answers the periodic ticks, and says whether it did, so routeMsg
// stays a short list of tables rather than growing a case per timer - #394
// added the fifth.
func (m *Model) onHeartbeat(msg tea.Msg) (tea.Cmd, bool) {
	switch msg.(type) {
	case TickMsg:
		return scheduleTick(), true
	case SpinTickMsg:
		return m.onSpinTick(), true
	case StatTickMsg:
		return m.onStatTick(), true
	case PRTickMsg:
		return m.onPRTick(), true
	case IssueTickMsg:
		return m.onIssueTick(), true
	case SweepTickMsg:
		return m.onSweepTick(), true
	}
	return nil, false
}

// onInput routes what the operator did - a key, the mouse, a paste - and
// reports whether msg was one. Everything else Update sees is what the
// program did. The host's paste brackets arrive as messages of their own and
// are not text: onPaste re-brackets the content itself (#190, invariant 8).
func (m *Model) onInput(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.onKey(msg), true
	case tea.MouseMsg:
		return m.onMouse(msg), true
	case tea.PasteMsg:
		return m.onPaste(msg), true
	case tea.PasteStartMsg, tea.PasteEndMsg:
		return nil, true
	}
	return nil, false
}

// onDataMsg handles the results of work that ran off the Update goroutine, then
// falls through to onSessionMsg and, past that, window focus and the broadcast.
//
// Named switches rather than a default arm that is really a second, unnamed
// one. Every case must be matched by type, because anything unmatched reaches
// broadcast and is fanned out to every emulator at once - so the next person
// adding a message type has to see these lists, not discover them. The mouse
// has its own case in Update for the same reason: a pointer event carries
// window coordinates that mean nothing to an individual pane (#40, #107).
//
// The split is paneCommand's: one table ran past the length limit, and the
// ones after it are named here so they still read as one list (#122).
func (m *Model) onDataMsg(msg tea.Msg) tea.Cmd {
	if cmd, handled := m.onStreamMsg(msg); handled {
		return cmd
	}
	switch typed := msg.(type) {
	case DiffLoadedMsg:
		return m.onDiffLoaded(typed)
	case FilesLoadedMsg:
		return m.onFilesLoaded(typed)
	case WorktreeRemovedMsg:
		return m.onWorktreeRemoved(typed)
	}
	if cmd, handled := m.onNamingMsg(msg); handled {
		return cmd
	}
	if cmd, handled := m.onTurnMsg(msg); handled {
		return cmd
	}
	return m.onPaneMsg(msg)
}

// onTurnMsg answers the turn baseline's two results: a snapshot taken, and a
// turn diff loaded (#311). A table of its own, as onNamingMsg is, because the
// two cases took onDataMsg past the length limit.
func (m *Model) onTurnMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case TurnLoadedMsg:
		return m.onTurnLoaded(typed), true
	case TurnSnappedMsg:
		return m.onTurnSnapped(typed), true
	}
	return nil, false
}

// onNamingMsg answers what names a session: its first prompt, the model's
// improvement on it, and the branch that takes the result (#127, #151). A
// table of its own because onDataMsg ran past the length limit with them in
// it, and they are one conversation.
func (m *Model) onNamingMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case NamedMsg:
		return m.onNamed(typed), true
	case ModelNamedMsg:
		return m.onModelNamed(typed), true
	case BranchNamedMsg:
		return m.onBranchNamed(typed), true
	}
	return nil, false
}

// onStreamMsg is the messages fed by a long-lived source over a channel: the
// watcher's status events and the gate Runner's reports. Each folds itself in
// and re-arms its own wait, which is what separates them from the one-shot
// results below - those answer a tea.Cmd omatty issued once and are done.
//
// A table of its own because onDataMsg was already at the statement limit that
// split it from onSessionMsg (#122), and #231's gate reports pushed it over.
// The second return says whether the message was one of these, so a nil
// command from a handler is not mistaken for "not mine".
func (m *Model) onStreamMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case StatusMsg:
		return m.onStatus(typed), true
	case GateMsg:
		return m.onGate(typed), true
	case coverageMsg:
		m.onCoverage(typed)
		return nil, true
	}
	return m.onColumnMsg(msg)
}

// onColumnMsg is what the review column's own work reports back: a
// classification (#338), a revert (#334) and a ship (#331). A table of its own
// because onStreamMsg was already at the statement limit, and these three share
// a subject - the same argument that split onStreamMsg off onDataMsg.
func (m *Model) onColumnMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case generatedMsg:
		m.onGenerated(typed)
		return nil, true
	case RevertedMsg:
		return m.onReverted(typed), true
	case ShippedMsg:
		return m.onShipped(typed), true
	}
	return nil, false
}

// onPaneMsg is onDataMsg's second table: what a running pane produced on its
// own, as opposed to work omatty went and did. A clipboard write is the first
// of those - the child copied something, and nothing omatty scheduled asked
// it to (#212).
//
// A table of its own rather than a seventh arm above, because onDataMsg is
// already at the statement limit that split it from onSessionMsg in the first
// place (#122), and because the distinction is real: everything above is a
// result coming back, this is a pane speaking unprompted.
func (m *Model) onPaneMsg(msg tea.Msg) tea.Cmd {
	if clip, ok := msg.(ClipboardMsg); ok {
		return m.onClipboard(clip)
	}
	return m.onSessionMsg(msg)
}

// onSessionMsg is the third table: the messages that change which sessions
// exist, or which process is behind one.
func (m *Model) onSessionMsg(msg tea.Msg) tea.Cmd {
	if cmd, ok := m.onForgeMsg(msg); ok {
		return cmd
	}
	switch typed := msg.(type) {
	case ProjectsProposedMsg:
		return m.onProjectsProposed(typed)
	case SessionsProposedMsg:
		return m.onSessionsProposed(typed)
	case sessionRelaunchMsg:
		return m.relaunch(typed.Session)
	case RepoStatMsg:
		return m.onRepoStat(typed)
	}
	return m.onWindowFocus(msg)
}

// onForgeMsg is what the three gh-backed reads answer with (#310, #394, #397),
// split out of onSessionMsg when the third pushed it past the statement limit -
// and they belong together: one forge, three reads.
func (m *Model) onForgeMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case PRsLoadedMsg:
		return m.onPRs(typed), true
	case IssuesLoadedMsg:
		return m.onIssues(typed), true
	case ItemLoadedMsg:
		return m.onItem(typed), true
	case BrowsedMsg:
		return m.onBrowsed(typed), true
	case previewRestMsg:
		return m.onPreviewRest(typed), true // the preview's read, once the cursor rests (#434)
	}
	return nil, false
}

// onWindowFocus records whether omatty itself has the operator's attention,
// which is what gates notifications, and otherwise broadcasts.
func (m *Model) onWindowFocus(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.FocusMsg:
		m.hasFocus = true
		// Catch up on whatever changed while the poll was gated off, rather
		// than leaving stale cards up for the rest of the ten-second period.
		return tea.Batch(m.pollAll(), m.pollPRs(), m.pollIssues())
	case tea.BlurMsg:
		m.hasFocus = false
		return nil
	}
	// Everything else is emulator traffic. Broadcast it: each bubbleterm
	// ignores messages from other emulators, and the message that re-arms a
	// poll must reach the terminal that scheduled it. Unfocused sessions are
	// pumped too, or they stop reading their PTYs (issue #33). Keys are
	// deliberately not broadcast - they belong to the focused session only.
	//
	// This is the one path that mutates a terminal and never the model, so
	// the frame outlives it. The single thing it can change - the focused
	// pane's content - is part of the memo's key, so it is checked rather
	// than assumed.
	m.paneOnly = true
	return m.broadcast(msg)
}

// broadcast forwards msg to every terminal and batches whatever they return.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.terms))
	for _, term := range m.terms {
		if cmd := term.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}
