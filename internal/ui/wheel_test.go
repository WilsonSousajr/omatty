package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// overPane is a window cell inside the focused session's pane; overSidebar is
// one outside it, and overReview one inside the review column. All three are
// derived from the layout rather than written down, so a change to it moves
// the tests with it.
func overPane() (x, y int) {
	ox, oy := ui.PaneOrigin()
	return ox + 5, oy + 3
}

func overSidebar() (x, y int) {
	_, oy := ui.PaneOrigin()
	return ui.SidebarWidth / 2, oy + 3
}

func overReview() (x, y int) {
	_, oy := ui.PaneOrigin()
	return ui.DefaultWidth - ui.ReviewWidth(ui.DefaultWidth, true) + 2, oy + 3
}

func wheel(x, y int, b tea.MouseButton) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: b}
}

// spin sends n notches of the same button, which is what a real flick is.
func spin(m *ui.Model, n int, b tea.MouseButton) {
	x, y := overPane()
	for range n {
		m.Update(wheel(x, y, b))
	}
}

// pageKeys pulls the page-up/page-down keys out of what a fake terminal was
// sent, ignoring everything else it received.
func pageKeys(msgs []tea.Msg) []rune {
	var got []rune
	for _, msg := range msgs {
		if k, ok := msg.(tea.KeyPressMsg); ok && (k.Code == tea.KeyPgUp || k.Code == tea.KeyPgDown) {
			got = append(got, k.Code)
		}
	}
	return got
}

// The fix: a wheel notch becomes the key claude documents for scrolling.
// Before it, the notch became arrow keys via the terminal's alternate scroll
// and edited the prompt instead.
//
// It is the key message, not its bytes: bubbleterm owns the key-to-escape
// translation, so a hand-written "\x1b[5~" would diverge from every other key
// the moment that encoder changes.
func TestUpdate_TheWheelScrollsWithClaudesOwnKeys_issue107(t *testing.T) {
	for _, tt := range []struct {
		name   string
		button tea.MouseButton
		want   rune
	}{
		{"down", tea.MouseWheelDown, tea.KeyPgDown},
		{"up", tea.MouseWheelUp, tea.KeyPgUp},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, fakes := modelWithFakes(t)

			spin(m, ui.WheelNotchesPerPage, tt.button)

			got := pageKeys(fakes["s1"].Msgs)
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("page keys = %v, want exactly [%v]", got, tt.want)
			}
			if sent := fakes["s1"].Sent; len(sent) != 0 {
				t.Errorf("Sent = %q, want the key message rather than raw bytes", sent)
			}
		})
	}
}

// Regression, issue #107: every notch was a full page, so one trackpad flick -
// tens of notches of momentum - jumped tens of pages into the transcript and
// overshot whatever the operator was reading.
func TestUpdate_AFlickOfTheWheelIsNotTensOfPages_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)

	spin(m, 30, tea.MouseWheelDown)

	got := pageKeys(fakes["s1"].Msgs)
	if want := 30 / ui.WheelNotchesPerPage; len(got) != want {
		t.Errorf("30 notches sent %d pages, want %d", len(got), want)
	}
}

// Fewer notches than a page is no page at all, which is what makes the
// accumulator a rate limit rather than a delay.
func TestUpdate_APartPageOfNotchesScrollsNothing_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)

	spin(m, ui.WheelNotchesPerPage-1, tea.MouseWheelDown)

	if got := pageKeys(fakes["s1"].Msgs); len(got) != 0 {
		t.Errorf("a part page sent %v, want nothing", got)
	}
}

// Reversing the wheel must scroll back at once rather than spending notches
// cancelling a part-page the operator cannot see.
func TestUpdate_ReversingTheWheelScrollsBackImmediately_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)

	spin(m, ui.WheelNotchesPerPage-1, tea.MouseWheelDown)
	spin(m, ui.WheelNotchesPerPage, tea.MouseWheelUp)

	got := pageKeys(fakes["s1"].Msgs)
	if len(got) != 1 || got[0] != tea.KeyPgUp {
		t.Errorf("page keys = %v, want exactly [PgUp]", got)
	}
}

// Regression, issue #107: the wheel guarded on "is there a running terminal"
// rather than on "would a keystroke reach it", so a notch over the pane wrote
// PageDown into a live session while the review column had focus - and while
// the note editor was capturing a comment. The dimmed border says a keystroke
// will not land there; the wheel must not contradict it.
func TestUpdate_TheWheelIsIgnoredWhileTheReviewColumnHasFocus_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('d')) // opens the review column with focus

	spin(m, ui.WheelNotchesPerPage*2, tea.MouseWheelDown)

	if got := pageKeys(fakes["s1"].Msgs); len(got) != 0 {
		t.Errorf("the wheel sent %v into the session while the review column had focus, want nothing", got)
	}
}

