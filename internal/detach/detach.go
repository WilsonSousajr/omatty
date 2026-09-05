// Package detach is omatty's only route to the dtach binary.
//
// Invariant 4: a third-party program omatty does not control is reachable
// through one package it owns, the way internal/vcs owns the git CLI and
// internal/termwrap owns bubbleterm. Nothing outside this package names dtach.
//
// The problem it solves: quitting omatty closes every PTY, which hangs up the
// slave and SIGHUPs each claude, so the turn in flight dies. The conversation
// survives on disk and #36 resumes it, but the work does not. Running claude
// under dtach makes quitting a detach instead of a kill (#43).
//
// dtach is optional. Where the binary is absent, New returns a Plain holder
// that changes nothing, so omatty behaves exactly as it did before this
// package existed - the same way a failed hook socket degrades to tailer-only
// (#49).
package detach

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"

	"github.com/WilsonSousajr/omatty/internal/paths"
)

// binary is the program a Holder looks for. Named once here rather than spelled
// at each call site, so the PATH lookup and the log message cannot disagree.
const binary = "dtach"

// Holder keeps a session's claude alive while omatty is not attached to it.
//
//	h := detach.New(home)
//	cmd, err := h.Wrap(sess.ID, exec.Command("claude", "--resume", sess.ID))
type Holder interface {
	// Wrap returns the command omatty should actually start for a session:
	// under a Dtach that is a dtach client, under Plain it is cmd itself.
	Wrap(sessionID string, cmd *exec.Cmd) (*exec.Cmd, error)
	// Stop ends the held process. Archiving a session is the one place omatty
	// deliberately kills a claude (#40); quitting must not.
	Stop(sessionID string) error
	// Persists reports whether a session survives quitting omatty, which is
	// what the UI warns about when it does not.
	Persists() bool
	// Notice is what the footer says at startup about this holder, and nothing
	// at all when there is nothing to say.
	//
	// Here rather than in cmd/ because the text names dtach and the command
	// that installs it, and invariant 4 is that nothing outside this package
	// does. It was a literal in cmd/omatty and twice more in ui's tests, so
	// swapping the holder for abduco, tmux or a built-in meant editing all
	// three (#43).
	Notice() string
}

// New returns the holder for this machine: a Dtach when the binary is on PATH,
// otherwise the Plain fallback, with a line in the log saying which and why.
//
//	holder := detach.New(home)
func New(home string) Holder { return NewFor(home, binary) }

// NewFor is New with the binary named, so a test can choose one that exists and
// one that cannot without depending on what the machine has installed.
func NewFor(home, bin string) Holder {
	path, err := exec.LookPath(bin)
	if err != nil {
		slog.Warn("dtach is not installed; sessions will not survive quitting omatty",
			"binary", bin, "err", err)
		return &Plain{}
	}
	return NewDtach(home, path)
}

// Plain is the holder for a machine without dtach: it holds nothing. Wrap
// returns the command untouched and Stop has nothing to stop, which together
// are exactly omatty's behaviour before #43.
type Plain struct{}

// Wrap returns cmd unchanged.
func (p *Plain) Wrap(_ string, cmd *exec.Cmd) (*exec.Cmd, error) { return cmd, nil }

// Stop reports success because there is no held process: the claude died with
// its PTY. An error here would make archiving look broken on every machine
// without dtach.
func (p *Plain) Stop(_ string) error { return nil }

// Persists is false: without dtach, quitting omatty still kills every session.
func (p *Plain) Persists() bool { return false }

// Notice warns that sessions die on quit and names the command that fixes it,
// because the alternative is a warning an operator cannot act on: dtach is an
// unusual enough dependency that the warning alone does not imply what to
// install.
//
// It shares the footer line with the exit key rather than replacing it, so it
// is kept short enough that both fit at the default 80 columns (#28, #43).
func (p *Plain) Notice() string {
	return "no dtach: sessions will not survive quit (" + installHint() + ")"
}

// installHint names the command that installs dtach on this machine.
//
// Switched on the platform because the notice named `brew install dtach`
// everywhere, which on a Linux box is a command the operator does not have -
// and the README this shipped with documents apt for exactly that reason (#43).
func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install dtach"
	case "linux":
		return "apt install dtach"
	default:
		return "install dtach"
	}
}

// Dtach runs each claude under a dtach master, so omatty's own exit detaches
// rather than killing it.
type Dtach struct {
	home string
	bin  string
	// maxSock is the socket-path limit this holder enforces, defaulting to
	// maxSocketPath. A field rather than the const alone so a test can reach
	// the limit under t.TempDir() (#43).
	maxSock int
}

// NewDtach returns a Dtach running bin against the sockets under home.
//
//	d := detach.NewDtach(home, "/opt/homebrew/bin/dtach")
func NewDtach(home, bin string) *Dtach { return NewDtachCapped(home, bin, maxSocketPath) }

