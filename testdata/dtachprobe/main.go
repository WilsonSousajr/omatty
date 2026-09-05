// Command dtachprobe proves the one thing M6 cannot prove with a unit test:
// that a process wrapped by internal/detach really does survive its client
// going away, and that reattaching finds the same process rather than starting
// a second one.
//
//	go run ./testdata/dtachprobe /tmp/probehome
//
// It runs the exact command line detach.Dtach.Wrap builds, inside a real PTY,
// because dtach refuses to attach without a terminal - which is how bubbleterm
// runs it in the app. The stand-in for a claude mid-turn is a shell counting
// once a second: if the second attach shows the count continuing rather than
// restarting at 1, the master and its child survived the detach.
//
// Not part of the gate; a person reads the output (roadmap rule 2).
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"

	"github.com/WilsonSousajr/omatty/internal/detach"
)

// counter stands in for a claude that is mid-turn: it produces numbered output
// forever, so the numbers say whether this is the same process as before.
const counter = "i=0; while :; do i=$((i+1)); echo tick $i; sleep 1; done"

func main() {
	if len(os.Args) < 2 {
		exit("usage: dtachprobe <home>")
	}
	home := os.Args[1]
	holder := detach.NewFor(home, "dtach")
	fmt.Println("persists:", holder.Persists())

	cmd, err := holder.Wrap("probe-session", exec.Command("sh", "-c", counter))
	if err != nil {
		exit("wrap: " + err.Error())
	}
	fmt.Println("command:", strings.Join(cmd.Args, " "))

	fmt.Println("\n--- first attach (creates the master) ---")
	fmt.Println(attachFor(cmd, 4*time.Second))

	fmt.Println("\n--- client gone; waiting, as if omatty were closed ---")
	time.Sleep(3 * time.Second)

	fmt.Println("\n--- second attach (must find the same process) ---")
	fmt.Println(attachFor(cmd, 4*time.Second))

	pid, _ := os.ReadFile(detach.PidPath(home, "probe-session"))
	fmt.Printf("\npidfile: %q\n", strings.TrimSpace(string(pid)))
	if err := holder.Stop("probe-session"); err != nil {
		exit("stop: " + err.Error())
	}
	fmt.Println("stopped; pidfile removed:", !exists(detach.PidPath(home, "probe-session")))
}

// attachFor runs one dtach client in a PTY for d, then kills the client only -
// which is what closing omatty's embedded terminal does - and returns what the
// client printed.
func attachFor(cmd *exec.Cmd, d time.Duration) string {
	client := exec.Command(cmd.Path, cmd.Args[1:]...)
	client.Dir, client.Env = cmd.Dir, cmd.Env
	f, err := pty.StartWithSize(client, &pty.Winsize{Rows: 20, Cols: 80})
	if err != nil {
		return "start: " + err.Error()
	}
	out := drain(f)
	time.Sleep(d)
	_ = client.Process.Kill()
	_, _ = client.Process.Wait()
	_ = f.Close()
	return out()
}

// drain reads the PTY until it closes, returning a function that blocks for the
// collected output. Reading has to run while the client lives or the master
// blocks writing to a full buffer.
func drain(f io.Reader) func() string {
	var (
		wg  sync.WaitGroup
		buf strings.Builder
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, f)
	}()
	return func() string {
		wg.Wait()
		return strings.TrimSpace(buf.String())
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func exit(msg string) {
	_, _ = os.Stderr.WriteString("dtachprobe: " + msg + "\n")
	os.Exit(1)
}
