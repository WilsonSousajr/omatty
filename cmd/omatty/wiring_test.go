package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/gate"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"net"
)

// A wiring test of the kind main_test.go concedes is missing: the configured
// binary reaches the launcher (#44).
// The config's lazy_start reaches the boot, in both directions (#317).
func TestTuiDeps_PassesLazyStart_issue317(t *testing.T) {
	for _, lazy := range []bool{true, false} {
		env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
		env.Cfg = config.Defaults("/h")
		env.Cfg.Sessions.LazyStart = lazy

		if got := runtimeFor(env).LazyStart; got != lazy {
			t.Errorf("config lazy_start = %v reached the boot as %v", lazy, got)
		}
	}
}

// The config's idle_stop reaches the model's sweep (#319).
func TestTuiDeps_PassesIdleStop_issue319(t *testing.T) {
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	env.Cfg.Sessions.IdleStop = config.Duration(90 * time.Minute)

	if got := tuiDeps(env, nil, sessions.State{}).IdleStop; got != 90*time.Minute {
		t.Errorf("config idle_stop = 90m reached the sweep as %v", got)
	}
}

func TestTuiDeps_PassesTheConfiguredClaudeBinToTheLauncher_issue44(t *testing.T) {
	// A short home: tuiDeps touches no file, but with dtach installed the
	// launcher derives a socket path from it and refuses one over 103 bytes,
	// which t.TempDir() exceeds on macOS (#43).
	home := "/h"
	env := tuiEnv{Home: home, Agent: claudeProfile(), HooksFile: filepath.Join(home, "hooks.json"), Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults(home)
	env.Cfg.ClaudeBin = "/opt/claude"
	env.Cfg.Leader = "ctrl+a"

	deps := tuiDeps(env, nil, sessions.State{})

	launch, err := runtimeFor(env).Launch.Launch(sessions.Session{ID: "id", Dir: home})
	if err != nil {
		t.Fatal(err)
	}
	if launch.Argv[0] != "/opt/claude" {
		t.Errorf("launcher runs %q, want /opt/claude", launch.Argv[0])
	}
	if deps.Leader != "ctrl+a" {
		t.Errorf("Deps.Leader = %q, want the configured ctrl+a", deps.Leader)
	}
}

// The creator forks from the configured base and places worktrees under the
// configured root, for the TUI and the CLI alike (#44).
func TestCreatorOpts_ComeFromTheConfig_issue44(t *testing.T) {
	cfg := config.Config{WorktreeRoot: "/vol/wt", BaseBranch: "develop"}

	got := creatorOpts(cfg)

	if got.WorktreeRoot != "/vol/wt" || got.BaseBranch != "develop" {
		t.Errorf("creatorOpts() = %+v, want /vol/wt forked from develop", got)
	}
	// Step 5.4 (#653): the path function is injected; unset, every worktree
	// session would be refused.
	if got.WorktreeDir == nil || got.WorktreeDir(got.WorktreeRoot, "p", "b") != "/vol/wt/p/b" {
		t.Error("creatorOpts() does not wire paths.WorktreeDir")
	}
}

// Step 5.4 (#653): the carry copy is injected, because copying files is
// infra's business. Both creators - the TUI's and `omatty new` - take their
// options from creatorOpts, so the real wiring is pinned there: the wired
// copier copies a real file.
func TestCreatorOpts_CarryThroughTheStore_issue653(t *testing.T) {
	root, dir := t.TempDir(), t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".env"), "TOKEN=shh")

	carry := creatorOpts(config.Config{}).Carry
	if carry == nil {
		t.Fatal("Carry is not wired: every project with a carry list would fail to create a worktree")
	}
	if err := carry(dir, root, []string{".env"}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".env")); err != nil || string(b) != "TOKEN=shh" {
		t.Errorf("the wired copier left %q, %v; want the carried file", b, err)
	}
}

// One Router answers every forge call the TUI makes (#452): the lists, an
// item, the browser pair, the label and #331's ship calls. The label is the
// Router's own - neutral for a project it has not resolved - and not ui's
// unwired default, which is GitHub's.
func TestTuiDeps_WiresEveryForgeCallThroughTheRouter_issue452(t *testing.T) {
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")

	deps := tuiDeps(env, nil, sessions.State{})

	for name, missing := range map[string]bool{
		"PRs": deps.PRs == nil, "Issues": deps.Issues == nil,
		"Item.Issue": deps.Item.Issue == nil, "Item.PR": deps.Item.PR == nil,
		"Browse.Issue": deps.Browse.Issue == nil, "Browse.PR": deps.Browse.PR == nil,
		"Label": deps.Label == nil, "Ship.CreatePR": deps.Ship.CreatePR == nil,
		"Ship.MergePR": deps.Ship.MergePR == nil, "Ship.BranchProtected": deps.Ship.BranchProtected == nil,
	} {
		if missing {
			t.Errorf("%s is not wired", name)
		}
	}
	if got := deps.Label("/nowhere"); got != forge.Neutral {
		t.Errorf("Label(unresolved) = %+v, want the Router's neutral label", got)
	}
}

