package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/gate"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/notify"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// The capabilities #526's degradations key off, one agent per shape.
var (
	processCaps    = agent.Caps{}
	noTurnCaps     = agent.Caps{Identity: agent.Assigned, Status: agent.StatusTranscript, Resume: true}
	noResumeCaps   = agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks, Waiting: true, TurnBoundary: true}
	capsByAgentFor = map[string]agent.Caps{"shell": processCaps, "toy": noTurnCaps, "fresh": noResumeCaps}
)

// fakeCaps stands in for cmd's catalog lookup: each agent's capabilities.
func fakeCaps(name string) (agent.Caps, bool) {
	c, ok := capsByAgentFor[name]
	return c, ok
}

// tierState is one project with one session, s1, running agentName.
func tierState(agentName string) session.State {
	return session.State{
		Projects: []session.Project{{Name: "omatty", Root: "/p/omatty", Gate: []gate.Step{{Name: "test", Run: "go test ./..."}}}},
		Sessions: []session.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: "/p/omatty", Agent: agentName}},
	}
}

func tierModel(t *testing.T, st session.State, terms map[string]terminal.Terminal, edit func(*app.Deps)) *app.Model {
	t.Helper()
	d := baseDeps(st, terms)
	d.AgentCaps = fakeCaps
	if edit != nil {
		edit(&d)
	}
	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// A Process-tier agent cannot report busy, so its card must never claim idle:
// it shows running while its process lives and exited once it does not
// (#526).
func TestCard_AProcessSessionNeverLooksIdle_issue526(t *testing.T) {
	st := tierState("shell")
	running := tierModel(t, st, fakeTermsFor(st), nil).CardOf("s1")[0]
	if strings.Contains(running, "○") || !strings.Contains(running, "▷") {
		t.Errorf("running card %q, want the running mark and no idle one", running)
	}
	exited := tierModel(t, st, map[string]terminal.Terminal{}, nil).CardOf("s1")[0]
	if strings.Contains(exited, "○") || !strings.Contains(exited, "∅") {
		t.Errorf("exited card %q, want the exited mark and no idle one", exited)
	}
}

// The header names the agent and its tier, and shows no meter where the
// adapter reports no usage (#526).
func TestHeader_AProcessSessionShowsItsAgentAndNoMeter_issue526(t *testing.T) {
	st := tierState("shell")
	header := strings.Split(tierModel(t, st, fakeTermsFor(st), nil).View().Content, "\n")[0]
	if !strings.Contains(header, "shell · process") || !strings.Contains(header, "running") {
		t.Errorf("header %q, want the agent, its tier and running", header)
	}
	if strings.Contains(header, "cached") || strings.Contains(header, " in / ") {
		t.Errorf("header %q shows a meter for an agent that reports no usage", header)
	}
}

// claude's header is as it was: naming the agent on every card is noise
// when there is only one (#526).
func TestHeader_ClaudeNamesNoAgent_issue526(t *testing.T) {
	st := tierState("")
	header := strings.Split(tierModel(t, st, fakeTermsFor(st), nil).View().Content, "\n")[0]
	if strings.Contains(header, "claude") || strings.Contains(header, "full") {
		t.Errorf("header %q names claude's tier, want it as before", header)
	}
}

// Without a turn boundary a quiet session is not a finished turn, so the
// gate does not auto-run on it - though it still runs on demand (#526).
func TestAutoGate_NoTurnBoundaryRunsNoGate_issue526(t *testing.T) {
	rec := &recordGateRun{}
	st := tierState("toy")
	m := tierModel(t, st, fakeTermsFor(st), func(d *app.Deps) { d.GateRun, d.GateAuto = rec.Run, true })

	sendStatus(m, "s1", dstatus.TurnEnded, time.Now())

	if len(rec.IDs) != 0 {
		t.Errorf("auto-ran %v for an agent that reports no turn's end, want none", rec.IDs)
	}
}

// A turn-end notification needs a turn end; "needs you" still goes (#526).
func TestNotify_NoTurnEndNoticeWithoutTurnBoundary_issue526(t *testing.T) {
	n := &notify.Fake{}
	st := tierState("toy")
	m := tierModel(t, st, fakeTermsFor(st), func(d *app.Deps) {
		d.Notifier = n
		d.Clock = func() time.Time { return fixedNow }
	})
	m.Update(tea.BlurMsg{})

	sendStatus(m, "s1", dstatus.TurnEnded, fixedNow)

	if len(n.Sent) != 0 {
		t.Errorf("sent %+v, want no turn-end notice for an agent with no turn boundary", n.Sent)
	}
}

// t is refused with the reason, naming the agent, and the diff stays on the
// whole session (#526).
func TestScope_RefusedWithoutTurnBoundary_issue526(t *testing.T) {
	st := tierState("toy")
	m := tierModel(t, st, fakeTermsFor(st), func(d *app.Deps) { d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn })
	leader(m, key('d'))

	pressAndSettle(m, key('t'))

	view := m.View().Content
	if !strings.Contains(view, "toy") || !strings.Contains(view, "turn") {
		t.Errorf("no reason naming toy and the turn:\n%s", view)
	}
	if strings.Contains(view, "this turn") {
		t.Errorf("the diff switched to this turn for an agent with no turn boundary:\n%s", view)
	}
}

// Without resume, a stopped session offers to start fresh and says the
// conversation is lost; it never offers a resume that cannot happen (#526).
func TestStopped_NoResumeOffersAFreshStart_issue526(t *testing.T) {
	view := tierModel(t, tierState("fresh"), map[string]terminal.Terminal{}, nil).View().Content
	if strings.Contains(view, "resumes it") {
		t.Errorf("offers a resume fresh cannot do:\n%s", view)
	}
	if !strings.Contains(view, "starts it fresh") || !strings.Contains(view, "conversation is lost") {
		t.Errorf("does not say it starts fresh and loses the conversation:\n%s", view)
	}
}
