// The leader's keys, as the review column's are (#750): one key.Binding per
// command carrying every spelling it accepts. The dispatch in routing.go
// matches on these and the help modal's leader table is built from them, so a
// key cannot be handled without being documented, nor documented without
// being handled - #422's fix for the column, which the leader, the table
// invariant 1 makes the one way to command omatty from a session, was the
// last to lack. TestLeader_everyKeyIsInTheTableOrSaysItIsNot_issue750 holds
// the dispatch to it through #751's notice.
//
// A shifted letter takes three spellings, and all three occur (#87, #103,
// #122): Keystroke() spells it with the base key in lower case ("shift+r"), a
// legacy terminal sends the bare upper case ("R"), and one that shifts the
// base key too sends "shift+R".

package app

import "charm.land/bubbles/v2/key"

// leaderBind is every command after the leader.
var leaderBind = struct {
	Down, Up, NextProject, PrevProject, Fold, Switcher, New, NewWorktree, Discover, Adopt,
	Rename, RenameBranch, Agent, Archive, Restart, Stop, Revert, Ship,
	Diff, Files, Gate, Tracker, Zoom, Mouse, Help, Quit key.Binding
}{
	Down:         bind("j / k", "next / previous session, wrapping", "j"),
	Up:           also("k"),
	NextProject:  bind("] / [", "next / previous project", "]"),
	PrevProject:  also("["),
	Fold:         bind("tab", "fold or unfold the project, keeping its sessions", "tab"),
	Switcher:     bind("/", "jump to a session by name or project", "/"),
	New:          bind("n", "new session on the main checkout", "n"),
	NewWorktree:  bind("N", "new session on a fresh worktree", "shift+n", "shift+N", "N"),
	Discover:     bind("a", "register a project claude already knows", "a"),
	Adopt:        bind("A", "adopt a session claude already knows", "shift+a", "shift+A", "A"),
	Rename:       bind("R", "rename the selected session", "shift+r", "shift+R", "R"),
	RenameBranch: bind("B", "rename a worktree session's branch", "shift+b", "shift+B", "B"),
	Agent:        bind("c", "choose the agent the selected project's new sessions run", "c"),
	Archive:      bind("x", "archive the session, or forget an empty project", "x"),
	Restart:      bind("r", "restart a crashed session", "r"),
	Stop:         bind("s", "stop the session's process, keeping it; enter resumes it", "s"),
	Revert:       bind("u", "put the session's worktree back to the start of its last turn", "u"),
	Ship:         bind("p", "ship a green session: push and open its pull request, or merge a green one", "p"),
	Diff:         bind("d", "open or close the diff pane", "d"),
	Files:        bind("f", "open or close the file tree", "f"),
	Gate:         bind("g", "open or close the gate pane", "g"),
	Tracker:      bind("i", "open or close this project's issues and pull requests", "i"),
	Zoom:         bind("z", "zoom the review column over the session, or back", "z"),
	Mouse:        bind("m", "hand the mouse back to your terminal, or take it back", "m"),
	// "?" is shift+/ on a US layout, so it takes the shifted spellings too.
	// Matching "?" alone left the whole keymap unreachable (#103).
	Help: bind("?", "this list", "shift+/", "shift+?", "?"),
	Quit: bind("q", "quit; asks again while a session works or a note is unsent", "q"),
}

// leaderBindings is the table in the order an operator meets it, which is the
// order the help modal lists it in.
func leaderBindings() []key.Binding {
	b := leaderBind
	return []key.Binding{
		b.Down, b.Up, b.NextProject, b.PrevProject, b.Fold, b.Switcher, b.New, b.NewWorktree,
		b.Discover, b.Adopt, b.Rename, b.RenameBranch, b.Agent, b.Archive, b.Restart, b.Stop,
		b.Revert, b.Ship, b.Diff, b.Files, b.Gate, b.Tracker, b.Zoom, b.Mouse, b.Help, b.Quit,
	}
}

// leaderOwns reports whether k is a spelling of any command after the leader.
func leaderOwns(k string) bool {
	for _, b := range leaderBindings() {
		if is(k, b) {
			return true
		}
	}
	return false
}