// Regression, issue #91: projectRegistrar discarded the Project AddProject
// wrote, so the TUI rebuilt one from the picked row instead. Discovery names a
// candidate after MainCheckout's directory and AddProject after RepoRoot's, so
// where those disagree the sidebar held a name state.json did not.
func TestProjectRegistrar_ReturnsTheProjectTheRegistryWrote_issue91(t *testing.T) {
	store := storeIn(t)
	// A worktree whose main checkout is elsewhere: the two names differ.
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}

	got := projectRegistrar(store, git)([]string{"/p/omatty"})

	if len(got) != 1 {
		t.Fatalf("registrar returned %d registrations, want 1", len(got))
	}
	if got[0].Err != nil {
		t.Fatalf("registering /p/omatty: %v", got[0].Err)
	}
	if got[0].Project.Name != "omatty" || got[0].Project.Root != "/p/omatty" {
		t.Errorf("Project = %+v, want the row the registry wrote", got[0].Project)
	}
}

// A collision is reported against the one root it belongs to, and the rest
// still register: one bad candidate must not abandon a bulk pick (#91).
func TestProjectRegistrar_ReportsACollisionAndCarriesOn_issue91(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{
		"/p/omatty": "/p/omatty", "/other/omatty": "/other/omatty", "/p/notes": "/p/notes",
	}}
	registrar := projectRegistrar(store, git)
	registrar([]string{"/p/omatty"})

	got := registrar([]string{"/other/omatty", "/p/notes"})

	if len(got) != 2 {
		t.Fatalf("registrar returned %d registrations, want 2", len(got))
	}
	if got[0].Err == nil {
		t.Errorf("registering a duplicate name reported no error: %+v", got[0])
	}
	if got[1].Err != nil {
		t.Errorf("the second root was abandoned after the first failed: %v", got[1].Err)
	}
}

// The archiver returns the row the registry removed, which is what decides
// whether a worktree may be deleted (#40).
func TestSessionArchiver_ReturnsTheRemovedSession_issue40(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}
	if _, err := sessions.AddProject(t.Context(), store, git, "/p/omatty"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	st, err := store.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	st.Sessions = append(st.Sessions, sessions.Session{
		ID: "s1", Project: "omatty", Title: "main", Dir: "/wt/omatty/fix", Worktree: true,
	})
	if err := store.Save(t.Context(), st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := sessionArchiver(store)("s1")

	if err != nil {
		t.Fatalf("archiving s1: %v", err)
	}
	if !got.Worktree || got.Dir != "/wt/omatty/fix" {
		t.Errorf("removed session = %+v, want the row with its worktree fields", got)
	}
}

func TestSessionRenamer_RefusesABlankTitle_issue41(t *testing.T) {
	store := storeIn(t)
	rename := sessionRenamer(store)

	if err := rename("s1", "   "); err == nil {
		t.Error("renaming to a whitespace-only title succeeded, want an error")
	}
}

// noAddProject-style check on the proposer: a store it cannot read is an error
// the picker surfaces, not an empty list that reads as "claude has never run".
func TestProjectProposer_SurfacesAnUnreadableStore_issue91(t *testing.T) {
	proposals, err := projectProposer(storeIn(t), t.TempDir(), &FakeGit{})()

	if err == nil {
		t.Errorf("proposer returned %v and no error for a store with no transcripts dir", proposals)
	}
}

// The wiring took the concrete *vcs.CLI, so not one of these adapters could be
// built in a test at all - the untestability sessions.RepoRooter's own doc
// records as the #91 defect, restated for the pickers (#122). This is the test
// that could not be written before, and it is the whole point of narrowing the
// parameter: every picker dependency is now reachable without a repository.
func TestWithPickerDeps_BuildsEveryPickerDependency_issue122(t *testing.T) {
	g := &FakeGit{}
	deps := withPickerDeps(ui.Deps{}, storeIn(t), t.TempDir(), g, g)

	for name, built := range map[string]bool{
		"Discover":     deps.Discover != nil,
		"AddProject":   deps.AddProject != nil,
		"AdoptPropose": deps.AdoptPropose != nil,
		"AdoptCommit":  deps.AdoptCommit != nil,
	} {
		if !built {
			t.Errorf("withPickerDeps left %s unset; the picker key would report missing wiring", name)
		}
	}
}

// sessionAdopter is the seam between the picker and the registry, and it has to
// hand back the row that was written: the branch is filled in there and nowhere
// else, so a picker fed the pick it sent would start a session with the wrong
// diff base (#122).
func TestSessionAdopter_ReturnsTheRowTheRegistryWrote_issue122(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}, Branch: "fix/parser"}
	if _, err := sessions.AddProject(t.Context(), store, git, "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	got := sessionAdopter(store, git)("omatty", []ui.SessionProposal{
		{ID: "abc-123", Title: "fix the parser", Dir: "/p/omatty/.omatty/wt/fix"},
	})

	if len(got) != 1 {
		t.Fatalf("adopted %d sessions, want 1", len(got))
	}
	if got[0].Err != nil {
		t.Fatalf("Err = %v, want nil", got[0].Err)
	}
	if got[0].Session.Branch != "fix/parser" {
		t.Errorf("Branch = %q, want the branch the registry recorded for the worktree", got[0].Session.Branch)
	}
}

