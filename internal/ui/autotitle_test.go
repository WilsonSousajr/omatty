package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func placeholderState() registry.State {
	st := twoProjectState()
	st.Sessions[0].Title = registry.PlaceholderTitle("s1") // a session created untitled
	return st
}

func modelWithNamer(t *testing.T, st registry.State) (*ui.Model, *FakeNamer, *FakeRename) {
	t.Helper()
	terms, _ := fakeTerms(t)
	namer := &FakeNamer{Titles: map[string]string{"s1": "fix the wheel"}}
	ren := &FakeRename{}
	d := baseDeps(st, terms)
	d.Name, d.Rename = namer.Name, ren.Rename
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m, namer, ren
}

// statusDeliver sends one status event and feeds every message its commands
// produce back into the model, so a name read off the event loop lands.
func statusDeliver(m *ui.Model, id string, k watcher.Kind, at time.Time) {
	_, cmd := m.Update(ui.StatusMsg{SessionID: id, Kind: k, At: at})
	deliver(m, cmd)
}

func TestModel_APlaceholderSessionTakesItsFirstPrompt_issue127(t *testing.T) {
	m, namer, ren := modelWithNamer(t, placeholderState())

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if len(namer.Asked) != 1 || ren.Titles["s1"] != "fix the wheel" {
		t.Fatalf("asked %v, persisted %v; want s1 named fix the wheel", namer.Asked, ren.Titles)
	}
	if !strings.Contains(m.View().Content, "fix the wheel") {
		t.Error("the sidebar does not show the new title")
	}
}

// Invariant 9: a session created untitled, prompted, and relaunched has an
// event older than this run and a kind that is not PromptSubmitted. It is
// still named. Fails if naming is gated on PromptSubmitted or on startedAt.
func TestModel_AnUntitledSessionIsNamedAfterARelaunch_issue127(t *testing.T) {
	m, _, ren := modelWithNamer(t, placeholderState())

	statusDeliver(m, "s1", watcher.TurnEnded, time.Now().Add(-time.Hour))

	if ren.Titles["s1"] != "fix the wheel" {
		t.Errorf("persisted %v, want s1 named from a replayed TurnEnded", ren.Titles)
	}
}

func TestModel_ATypedTitleIsNeverOverwritten_issue127(t *testing.T) {
	m, namer, _ := modelWithNamer(t, twoProjectState()) // s1 is "main"

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if len(namer.Asked) != 0 {
		t.Errorf("a titled session was asked for a name: %v", namer.Asked)
	}
}

func TestModel_ARenameDuringTheReadWins_issue127(t *testing.T) {
	m, _, ren := modelWithNamer(t, placeholderState())
	_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: time.Now()})
	// The read is in flight; the operator renames first.
	leader(m, shift('r', "R"))
	for range len(registry.PlaceholderTitle("s1")) {
		press(m, special(tea.KeyBackspace))
	}
	for _, r := range "mine" {
		press(m, key(r))
	}
	pressAndSettle(m, special(tea.KeyEnter))

	deliver(m, cmd) // now the NamedMsg lands

	if ren.Titles["s1"] != "mine" {
		t.Errorf("persisted %q, want the operator's rename to win", ren.Titles["s1"])
	}
}

func TestModel_OnlyOneNameReadIsInFlightPerSession_issue127(t *testing.T) {
	m, namer, _ := modelWithNamer(t, placeholderState())
	var cmds []tea.Cmd
	for range 3 {
		_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: time.Now()})
		cmds = append(cmds, cmd)
	}
	for _, cmd := range cmds {
		deliver(m, cmd)
	}
	if len(namer.Asked) != 1 {
		t.Errorf("namer asked %d times for three events with one read in flight, want 1", len(namer.Asked))
	}
}

func TestModel_AnEmptyTitleLeavesThePlaceholderAndRetriesLater_issue127(t *testing.T) {
	m, namer, ren := modelWithNamer(t, placeholderState())
	namer.Titles["s1"] = ""
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	if ren.Calls != 0 {
		t.Fatalf("an empty title was persisted: %v", ren.Titles)
	}

	namer.Titles["s1"] = "fix the wheel"
	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if len(namer.Asked) != 2 || ren.Titles["s1"] != "fix the wheel" {
		t.Errorf("asked %v persisted %v; want a retry that names s1", namer.Asked, ren.Titles)
	}
}

func TestModel_NamingNeverBlocksUpdate_issue127(t *testing.T) {
	m, namer, _ := modelWithNamer(t, placeholderState())
	namer.Block = make(chan struct{})
	done := make(chan struct{})
	go func() {
		m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: time.Now()})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Update blocked on the transcript read")
	}
	close(namer.Block)
}

// A failed read is a log line, never a footer error: an auto-name is not
// something the operator did, so it must not take the footer from them.
func TestModel_AFailedNameLeavesTheRowAlone_issue127(t *testing.T) {
	m, namer, ren := modelWithNamer(t, placeholderState())
	namer.Err = errors.New("disk")

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if ren.Calls != 0 || strings.Contains(m.View().Content, "error:") {
		t.Errorf("a failed read persisted %d titles or put an error in the footer", ren.Calls)
	}
}

// DeriveKind reports PromptSubmitted for a tool result too. The namer reads
// the transcript, so a trigger that is not a prompt costs one read and
// answers "" - documented here so nobody narrows the trigger back.
func TestModel_AToolResultDoesNotInventATitle_issue127(t *testing.T) {
	m, namer, ren := modelWithNamer(t, placeholderState())
	namer.Titles["s1"] = ""

	statusDeliver(m, "s1", watcher.PromptSubmitted, time.Now())

	if ren.Calls != 0 || len(namer.Asked) != 1 {
		t.Errorf("persisted %d titles after %d reads; want none and one", ren.Calls, len(namer.Asked))
	}
}
