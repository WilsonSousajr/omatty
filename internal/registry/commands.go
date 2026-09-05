package registry

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RepoRooter is the slice of vcs.Git the registry needs. Declared narrow so a
// fake in a caller's tests carries one method rather than nine - cmd/omatty's
// adapters could not be tested at all while they demanded the concrete *vcs.CLI
// (#91).
type RepoRooter interface {
	RepoRoot(dir string) (string, error)
}

// SessionBrancher is the slice of vcs.Git adoption needs: which branch a
// directory is on. Declared beside RepoRooter and just as narrow, for the
// reason RepoRooter's own comment gives.
type SessionBrancher interface {
	CurrentBranch(dir string) (string, error)
}

// SessionPick is one session offered for adoption, as far as the registry is
// concerned: enough to write a row and nothing more.
type SessionPick struct {
	ID    string
	Title string
	Dir   string
}

// Adoption is what became of one pick AdoptAll was given, mirroring
// Registration. Session is the row that was actually written; Err is why there
// is none.
type Adoption struct {
	Session Session
	Err     error
}

// AdoptAll adopts each pick, carrying on past a failure so one collision does
// not abandon the rest of a bulk pick.
//
//	for _, a := range registry.AdoptAll(store, git, "omatty", picks) { ... }
//
// It exists rather than a loop in cmd and another in ui's adapter, which is
// RegisterAll's argument restated: both loops existed, and both would have had
// to learn about the branch below (#91, #122).
func AdoptAll(s *Store, git SessionBrancher, project string, picks []SessionPick) []Adoption {
	out := make([]Adoption, 0, len(picks))
	for _, p := range picks {
		sess, err := AdoptSession(s, git, p.ID, project, p.Title, p.Dir)
		out = append(out, Adoption{Session: sess, Err: err})
	}
	return out
}

// AddProject registers the git repository containing dir, naming it after
// the repository's own directory. It is the whole of `omatty add`.
//
//	p, err := registry.AddProject(store, vcs.NewCLI(), cwd)
func AddProject(s *Store, git RepoRooter, dir string) (Project, error) {
	root, err := git.RepoRoot(dir)
	if err != nil {
		// Deliberately does not assert the cause: the path may be missing, a
		// file, or a directory that simply is not a repository (issue #29).
		return Project{}, fmt.Errorf("registry: cannot register %q: %w", dir, err)
	}
	st, err := s.Load()
	if err != nil {
		return Project{}, err
	}
	p := Project{Name: filepath.Base(root), Root: root}
	// The name is the key sessions look up, so a collision would silently
	// attach new sessions to the wrong repository.
	if existing, err := findProject(&st, p.Name); err == nil {
		return Project{}, fmt.Errorf(
			"registry: project %q is already registered at %q", p.Name, existing.Root)
	}
	st.Projects = append(st.Projects, p)
	return p, s.Save(st)
}

// Registration is what became of one root RegisterAll was given. Project is
// the row that was actually written, so a caller never has to reconstruct it.
type Registration struct {
	Root    string
	Project Project
	Err     error
}

// RegisterAll registers each root, carrying on past a failure so one collision
// does not abandon the rest of a bulk pick. AddProject refuses a duplicate
// *name* even when the roots differ, which one repository at a time is a rare
// annoyance and in bulk is not (#91).
//
// It exists so `omatty discover` and the TUI picker share one algorithm. They
// had a copy each - the same loop in cmd/omatty and in ui - which invariant 10
// forbids and which `dupl` cannot see across packages, so a change to the
// collision policy would have had to be made twice or the two would disagree.
//
//	for _, r := range registry.RegisterAll(store, git, roots) { ... }
func RegisterAll(s *Store, git RepoRooter, roots []string) []Registration {
	out := make([]Registration, 0, len(roots))
	for _, root := range roots {
		p, err := AddProject(s, git, root)
		out = append(out, Registration{Root: root, Project: p, Err: err})
	}
	return out
}

// RenameSession retitles a session in place. The title is display-only, so
// this touches nothing that relaunching a session depends on (invariant 9):
// it is a state.json edit and a sidebar rebuild (#41).
//
//	err := registry.RenameSession(store, sess.ID, "parser-fix")
func RenameSession(s *Store, id, title string) error {
	// A blank title would leave a sidebar row with nothing to aim at. The check
	// is on the trimmed title, not on title == "": a name of nothing but spaces
	// renders exactly as empty a row, and passed the old guard (#41).
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf(
			"registry: session %q: title %q is blank, want a name with a non-space character", id, title)
	}
	st, err := s.Load()
	if err != nil {
		return err
	}
	i, err := indexOfSession(&st, id)
	if err != nil {
		return err
	}
	st.Sessions[i].Title = title
	return s.Save(st)
}

// RemoveSession drops a session from the registry and returns it, so the
// caller can decide from its own fields whether a worktree may be deleted
// (#40). The transcript on disk is untouched: this forgets a session, it does
// not destroy its history.
//
//	sess, err := registry.RemoveSession(store, id)
//	if err == nil && sess.Worktree { /* git worktree remove sess.Dir */ }
func RemoveSession(s *Store, id string) (Session, error) {
	st, err := s.Load()
	if err != nil {
		return Session{}, err
	}
	i, err := indexOfSession(&st, id)
	if err != nil {
		return Session{}, err
	}
	sess := st.Sessions[i]
	st.Sessions = append(st.Sessions[:i], st.Sessions[i+1:]...)
	return sess, s.Save(st)
}

