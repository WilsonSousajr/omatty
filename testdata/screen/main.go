// Command screen replays captured PTY output into a terminal emulator and
// prints the final screen, so a person can read the frame the way the
// terminal drew it rather than as a stream of escapes.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/charmbracelet/x/vt"
)

func main() {
	cols, _ := strconv.Atoi(os.Args[2])
	rows, _ := strconv.Atoi(os.Args[3])
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	e := vt.NewEmulator(cols, rows)
	// The emulator answers mode queries on its own reader; with nobody
	// draining it, the first reply deadlocks the write.
	go func() { _, _ = io.Copy(io.Discard, e) }()
	if _, err := e.Write(raw); err != nil {
		panic(err)
	}
	for y := range rows {
		line := ""
		for x := range cols {
			c := e.CellAt(x, y)
			if c == nil || c.String() == "" {
				line += " "
				continue
			}
			line += c.String()
		}
		fmt.Printf("%s|\n", line)
	}
}
