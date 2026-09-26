package ui_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// modelAt is a model whose clock reads *now, so a test can step time between
// two renders of the same card.
func modelAt(t *testing.T, now *time.Time, terms map[string]termwrap.Terminal) *ui.Model {
	t.Helper()
	d := baseDeps(twoProjectState(), terms)
	d.Clock = func() time.Time { return *now }
	return ui.NewModel(d)
}

// glyphOf is the status glyph a session's card draws: the cell after the rail.
func glyphOf(t *testing.T, m *ui.Model, id string) string {
	t.Helper()
	card := m.CardOf(id)
	if len(card) == 0 {
		t.Fatalf("no card for %s", id)
	}
	return string([]rune(stripSGR(card[0]))[1])
}

func TestSpinnerFrame_StepsEverySpinEveryAndWraps_issue410(t *testing.T) {
	frames := ui.SpinFrames()
	for i := range 2 * len(frames) {
		at := fixedNow.Add(time.Duration(i) * ui.SpinEvery())
		if got, want := ui.SpinFrameAt(at), frames[i%len(frames)]; got != want {
			t.Errorf("frame %d = %q, want %q", i, got, want)
		}
	}
}

// A clock before 1970 has a negative UnixMilli, and Go's % keeps the sign,
// so the frame index went negative and indexing the frames panicked (found
// writing #410). The zero time.Time happens to land on frame 0, so the probe
// is a tenth of a second before the epoch.
func TestSpinnerFrame_AClockBefore1970DoesNotPanic_issue410(t *testing.T) {
	if got := ui.SpinFrameAt(time.UnixMilli(-100)); got != ui.SpinFrames()[len(ui.SpinFrames())-1] {
		t.Errorf("frame a step before the epoch = %q, want the last frame", got)
	}
}

// Measured the way TestStatusGlyphs_AreOneCellWide_issue128 measures the
// glyphs, and under the East Asian width rule too: braille is not Ambiguous,
// so unlike ◐ it must stay one cell even with RUNEWIDTH_EASTASIAN=1.
func TestSpinFrames_AreOneCellWide_issue410(t *testing.T) {
	wide := runewidth.NewCondition()
	wide.EastAsianWidth = true
	for _, f := range ui.SpinFrames() {
		if lipgloss.Width(f) != 1 || runewidth.StringWidth(f) != 1 || wide.StringWidth(f) != 1 {
			t.Errorf("frame %q is not one cell wide", f)
		}
	}
}

func TestCard_AWorkingSessionSpins_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	m := modelAt(t, &now, terms)
	status(m, "s2", watcher.PromptSubmitted, fixedNow)

	first := glyphOf(t, m, "s2")
	now = now.Add(ui.SpinEvery())
	second := glyphOf(t, m, "s2")

	if !slices.Contains(ui.SpinFrames(), first) || !slices.Contains(ui.SpinFrames(), second) || first == second {
		t.Errorf("glyphs %q then %q one step apart, want two different spinner frames", first, second)
	}
}

// Thinking and tool are both "busy, leave it": one spinner, not two.
func TestCard_ThinkingAndToolSpinAlike_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	m := modelAt(t, &now, terms)
	status(m, "s1", watcher.PromptSubmitted, fixedNow)
	status(m, "s2", watcher.ToolStarted, fixedNow)

	if a, b := glyphOf(t, m, "s1"), glyphOf(t, m, "s2"); a != b || !slices.Contains(ui.SpinFrames(), a) {
		t.Errorf("thinking draws %q and tool %q, want the same spinner frame", a, b)
	}
}

func TestCard_ASessionAtRestKeepsItsGlyph_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	m := modelAt(t, &now, terms)
	status(m, "s1", watcher.PermissionRequested, fixedNow)
	status(m, "s2", watcher.TurnEnded, fixedNow)

	if got := glyphOf(t, m, "s1"); got != "●" {
		t.Errorf("waiting draws %q, want ●", got)
	}
	if got := glyphOf(t, m, "s2"); got != "✓" {
		t.Errorf("done draws %q, want ✓", got)
	}
}