// NewDtachCapped is NewDtach with the socket-path limit named, the way NewFor
// is New with the binary named: a test can exercise the limit from a t.TempDir
// that is nowhere near it, rather than hunting for a directory short enough to
// sit under the real one (#43).
//
//	d := detach.NewDtachCapped(t.TempDir(), "dtach", 40)
func NewDtachCapped(home, bin string, maxSock int) *Dtach {
	return &Dtach{home: home, bin: bin, maxSock: maxSock}
}

// socketPath is SocketPath under this holder's own limit.
func (d *Dtach) socketPath(sessionID string) (string, error) {
	return socketPathWithin(d.home, sessionID, d.maxSock)
}

// Persists is true: that is the whole point of this holder.
func (d *Dtach) Persists() bool { return true }

// Notice is empty: sessions survive, so there is nothing to warn about and the
// footer keeps its keymap.
func (d *Dtach) Notice() string { return "" }

// Wrap rebuilds cmd as a dtach client for the session, keeping its directory
// and environment.
//
//	cmd, err := d.Wrap(sess.ID, exec.Command("claude", "--resume", sess.ID))
//
// An over-long socket path is refused here rather than passed to dtach, whose
// own failure names neither the session nor the limit (#43).
func (d *Dtach) Wrap(sessionID string, cmd *exec.Cmd) (*exec.Cmd, error) {
	if cmd.Err != nil {
		// exec.Command records an unresolvable binary here rather than
		// returning it, and Plain hands cmd straight to Start, which surfaces
		// it. Rebuilding the command around dtach drops the field, so a
		// missing claude became a session that started, flashed
		// "sh: claude: not found" and died with nothing in the log (#43).
		return nil, fmt.Errorf("detach: session %s: %w", sessionID, cmd.Err)
	}
	sock, err := d.socketPath(sessionID)
	if err != nil {
		return nil, err
	}
	if err := ensureSessionDir(d.home, sessionID); err != nil {
		return nil, err
	}
	out := exec.Command(d.bin, dtachArgs(sock, PidPath(d.home, sessionID), cmd.Args)...)
	out.Dir, out.Env = cmd.Dir, cmd.Env
	out.Stdin, out.Stdout, out.Stderr = cmd.Stdin, cmd.Stdout, cmd.Stderr
	out.SysProcAttr = cmd.SysProcAttr
	return out, nil
}

// ensureSessionDir creates the directory the socket and pidfile live in.
//
// Regression, issue #43: nothing created it, so on a machine that had never run
// this before dtach exited 1 with "No such file or directory" and every session
// failed to start. Every unit test passed, because they assert the command line
// rather than run it - the smoke test is what found it, which is the argument
// for roadmap rule 2 restated.
//
// 0700 because the socket is a control channel into a running claude: anyone
// who can connect to it can type into that session.
func ensureSessionDir(home, sessionID string) error {
	dir := paths.SessionDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("detach: session %s: creating %q for its socket: %w", sessionID, dir, err)
	}
	// MkdirAll is a no-op on a directory that already exists and never touches
	// its mode, so a ~/.omatty/s left at 0755 by an earlier build, a restored
	// backup or a wider umask kept those bits and the 0700 above was a claim
	// rather than a guarantee.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("detach: session %s: making %q private: %w", sessionID, dir, err)
	}
	return nil
}

// pidScript records the held process's pid and then becomes it.
//
// dtach exposes neither its own pid nor its child's, and archiving a session
// has to be able to stop the claude behind it. $0 is the pidfile and $@ is the
// claude command; exec replaces the shell, so the pid already written is
// claude's own rather than a shell that has since gone (#43).
//
// The write is fatal by design. Without the `|| exit 1` a failed redirection -
// a full or read-only home - still fell through to exec, and that session was
// one Stop could never end: readPid found no file, archiving became a silent
// no-op, and a claude went on running behind a socket with no row in
// state.json. A session that refuses to start is recoverable; one that cannot
// be stopped is the leak archive.go says this code exists to prevent.
const pidScript = `echo $$ > "$0" || exit 1; exec "$@"`

// dtachArgs is the command line, with one comment per flag because each is
// load-bearing and none is obvious:
//
//   - -A attaches to the socket if it is live and creates it otherwise, so
//     first launch and reattach are one code path. After a reboot the socket is
//     gone, dtach creates a fresh master, and the claude it runs carries
//     --resume because the transcript exists (#36).
//   - -E disables dtach's own detach key (ctrl+\) and -z its suspend key
//     (ctrl+z), so both still reach Claude. Invariant 1: omatty intercepts one
//     key, and a layer underneath it must not quietly intercept two more.
//   - -r winch redraws by sending SIGWINCH on attach. Claude Code repaints on a
//     window-size change, so without this the reattached pane stays blank until
//     the session next writes something.
func dtachArgs(sock, pid string, claude []string) []string {
	args := []string{"-A", sock, "-E", "-z", "-r", "winch", "sh", "-c", pidScript, pid}
	return append(args, claude...)
}
