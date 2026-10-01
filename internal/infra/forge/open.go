package forge

import (
	"os/exec"
	"runtime"
)

// openInBrowser hands url to the operator's browser, for a backend with no CLI
// of its own to do it (#462). The URL is built from the remote and a number,
// and passed as an argument, never through a shell.
func openInBrowser(url string) error { return launch(browserOpener(runtime.GOOS), url) }

// browserOpener is the platform's own opener: open on macOS, xdg-open elsewhere.
func browserOpener(goos string) string {
	if goos == "darwin" {
		return "open"
	}
	return "xdg-open"
}

// launch starts bin on url and does not wait: xdg-open with no desktop to hand
// off to runs the browser itself in the foreground, and the item must not wait
// until the browser is closed. It is reaped in the background, and its output
// goes nowhere (invariant 5).
func launch(bin, url string) error {
	cmd := exec.Command(bin, url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