// The review column carries three scroll offsets and, once omatty asks the
// host for the wheel, no other way to drive them: enabling mouse reporting
// takes away the terminal's own alternate scroll.
func TestUpdate_TheWheelScrollsTheReviewColumn_issue107(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('f')) // the file tree, whose cursor is visible in the frame
	m.Update(ui.FilesLoadedMsg{SessionID: "s1", Paths: []string{
		"internal/ui/model.go", "internal/ui/wheel.go", "README.md",
	}})
	before := m.View().Content

	x, y := overReview()
	m.Update(wheel(x, y, tea.MouseWheelDown))

	if m.View().Content == before {
		t.Errorf("a notch over the review column changed nothing:\n%s", m.View().Content)
	}
}

// The pointer is over the sidebar, not the session; scrolling there must not
// scroll a pane it is not on.
func TestUpdate_TheWheelOutsideThePaneReachesNobody_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	x, y := overSidebar()

	for range ui.WheelNotchesPerPage * 2 {
		m.Update(wheel(x, y, tea.MouseWheelDown))
	}

	if got := fakes["s1"].Msgs; len(pageKeys(got)) != 0 {
		t.Errorf("Msgs = %v for a notch over the sidebar, want no page keys", got)
	}
}

// The trap this fix exists to avoid: Update's default arm ends in broadcast,
// which would hand an untranslated mouse event to every emulator at once.
func TestUpdate_MouseEventsNeverReachAnUnfocusedSession_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)

	spin(m, ui.WheelNotchesPerPage, tea.MouseWheelDown)

	for _, id := range []string{"s2", "s3"} {
		if got := fakes[id].Sent; len(got) != 0 {
			t.Errorf("unfocused %s was sent %q, want nothing", id, got)
		}
		if got := fakes[id].Msgs; len(got) != 0 {
			t.Errorf("unfocused %s got messages %v, want none", id, got)
		}
	}
}

// Clicks and drags belong to #45, which needs sidebar hit-testing. Until then
// they must be dropped rather than broadcast into every PTY.
func TestUpdate_ClicksAreDroppedRatherThanBroadcast_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	x, y := overPane()

	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x, Y: y})

	for id, f := range fakes {
		if len(f.Sent) != 0 || len(f.Msgs) != 0 {
			t.Errorf("%s got Sent=%q Msgs=%v from a click, want nothing", id, f.Sent, f.Msgs)
		}
	}
}

// Nothing arrives at all unless omatty asks the host terminal for it, and
// asking is also what stops alternate scroll turning the wheel into arrows.
func TestView_EnablesMouseReportingSoTheWheelArrives_issue107(t *testing.T) {
	m, _ := modelWithFakes(t)

	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Errorf("View().MouseMode = %v, want MouseModeCellMotion", got)
	}
}

// pgup/pgdn always reached claude - bubbleterm translates them and the router
// forwards them - but nothing said so, which is why the pane looked unable to
// scroll at all. The help modal is where a key that omatty does not own gets
// written down.
//
// The drag modifier is named without committing to one: Ghostty, kitty, xterm
// and Alacritty bypass mouse reporting on shift, Apple Terminal and iTerm2 on
// option, and claiming shift outright was wrong for half of them.
func TestModel_helpNamesTheKeysThatScrollTheSession_issue107(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	press(m, ctrl('o'))
	press(m, key('?'))

	got := m.View().Content
	for _, want := range []string{"pgup / pgdn", "drag"} {
		if !strings.Contains(got, want) {
			t.Errorf("the help modal does not name %q:\n%s", want, got)
		}
	}
}

// Regression, issue #103: the keymap outgrew the pane. On the 20-row window
// AGENTS.md uses for the M4 smoke test, fitBlock cut the last entries off with
// no way to reach them - the same "whatever falls off the end" failure the
// help modal was built to cure, moved from the footer into the modal.
func TestModel_theHelpModalScrollsWhenItDoesNotFit_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)
	// Wide enough that a description is not truncated, short enough that the
	// keymap cannot fit - which is the 20-row window the M4 smoke test uses.
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	press(m, ctrl('o'))
	press(m, key('?'))

	if got := m.View().Content; strings.Contains(got, "scroll the transcript") {
		t.Fatalf("the fixture is too tall to test scrolling; the last section already shows:\n%s", got)
	}
	for range len(ui.LeaderKeys()) + 4 {
		press(m, key('j'))
	}

	if got := m.View().Content; !strings.Contains(got, "scroll the transcript") {
		t.Errorf("scrolling the help modal never reached its last section:\n%s", got)
	}
}

// A modal owns the pane's surface, so there is no transcript under the
// pointer to scroll.
func TestUpdate_TheWheelIsIgnoredBehindAModal_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	spin(m, ui.WheelNotchesPerPage*2, tea.MouseWheelDown)

	if got := fakes["s1"].Sent; len(got) != 0 {
		t.Errorf("Sent = %q with a modal open, want nothing", got)
	}
	if got := pageKeys(fakes["s1"].Msgs); len(got) != 0 {
		t.Errorf("page keys = %v with a modal open, want none", got)
	}
}