// indexOfSession locates a session by id. It returns an index rather than a
// Session because callers mutate the one inside the state they are about to
// save; a copy would be written back over.
//
// The miss returns -1, not 0: a caller that dropped the error would otherwise
// rename or archive whichever session happens to sit first in state.json, and
// silent data corruption is worse than the immediate panic an out-of-range
// index gives (#41).
func indexOfSession(st *State, id string) (int, error) {
	for i := range st.Sessions {
		if st.Sessions[i].ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf(
		"registry: no session with id %q (known sessions: %v)", id, sessionIDs(st))
}

func sessionIDs(st *State) []string {
	ids := make([]string, 0, len(st.Sessions))
	for _, sess := range st.Sessions {
		ids = append(ids, sess.ID)
	}
	return ids
}

// NamedProject is the project registered under name.
//
//	p, err := registry.NamedProject(store, "omatty")
//
// Exported because `omatty adopt` acts on one named project and cmd/ carried
// its own copy of this loop, against invariant 10 - so a change to what a name
// means would have had to be made twice, or the CLI and the registry would
// disagree about which project an argument selects (#122).
func NamedProject(s *Store, name string) (Project, error) {
	st, err := s.Load()
	if err != nil {
		return Project{}, err
	}
	return findProject(&st, name)
}

// KnownSessionIDs is every session id state.json holds, which is what adoption
// leaves out of what it offers (#91, #122). Exported for the same reason
// NamedProject is: cmd/ had a second copy of it.
func KnownSessionIDs(s *Store) ([]string, error) {
	st, err := s.Load()
	if err != nil {
		return nil, err
	}
	return sessionIDs(&st), nil
}

// AdoptSession registers a claude session that already exists, so omatty can
// show and resume one it did not create (#122).
//
//	sess, err := registry.AdoptSession(store, git, cand.ID, "omatty", cand.Title, cand.Dir)
//
// Worktree is false and that is load-bearing rather than incidental: omatty did
// not create dir, so archive must never offer to delete it. That is the rule
// archiveChoices already applies to a main-checkout session (#40).
//
// Nothing else is needed to relaunch it (invariant 9): the launcher stats the
// transcript, finds one, and uses `--resume` (#36).
func AdoptSession(s *Store, git SessionBrancher, id, project, title, dir string) (Session, error) {
	if strings.TrimSpace(title) == "" {
		return Session{}, fmt.Errorf(
			"registry: session %q: title %q is blank, want a name with a non-space character", id, title)
	}
	st, err := s.Load()
	if err != nil {
		return Session{}, err
	}
	sess, err := adoptable(&st, git, id, project, title, dir)
	if err != nil {
		return Session{}, err
	}
	st.Sessions = append(st.Sessions, sess)
	return sess, s.Save(st)
}

// adoptable is the row to write for a pick, or why there is none: the project
// has to exist, the id has to be new, and a worktree session's branch has to be
// read before anything is saved - so a git call that fails leaves state.json
// untouched rather than half-written.
func adoptable(st *State, git SessionBrancher, id, project, title, dir string) (Session, error) {
	p, err := findProject(st, project)
	if err != nil {
		return Session{}, err
	}
	if err := refuseKnownSession(st, id); err != nil {
		return Session{}, err
	}
	branch, err := adoptedBranch(git, p.Root, dir)
	if err != nil {
		return Session{}, err
	}
	return Session{ID: id, Project: project, Title: title, Dir: dir, Branch: branch}, nil
}

// adoptedBranch is the branch to record for a session adopted in dir, and "" for
// one that ran in the project's own checkout.
//
// Empty is not merely a default there: review.Source.baseCommit reads a blank
// Branch as "this is a main-checkout session" and diffs against HEAD, which is
// right for the checkout and wrong for a worktree. Recording nothing meant
// `ctrl+o d` on an adopted worktree session showed only its uncommitted changes
// and hid every commit it had made - and the comments composed from that diff
// anchored to the wrong base (#122).
//
// Worktree stays false regardless: omatty did not create dir, so archive must
// never offer to delete it (#40). This says which branch, not whose directory.
func adoptedBranch(git SessionBrancher, projectRoot, dir string) (string, error) {
	if dir == "" || filepath.Clean(dir) == filepath.Clean(projectRoot) {
		return "", nil
	}
	branch, err := git.CurrentBranch(dir)
	if err != nil {
		return "", fmt.Errorf("registry: session directory %q: cannot read the branch it is on: %w", dir, err)
	}
	return branch, nil
}

// refuseKnownSession rejects an id the registry already holds. Two sidebar rows
// sharing one session would share its process, and the second would fight the
// first for the PTY.
func refuseKnownSession(st *State, id string) error {
	if _, err := indexOfSession(st, id); err == nil {
		return fmt.Errorf("registry: session %q is already registered", id)
	}
	return nil
}

// AddSession creates and persists a session. It is the whole of `omatty new`.
// State is saved only after the session is fully created, so a failed
// worktree leaves nothing behind.
func AddSession(s *Store, c *Creator, project, title, branch string) (Session, error) {
	st, err := s.Load()
	if err != nil {
		return Session{}, err
	}
	sess, err := c.Create(&st, project, title, branch)
	if err != nil {
		return Session{}, err
	}
	return sess, s.Save(st)
}
