package main

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
)

// Step 5.10 (#653): the status service is told where the hook socket is,
// because where omatty keeps files is infra's knowledge. Unwired, the hook
// server would listen on "" and no hook would ever reach a card.
func TestRuntimeFor_TellsTheStatusServiceTheHookSocket_issue653(t *testing.T) {
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	if got, want := runtimeFor(env).Watch.HookSocket, paths.HookSocket("/h"); got != want {
		t.Errorf("Watch.HookSocket = %q, want %q", got, want)
	}
}
