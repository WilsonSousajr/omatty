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
	"log/slog"
	"os/exec"
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

// Dtach runs each claude under a dtach master, so omatty's own exit detaches
// rather than killing it.
type Dtach struct {
	home string
	bin  string
}

// NewDtach returns a Dtach running bin against the sockets under home.
//
//	d := detach.NewDtach(home, "/opt/homebrew/bin/dtach")
func NewDtach(home, bin string) *Dtach { return &Dtach{home: home, bin: bin} }

// Persists is true: that is the whole point of this holder.
func (d *Dtach) Persists() bool { return true }

// Wrap rebuilds cmd as a dtach client for the session, keeping its directory
// and environment.
//
//	cmd, err := d.Wrap(sess.ID, exec.Command("claude", "--resume", sess.ID))
//
// An over-long socket path is refused here rather than passed to dtach, whose
// own failure names neither the session nor the limit (#43).
func (d *Dtach) Wrap(sessionID string, cmd *exec.Cmd) (*exec.Cmd, error) {
	sock, err := SocketPath(d.home, sessionID)
	if err != nil {
		return nil, err
	}
	out := exec.Command(d.bin, dtachArgs(sock, PidPath(d.home, sessionID), cmd.Args)...)
	out.Dir, out.Env = cmd.Dir, cmd.Env
	return out, nil
}

// pidScript records the held process's pid and then becomes it.
//
// dtach exposes neither its own pid nor its child's, and archiving a session
// has to be able to stop the claude behind it. $0 is the pidfile and $@ is the
// claude command; exec replaces the shell, so the pid already written is
// claude's own rather than a shell that has since gone (#43).
const pidScript = `echo $$ > "$0"; exec "$@"`

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
