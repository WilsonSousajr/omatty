package forge

import (
	"os/exec"
	"runtime"
)

// openInBrowser hands url to the operator's browser, for a backend with no CLI
// of its own to do it (#462): open on macOS, xdg-open elsewhere. The URL is
// built from the remote and a number, and passed as an argument, never
// through a shell.
func openInBrowser(url string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, url).Run()
}
