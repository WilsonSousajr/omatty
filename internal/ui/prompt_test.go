package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordCreate is a named fake capturing what the prompt asked for.
type recordCreate struct {
	Project string
	Title   string
	Branch  string
	Calls   int
	Err     error
}

func (r *recordCreate) fn(project, title, branch string) error {
	r.Calls++
	r.Project, r.Title, r.Branch = project, title, branch
	return r.Err
}

func modelWithCreate(t *testing.T, c *recordCreate) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	return ui.NewModel(twoProjectState(), terms, c.fn), fakes
}

func TestModel_leaderNOpensAWorktreePrompt(t *testing.T) {
	m, fakes := modelWithCreate(t, &recordCreate{})

	press(m, ctrl('o'))
	press(m, tea.KeyPressMsg{Code: 'N', Mod: tea.ModShift, Text: "N"})

	p := m.Prompt()
	if !p.Active || !p.Worktree {
		t.Fatalf("Prompt() = %+v, want an active worktree prompt", p)
	}
	for id, f := range fakes {
		if len(f.Msgs) != 0 {
			t.Errorf("terminal %s received %v while a prompt is open, want nothing", id, f.Msgs)
		}
	}
}

func TestModel_leaderNOpensAMainCheckoutPrompt(t *testing.T) {
	m, _ := modelWithCreate(t, &recordCreate{})

	press(m, ctrl('o'))
	press(m, key('n'))

	p := m.Prompt()
	if !p.Active || p.Worktree {
		t.Errorf("Prompt() = %+v, want an active non-worktree prompt", p)
	}
}

func TestModel_promptKeysBuildTheBufferNotThePTY(t *testing.T) {
	m, fakes := modelWithCreate(t, &recordCreate{})

	press(m, ctrl('o'))
	press(m, key('n'))
	for _, r := range "fix" {
		press(m, key(r))
	}

	if got := m.Prompt().Buffer; got != "fix" {
		t.Errorf("Buffer = %q, want %q", got, "fix")
	}
	if len(fakes["s1"].Msgs) != 0 {
		t.Errorf("terminal received %v while a prompt was open, want nothing", fakes["s1"].Msgs)
	}
}

func TestModel_promptBackspaceTrimsTheBuffer(t *testing.T) {
	m, _ := modelWithCreate(t, &recordCreate{})

	press(m, ctrl('o'))
	press(m, key('n'))
	for _, r := range "fix" {
		press(m, key(r))
	}
	press(m, special(tea.KeyBackspace))

	if got := m.Prompt().Buffer; got != "fi" {
		t.Errorf("Buffer = %q, want %q", got, "fi")
	}
}

func TestModel_promptBackspaceOnAnEmptyBufferIsHarmless(t *testing.T) {
	m, _ := modelWithCreate(t, &recordCreate{})

	press(m, ctrl('o'))
	press(m, key('n'))
	press(m, special(tea.KeyBackspace))

	if got := m.Prompt().Buffer; got != "" {
		t.Errorf("Buffer = %q, want empty", got)
	}
}

func TestModel_promptEnterCallsCreateWithBranchForAWorktree(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, tea.KeyPressMsg{Code: 'N', Mod: tea.ModShift, Text: "N"})
	for _, r := range "fix" {
		press(m, key(r))
	}
	press(m, special(tea.KeyEnter))

	if c.Title != "fix" || c.Branch != "fix" {
		t.Errorf("create(%q, %q), want (\"fix\", \"fix\") for a worktree prompt", c.Title, c.Branch)
	}
	if m.Prompt().Active {
		t.Error("Prompt() still active after enter, want closed")
	}
}

func TestModel_promptEnterOnAMainSessionPassesNoBranch(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, key('n'))
	for _, r := range "poke" {
		press(m, key(r))
	}
	press(m, special(tea.KeyEnter))

	if c.Title != "poke" || c.Branch != "" {
		t.Errorf("create(%q, %q), want (\"poke\", \"\")", c.Title, c.Branch)
	}
}

func TestModel_promptEscCancelsWithoutCreating(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, key('n'))
	press(m, special(tea.KeyEscape))

	if c.Calls != 0 {
		t.Errorf("create was called %d times after esc, want 0", c.Calls)
	}
	if m.Prompt().Active {
		t.Error("Prompt() still active after esc, want closed")
	}
}

func TestModel_promptEnterOnAnEmptyBufferDoesNotCreate(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, key('n'))
	press(m, special(tea.KeyEnter))

	if c.Calls != 0 {
		t.Errorf("create was called %d times for an empty title, want 0", c.Calls)
	}
	if !m.Prompt().Active {
		t.Error("Prompt() closed on an empty title, want it to stay open")
	}
}

func TestModel_promptCreateFailureClosesThePromptAndShowsTheError(t *testing.T) {
	c := &recordCreate{Err: errors.New("branch exists")}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o'))
	press(m, tea.KeyPressMsg{Code: 'N', Mod: tea.ModShift, Text: "N"})
	press(m, key('x'))
	press(m, special(tea.KeyEnter))

	if m.Prompt().Active {
		t.Error("Prompt() still active after a failed create, want closed")
	}
	if !strings.Contains(m.View().Content, "branch exists") {
		t.Errorf("View() does not surface the failure:\n%s", m.View().Content)
	}
}

func TestModel_ViewShowsTheOpenPrompt(t *testing.T) {
	m, _ := modelWithCreate(t, &recordCreate{})

	press(m, ctrl('o'))
	press(m, tea.KeyPressMsg{Code: 'N', Mod: tea.ModShift, Text: "N"})
	press(m, key('z'))

	got := m.View().Content
	if !strings.Contains(got, "z") || !strings.Contains(strings.ToLower(got), "branch") {
		t.Errorf("View() does not show the worktree prompt and its buffer:\n%s", got)
	}
}

// Creating a session while the cursor is in api-svc must not silently put it
// in the first registered project.
func TestModel_promptCreatesInTheSelectedProject(t *testing.T) {
	c := &recordCreate{}
	m, _ := modelWithCreate(t, c)

	press(m, ctrl('o')) // move the cursor to s3, which lives in api-svc
	press(m, key('j'))
	press(m, ctrl('o'))
	press(m, key('j'))
	if got := m.SelectedProject(); got != "api-svc" {
		t.Fatalf("SelectedProject() = %q, want api-svc", got)
	}

	press(m, ctrl('o'))
	press(m, key('n'))
	press(m, key('z'))
	press(m, special(tea.KeyEnter))

	if c.Project != "api-svc" {
		t.Errorf("create() got project %q, want api-svc", c.Project)
	}
}
