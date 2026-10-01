// The TUI's lifecycle: everything that lives as long as the program - the
// terminals, the status service, the gate Runner - built and closed here,
// in the one composition root (ADR 0001; migration step 5.10, #653). Until
// then app.Run was a second composition root, and RunDeps a second copy of
// most of app.Deps that every new dependency had to be plumbed through twice.

package main

import (
	"context"
	"time"

	"github.com/WilsonSousajr/omatty/internal/infra/gateexec"
	"github.com/WilsonSousajr/omatty/internal/infra/notify"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/gate"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// tuiRuntime is what the TUI's lifecycle needs beyond app.Deps: how to start
// a session's terminal, what the status service reads, and how gates run.
type tuiRuntime struct {
	Launch       *sessions.Launcher
	Factory      terminal.Factory
	Watch        status.WatchDeps
	GateParallel int
	RunGate      gate.RunFunc
	LazyStart    bool
	Width        int
	Height       int
}

// runtimeFor builds the lifecycle's half of the wiring from env.
func runtimeFor(env tuiEnv) tuiRuntime {
	return tuiRuntime{
		Launch:  sessions.NewLauncher(env.Agents.WithBins(map[string]string{"claude": env.Cfg.ClaudeBin}).WithHooksFiles(env.HooksFiles), env.Home, env.Holder),
		Factory: terminal.Start,
		Watch: status.WatchDeps{Home: env.Home, HookSocket: paths.HookSocket(env.Home), Clock: time.Now, OpenTranscript: openTranscript, ListenHooks: listenHooks,
			Agents: env.Agents},
		// The gate's bound comes from the config; the Runner raises a zero to
		// one, so an old config file without a [gate] section still works.
		GateParallel: env.Cfg.Gate.MaxParallel, RunGate: gateexec.Run,
		LazyStart: env.Cfg.Sessions.LazyStart, Width: env.Width, Height: env.Height,
	}
}

// runProgram starts every wanted session's terminal, the status service and
// the gate Runner, hands the model what they produce, and runs it until the
// operator quits; then closes all three.
func runProgram(deps app.Deps, rt tuiRuntime) error {
	deps.Leader = app.LeaderOr(deps.Leader)
	// Asked before the terminals start: once a client is attached the socket
	// exists whether or not a claude was already behind it (#191).
	held := app.HeldSessions(rt.Launch, deps.State)
	terms := app.StartTerminals(deps.State, app.SessionsToStart(rt.LazyStart, deps.State, held), rt.Launch, rt.Factory, rt.Width, rt.Height, deps.Leader)
	defer app.CloseTerminals(terms)
	watch := status.Start(rt.Watch, deps.State.Sessions)
	defer watch.Close()
	// The model's subscriptions live as long as the program: cancelled on
	// return, so the brokers let go of them (#653).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// One Runner for the whole app, bounded: four concurrent `go test -race`
	// would make the machine unusable (#229).
	gates := gate.NewRunner(rt.GateParallel, rt.RunGate)
	defer gates.Close()
	deps.Terms, deps.Reattached, deps.Start = terms, held, app.GuardedStarter(rt.Launch, rt.Factory, deps.Leader)
	deps.Events, deps.HooksDown, deps.TailStart, deps.TailStop = watch.Subscribe(ctx), !watch.HooksLive(), watch.Add, watch.Remove
	deps.GateReports, deps.GateRun = gates.Subscribe(ctx), gates.Start
	deps.Clock, deps.Notifier = time.Now, notify.New()
	return app.RunProgram(app.NewModel(deps), len(terms))
}
