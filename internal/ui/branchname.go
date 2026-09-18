// Naming a worktree's branch from its first prompt (#151, #127 step 3).
//
// ctrl+o N was the last prompt that demanded a string before anything could
// start, and it demanded one for a reason: `git worktree add -b` runs at
// creation and bakes the name into a directory and into state.json. So the
// name is still decided at creation - by omatty, as a placeholder - and the
// first prompt renames it, exactly as it renames a placeholder title.
//
// Two rules bound it, and both are about not taking something back that
// someone else already owns:
//
//   - only a placeholder is renamed. A branch the operator typed at creation
//     is theirs.
//   - only while the branch has nothing committed to it. After the first
//     commit the name is in a history that may already have been pushed, and
//     renaming it is the operator's call - ctrl+o B, below.
//
// The worktree *directory* keeps the name it was created with, deliberately.
// claude is running in it and its transcript path is derived from it, so
// moving it would blind the tailer (#60). state.json has always stored Dir and
// Branch separately, so they were never required to agree.
//
// Nothing here blocks: the rename is a tea.Cmd fired from a status event, and
// a failure is a footer line and the branch the session already had.

package ui

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// BranchRenameFunc renames a worktree session's branch, and reports whether it
// did.
//
// unstartedOnly is the automatic path's rule: rename only a branch with
// nothing committed to it, and otherwise return false with a nil error. That
// is a refusal rather than a failure - the rule working - so the operator is
// shown nothing. The operator's own ctrl+o B passes false, because they asked
// for this one and the history is theirs to rename.
//
// Injected because ui never reaches git or the registry store itself
// (invariant 4); cmd/omatty closes it over both.
//
//	deps.RenameBranch = branchRenamer(store, vcs.NewCLI())
type BranchRenameFunc func(sess registry.Session, branch string, unstartedOnly bool) (bool, error)

// noBranchRename is the Deps.RenameBranch default. Like noRename and unlike
// noName, it names the missing wiring rather than appearing to succeed: a
// model that silently kept a placeholder would look like the rule declining.
func noBranchRename(sess registry.Session, branch string, _ bool) (bool, error) {
	return false, fmt.Errorf("ui: no branch renamer configured for session %s (branch %q)", sess.ID, branch)
}

// BranchNamedMsg carries a branch rename's outcome back into Update.
type BranchNamedMsg struct {
	SessionID string
	Branch    string
	Renamed   bool
	Err       error
}

// isPlaceholderBranch reports whether a session is on a worktree still
// carrying the branch name omatty gave it. Derived from the session, never
// tracked, for the reason isPlaceholderTitle gives: a map is empty after a
// relaunch (invariant 9).
func isPlaceholderBranch(sess registry.Session) bool {
	return sess.Worktree && sess.Branch == registry.PlaceholderBranch(sess.ID)
}

// maybeRenameBranch gives a placeholder branch the name the session's title
// just settled on. It is called where the naming chain ends - after the model
// namer has answered, or immediately when there is no model namer - so the
// branch takes the final title rather than the first guess at it.
func (m *Model) maybeRenameBranch(sessionID, title string) tea.Cmd {
	sess, ok := m.session(sessionID)
	branch := registry.Slug(title)
	if !ok || !isPlaceholderBranch(sess) || branch == "" || branch == sess.Branch {
		return nil
	}
	rename := m.renameBranch
	return func() tea.Msg {
		renamed, err := rename(sess, branch, true)
		return BranchNamedMsg{SessionID: sessionID, Branch: branch, Renamed: renamed, Err: err}
	}
}

// onBranchNamed records a rename that happened. A refusal is silent: the
// branch has commits, which is the rule working, and the operator did not ask
// for this rename in the first place.
func (m *Model) onBranchNamed(msg BranchNamedMsg) tea.Cmd {
	if msg.Err != nil {
		slog.Warn("naming branch", "session", msg.SessionID, "branch", msg.Branch, "err", msg.Err)
		m.lastErr = msg.Err.Error()
		return nil
	}
	if msg.Renamed {
		m.rebranch(msg.SessionID, msg.Branch)
	}
	return nil
}

// rebranch updates the model's copy of a session's branch. The store already
// has it; this is the sidebar's view catching up, the way retitle is.
func (m *Model) rebranch(id, branch string) {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			m.state.Sessions[i].Branch = branch
			return
		}
	}
}

// openBranchRename starts editing the selected session's branch, pre-filled
// so correcting a placeholder is a small edit rather than a retype.
//
// The manual key the issue asks for: without it a placeholder that outlived
// its first prompt - a session started and never spoken to, a branch that took
// a commit before the naming landed - would be permanent.
func (m *Model) openBranchRename() {
	row, ok := m.sidebar.Selected()
	if !ok || !row.Session.Worktree {
		return
	}
	m.openModal(modal{
		Kind:   modalBranch,
		Editor: lineEditor{Target: row.Session.ID, Buffer: row.Session.Branch},
	})
}

// commitBranchRename renames by hand. The editor closes first, so a failure
// surfaces in the footer rather than behind a box still asking for input - the
// order commitRename takes, for the same reason.
func (m *Model) commitBranchRename(id, branch string) tea.Cmd {
	m.modal = modal{}
	sess, ok := m.session(id)
	slug := registry.Slug(branch)
	if !ok || slug == "" || slug == sess.Branch {
		return nil
	}
	rename := m.renameBranch
	return func() tea.Msg {
		renamed, err := rename(sess, slug, false)
		return BranchNamedMsg{SessionID: id, Branch: slug, Renamed: renamed, Err: err}
	}
}
