// Command ptyrun runs a program in a real, sized pseudo-terminal, feeds it
// keystrokes after a delay, and dumps everything it wrote. It exists because
// M1's three worst bugs (#31, #33, #36) all passed the coverage gate: every
// test substituted a fake for claude, so the wiring between real parts was
// never exercised. Roadmap rule 2 says every milestone ends with this.
//
//	go run ./testdata/ptyrun omatty                       # 100x30, ctrl+o q after 8s
//	PTY_COLS=60 PTY_ROWS=20 PTY_KEYS=$'\x0fj' go run ./testdata/ptyrun omatty
//	PTY_KEYS=$'\x0fd' PTY_KEYS2=$'jjjjc' go run ./testdata/ptyrun omatty
//
// PTY_KEYS2 is written PTY_WAIT2 later, for a flow whose first keys start
// work the next ones depend on. Read the frame with ./testdata/screen.
//
// Not part of the gate; a person reads the output.
package main

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/creack/pty"
)

func main() {
	if len(os.Args) < 2 {
		os.Stderr.WriteString("usage: ptyrun <command> [args...]\n")
		os.Exit(2)
	}
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: envUint16("PTY_ROWS", 30), Cols: envUint16("PTY_COLS", 100)})
	if err != nil {
		os.Stderr.WriteString("ptyrun: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	out := capture(f)
	time.Sleep(envDuration("PTY_WAIT", 8*time.Second))
	_, _ = f.Write([]byte(envString("PTY_KEYS", "\x0fq"))) // ctrl+o q
	// A second batch, for a flow that must wait on work the first batch
	// started: the review pane loads its diff in a command, so keys sent in
	// the same burst as `ctrl+o d` arrive before there is a diff to move
	// around in (#21). Empty by default, so existing runs are unchanged.
	if keys := envString("PTY_KEYS2", ""); keys != "" {
		time.Sleep(envDuration("PTY_WAIT2", 3*time.Second))
		_, _ = f.Write([]byte(keys))
	}
	select {
	case <-out.done:
	case <-time.After(4 * time.Second):
		_ = cmd.Process.Kill()
	}
	_, _ = os.Stdout.Write(out.buf)
}

type captured struct {
	buf  []byte
	done chan struct{}
}

func capture(f *os.File) *captured {
	c := &captured{done: make(chan struct{})}
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := f.Read(b)
			c.buf = append(c.buf, b[:n]...)
			if err != nil {
				close(c.done)
				return
			}
		}
	}()
	return c
}

func envUint16(key string, def uint16) uint16 {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return uint16(v)
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
