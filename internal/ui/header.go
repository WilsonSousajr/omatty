// The header row's pane segment (#177): the breadcrumb on the left, the
// usage on the right, or an open modal's name. ade's header reads app, lane,
// branch, chat; omatty's reads project, session, branch, status, because
// those are the four things a glance at a pane has to answer.

package ui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// headerParts is the pane segment before collapsing: three pieces on the
// left, two on the right, each droppable in the order collapse gives.
type headerParts struct{ Crumb, Branch, State, Meter, Counts string }

// sides joins the pieces present on each side with " · ".
func (p headerParts) sides() (left, right string) {
	return dots(p.Crumb, p.Branch, p.State), dots(p.Meter, p.Counts)
}

// steps is the collapse order: whole, without the counts, without the meter,
// without the branch. The status is never dropped: it is what the header
// exists to say.
func (p headerParts) steps() [4]headerParts {
	noCounts := p
	noCounts.Counts = ""
	noMeter := noCounts
	noMeter.Meter = ""
	noBranch := noMeter
	noBranch.Branch = ""
	return [4]headerParts{p, noCounts, noMeter, noBranch}
}

// collapse lays the parts on width cells, right side right-aligned, dropping
// pieces in steps until they fit; when even the last step does not, the left
// side is cut from the right (#177).
func collapse(width int, p headerParts) string {
	steps := p.steps()
	for _, try := range steps {
		if left, right := try.sides(); fits(left, right, width) {
			return joinEnds(left, right, width)
		}
	}
	left, _ := steps[3].sides()
	return fitLine(left, width)
}

// fits reports whether left and right sit on width cells with the two-space
// gap joinEnds keeps between them.
func fits(left, right string, width int) bool {
	gap := 0
	if right != "" {
		gap = 2
	}
	return lipgloss.Width(left)+gap+lipgloss.Width(right) <= width
}

// dots joins the non-empty parts with the middle dot the rule already used.
func dots(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// paneSegment is the header row's pane share. A modal names itself; a pane
// with no session says nothing; otherwise the breadcrumb collapses into
// width. owns colours the crumb's rail: accent while the pane has the keys.
func (m *Model) paneSegment(now time.Time, width int, owns bool) string {
	if m.modalOpen() {
		return modalName(m.modal)
	}
	row, ok := m.sidebar.Selected()
	if !ok {
		return ""
	}
	return collapse(width, m.paneParts(row, now, owns))
}

// paneParts gathers the selected session's pieces. The title is left
// unstyled so the segment's own style - ink for the keyboard owner, muted
// otherwise - is what colours it.
func (m *Model) paneParts(row Row, now time.Time, owns bool) headerParts {
	st := m.status[row.Session.ID]
	p := headerParts{
		Crumb:  mutedStyle.Render(row.Project) + " " + crumbRail(owns) + " " + row.Session.Title,
		Branch: m.breadcrumbBranch(row.Session.ID),
	}
	if st.Status != "" {
		glyph := glyphStyle(st.Status).Render(statusGlyph(st.Status))
		p.State = strings.TrimSpace(glyph + " " + string(st.Status) + " " + AgeString(now, st.At))
	}
	p.Meter, p.Counts = meterPart(st.Tokens), countsPart(st.Tokens)
	return p
}

// crumbRail marks the session the segment is about: accent while the pane
// owns the keys, muted otherwise, the rail the card wears (#176).
func crumbRail(owns bool) string {
	if owns {
		return accentStyle.Render(rail)
	}
	return mutedStyle.Render(rail)
}

// breadcrumbBranch is the branch piece: what the card's poll last found
// (#180), "" for a session not polled yet, which the third collapse step
// already handles.
func (m *Model) breadcrumbBranch(id string) string { return m.repoStat[id].Branch }

// sidebarSegment is the header row's sidebar share: how many projects.
func (m *Model) sidebarSegment() string {
	return "projects · " + strconv.Itoa(len(m.state.Projects))
}