// A card keeps its status after ctrl+o s (#318), so a session stopped
// mid-turn still reads "thinking". Spinning it would claim work that no
// process is doing; it keeps the still glyph instead.
func TestCard_ASessionWithNoProcessDoesNotSpin_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	delete(terms, "s2")
	m := modelAt(t, &now, terms)
	status(m, "s2", watcher.PromptSubmitted, fixedNow)

	if got := glyphOf(t, m, "s2"); got != "◐" {
		t.Errorf("a working session with no process draws %q, want the still ◐", got)
	}
}

func TestHeader_TheStateSpinsWhileWorking_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	m := modelAt(t, &now, terms)
	status(m, "s1", watcher.PromptSubmitted, fixedNow)

	got := stripSGR(m.View().Content)
	if !strings.Contains(got, ui.SpinFrameAt(now)+" thinking") {
		t.Errorf("the header does not show %q:\n%s", ui.SpinFrameAt(now)+" thinking", got)
	}
}

// The screen redraws at spinner speed only while something spins: an idle
// omatty must tick exactly as it did before (M13). #412 moved the fast frames
// from the heartbeat to a spin tick of their own, so this asserts that the
// spin tick is armed while a session works, re-arms on each step, and stops
// once the turn ends.
func TestTick_RunsAtSpinnerSpeedOnlyWhileASessionSpins_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	m := modelAt(t, &now, terms)
	m.Update(ui.TickMsg(fixedNow))
	if m.SpinArmed() {
		t.Fatal("idle, a spin tick is armed; omatty would redraw ten times a second for nothing")
	}
	status(m, "s1", watcher.PromptSubmitted, fixedNow)
	m.Update(ui.SpinTickMsg(fixedNow))
	if !m.SpinArmed() {
		t.Error("a spin step while a session works did not re-arm the spin tick")
	}
	status(m, "s1", watcher.TurnEnded, fixedNow.Add(time.Second))
	m.Update(ui.SpinTickMsg(fixedNow))
	if m.SpinArmed() {
		t.Error("the spin tick re-armed after the turn ended")
	}
}

// A working session with no process does not spin, so it must not hold the
// fast tick either.
func TestTick_AStoppedWorkingSessionDoesNotHoldTheFastTick_issue410(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	delete(terms, "s2")
	m := modelAt(t, &now, terms)
	status(m, "s2", watcher.PromptSubmitted, fixedNow)

	if m.SpinArmed() {
		t.Error("a stopped working session armed the spin tick")
	}
}

// The lane's six columns went back to the branch: a 23-column branch on a
// clean tree is drawn whole.
func TestCard_LineTwoGivesTheBranchTwentyThreeColumns_issue410(t *testing.T) {
	m, _ := modelWithStat(t)
	branch := "feat/410-working-spinne"
	m.SetRepoStat("s1", review.Stat{Branch: branch})

	if got := stripSGR(m.CardOf("s1")[1]); !strings.Contains(got, branch) {
		t.Errorf("line two %q does not hold the %d-column branch %q", got, len(branch), branch)
	}
}

// Regression, #412: #410 sped the heartbeat up only when it scheduled the
// next tick, so a session that started working waited out the one-second
// tick already pending - a still glyph for up to a second - before it
// turned. The message that makes a session spin must start the spin itself.
func TestSpin_TheStatusThatStartsWorkStartsTheSpin_issue412(t *testing.T) {
	now := fixedNow
	terms, _ := fakeTerms(t)
	m := modelAt(t, &now, terms)

	status(m, "s1", watcher.PromptSubmitted, fixedNow)

	if !m.SpinArmed() {
		t.Error("a session started working and no spin tick was armed; it waits for the heartbeat")
	}
}

// The other way a session starts spinning is without a status at all: a
// session stopped mid-turn keeps "thinking" (#318) and does not spin, and
// resuming it gives it a process. The spin starts on that keypress too, not
// on the next heartbeat (#412).
func TestSpin_ResumingASessionMidTurnStartsTheSpin_issue412(t *testing.T) {
	r := newStopRig(t)
	status(r.m, "s1", watcher.PromptSubmitted, fixedNow)
	r.stop()
	r.m.Update(ui.SpinTickMsg(fixedNow))
	if r.m.SpinArmed() {
		t.Fatal("a stopped session still holds the spin tick")
	}

	pressAndSettle(r.m, special(tea.KeyEnter))

	if !r.m.SpinArmed() {
		t.Error("resuming a session mid-turn armed no spin tick; it waits for the heartbeat")
	}
}
