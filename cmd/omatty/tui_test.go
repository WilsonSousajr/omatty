package main

import (
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
)

// Step 5.10 (#653): the status service is told where the hook socket is,
// because where omatty keeps files is infra's knowledge. Unwired, the hook
// server would listen on "" and no hook would ever reach a card.
func TestRuntimeFor_TellsTheStatusServiceTheHookSocket_issue653(t *testing.T) {
	env := tuiEnv{Home: "/h", Agents: mustAgents(t), HooksFiles: map[string]string{"claude": "/h/hooks.json"}, Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	if got, want := runtimeFor(env).Watch.HookSocket, paths.HookSocket("/h"); got != want {
		t.Errorf("Watch.HookSocket = %q, want %q", got, want)
	}
}

// Step 6.10 (#653): the TUI colours code through an injected Highlighter.
// Unwired, every diff and preview would draw uncoloured, silently, so the
// real wiring is pinned: Go source comes back carrying colour.
func TestTuiDeps_HighlightsThroughChroma_issue653(t *testing.T) {
	env := tuiEnv{Home: "/h", Agents: mustAgents(t), HooksFiles: map[string]string{"claude": "/h/hooks.json"}, Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	hl := tuiDeps(env, nil, session.State{}).Highlighter
	if hl == nil {
		t.Fatal("Highlighter is not wired: every diff and preview would draw uncoloured")
	}
	if got := hl.Lines("main.go", []string{"package main"}); len(got) != 1 || !strings.Contains(got[0], "\x1b[") {
		t.Errorf("the wired highlighter returned %q, want coloured Go", got)
	}
}
