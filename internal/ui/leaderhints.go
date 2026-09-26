// The footer while the leader is armed (#439): the keys the next press can
// take, which-key's and Helix's space mode's answer to "what was that key".
//
// In the footer rather than a popup, because a popup over the terminal pane
// would cover claude's screen during the one keystroke the operator is
// choosing, and the footer is omatty's own row. Display only: it reads the
// router's Pending and changes nothing about routing - no key is inspected
// differently and no delay is added (invariant 1).

package ui

// overridingKeys is a footer that replaces the focused pane's: an open modal's,
// whose keys are the only ones that do anything, or the leader's hints while it
// is armed (#439).
func (m *Model) overridingKeys() ([]footerKey, bool) {
	if s := modalFooter(m.modal); s != "" {
		return []footerKey{{s, keepKey}}, true
	}
	if m.router.Pending() {
		return m.leaderHintKeys(), true
	}
	return nil, false
}

// leaderHintKeys is the leader's commands as footer entries, ranked by how
// often they are the one wanted: the help key and the way out never go, the
// column's faces before the session keys, the zoom only when there is a
// column to zoom.
func (m *Model) leaderHintKeys() []footerKey {
	keys := []footerKey{
		{m.leader + " ›", keepKey}, {"d diff", 5}, {"f files", 5}, {"g gate", 5}, {"i tracker", 4},
	}
	if m.review.Open {
		keys = append(keys, footerKey{"z zoom", 4})
	}
	return append(keys,
		footerKey{"j/k switch", 3}, footerKey{"n new", 2}, footerKey{"/ jump", 2}, footerKey{"s stop", 1},
		footerKey{"x archive", 1}, footerKey{"? keys", keepKey}, footerKey{"q quit", keepKey},
	)
}