// The remover returns the row the registry dropped, so the TUI can name it.
func TestProjectRemover_ReturnsTheRemovedProject_issue159(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}
	if _, err := sessions.AddProject(t.Context(), store, git, "/p/omatty"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	got, err := projectRemover(store)("omatty")

	if err != nil || got.Name != "omatty" {
		t.Errorf("projectRemover = %+v, %v; want omatty removed", got, err)
	}
}

// The folder writes the fold to state.json, so a folded project is still
// folded on the next launch (#505).
func TestProjectFolder_PersistsTheFold_issue505(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}
	if _, err := sessions.AddProject(t.Context(), store, git, "/p/omatty"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	if err := projectFolder(store)("omatty", true); err != nil {
		t.Fatalf("projectFolder: %v", err)
	}

	st, err := store.Load(t.Context())
	if err != nil || !st.Projects[0].Collapsed {
		t.Errorf("after folding, Load() = %+v, %v; want omatty collapsed", st.Projects, err)
	}
}

// Regression, issue #316: the TUI's rebind reaches state.json, which is what
// the next start resumes from.
func TestSessionRebinder_PersistsTheConversation_issue316(t *testing.T) {
	store := storeIn(t)
	st := sessions.State{Version: sessions.Version,
		Projects: []sessions.Project{{Name: "omatty", Root: "/p/omatty"}},
		Sessions: []sessions.Session{{ID: "s1", Project: "omatty", Title: "main", Dir: "/p/omatty"}}}
	if err := store.Save(t.Context(), st); err != nil {
		t.Fatal(err)
	}

	if err := sessionRebinder(store)("s1", "after-clear"); err != nil {
		t.Fatalf("rebind error = %v, want nil", err)
	}

	got, _ := store.Load(t.Context())
	if got.Sessions[0].Conversation != "after-clear" {
		t.Errorf("persisted row = %+v, want conversation after-clear", got.Sessions[0])
	}
}

// Regression, issue #564: the namer read the transcript at the directory as
// registered, but claude writes it under the resolved one, so a project
// behind a symlink was never named from its first prompt.
func TestSessionNamer_ReadsATranscriptBehindASymlink_issue564(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "real", "omatty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(filepath.Join(root, "real", "omatty"))
	if err != nil {
		t.Fatal(err)
	}
	adoptFixture(t, home, physical, "abc", "fix the parser")

	title, err := sessionNamer(home, claudeProfile())(sessions.Session{ID: "abc", Dir: filepath.Join(root, "link", "omatty")})

	if err != nil || !strings.Contains(title, "parser") {
		t.Errorf("sessionNamer = (%q, %v), want a title from the transcript under the resolved directory", title, err)
	}
}

// A cleared session is named from the conversation it is on, not from the
// transcript its row was created with (#316).
func TestSessionNamer_ReadsTheReboundConversation_issue316(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "omatty")
	adoptFixture(t, home, dir, "before-clear", "the old work")
	adoptFixture(t, home, dir, "after-clear", "fix the parser")

	title, err := sessionNamer(home, claudeProfile())(sessions.Session{ID: "before-clear", Dir: dir, Conversation: "after-clear"})

	if err != nil || !strings.Contains(title, "parser") {
		t.Errorf("sessionNamer = (%q, %v), want a title from the post-clear prompt", title, err)
	}
}

// Step 5.2c (#653): the watcher reads transcripts through an injected opener,
// because reading files is infra's business. Left unset, every session's
// tailer would call a nil function at startup - which no watcher test with a
// fake opener could notice - so the wiring itself is pinned here.
func TestTuiDeps_OpensTranscriptsThroughTheReader_issue653(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, []byte("line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	if runtimeFor(env).Watch.OpenTranscript == nil {
		t.Fatal("OpenTranscript is not wired: every tailer would call a nil opener")
	}
	lines, _, ok := runtimeFor(env).Watch.OpenTranscript(path).Poll()
	if !ok || len(lines) != 1 || string(lines[0]) != "line" {
		t.Errorf("the wired opener read %q, %v; want [line], true", lines, ok)
	}
}

// Step 5.2d (#653): the hook socket is served by an injected server, because
// running one is infra's business. Left unset the watcher would call a nil
// function at start, so the real wiring is pinned: a payload sent to the
// socket the wired server listens on arrives on its sink.
func TestTuiDeps_ServesTheHookSocket_issue653(t *testing.T) {
	dir, err := os.MkdirTemp("", "om") // a unix socket path is capped near 104 bytes
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	if runtimeFor(env).Watch.ListenHooks == nil {
		t.Fatal("ListenHooks is not wired: the watcher would call a nil server")
	}
	sink := make(chan dstatus.HookPayload, 1)
	sock := filepath.Join(dir, "s")
	l, err := runtimeFor(env).Watch.ListenHooks(sock, sink)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	c, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = c.Write([]byte(`{"session_id":"s1","hook_event_name":"Stop"}` + "\n"))
	_ = c.Close()
	if p := <-sink; p.SessionID != "s1" || p.HookEventName != "Stop" {
		t.Errorf("the wired server handed over %+v, want s1 Stop", p)
	}
}

// Step 5.3 (#653): a gate is run by an injected function, because running a
// step is infra's business. Left unset the Runner would call a nil function
// on the first gate, which no ui test with a recording runner could notice,
// so the real wiring is pinned: the wired function runs a real step.
func TestTuiDeps_RunsGatesThroughGateexec_issue653(t *testing.T) {
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	if runtimeFor(env).RunGate == nil {
		t.Fatal("RunGate is not wired: the first gate would call a nil runner")
	}
	results, err := runtimeFor(env).RunGate(context.Background(), t.TempDir(), []gate.Step{{Name: "ok", Run: "true"}})
	if err != nil || len(results) != 1 || results[0].Verdict != gate.Pass {
		t.Errorf("the wired runner returned %+v, %v; want one Pass", results, err)
	}
}

// Step 5.3 (#653): a coverage profile is read through an injected reader,
// because reading a file is infra's business. Left unset the TUI would load
// no overlay at all, silently, so the real wiring is pinned: the wired reader
// reads a real profile.
func TestTuiDeps_ReadsCoverageThroughFsread_issue653(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/m\n")
	mustWriteFile(t, filepath.Join(dir, "cover.out"), "mode: set\nexample.com/m/a.go:3.10,5.4 2 1\n")
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	deps := tuiDeps(env, nil, sessions.State{})
	if deps.Profiles == nil {
		t.Fatal("Profiles is not wired: no coverage overlay would ever load")
	}
	p, err := deps.Profiles.Load(context.Background(), filepath.Join(dir, "cover.out"), dir)
	if err != nil || !p.Files["a.go"].Lines[3] {
		t.Errorf("the wired reader returned %+v, %v; want a.go:3 covered", p, err)
	}
}

func mustWriteFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Step 5.8 (#653): the generated-file sniff reads a file's head through an
// injected reader, because opening a file is infra's business. Unwired, no
// header says "generated" and a hand-off file Go generated would be reviewed
// as hand-written, silently - so the real wiring is pinned: a real repository,
// a real header.
func TestTuiDeps_SniffsGeneratedHeadersThroughFsread_issue653(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	mustWriteFile(t, filepath.Join(dir, "stringer.go"), "// Code generated by stringer. DO NOT EDIT.\n\npackage p\n")
	env := tuiEnv{Home: "/h", Agent: claudeProfile(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	deps := tuiDeps(env, nil, sessions.State{})

	gen, err := deps.Generated(sessions.Session{ID: "s1", Dir: dir}, []string{"stringer.go"})

	if err != nil || !gen["stringer.go"] {
		t.Errorf("Generated = %v, %v; want stringer.go sniffed as generated", gen, err)
	}
}
