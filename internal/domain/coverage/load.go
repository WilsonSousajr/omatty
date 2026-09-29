package coverage

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads a coverage profile and picks the parser by looking at it.
//
//	p, err := coverage.Load(filepath.Join(sess.Dir, step.Profile), sess.Dir, mod)
//
// Sniffed rather than chosen by file extension: .info, .lcov, .out and .txt
// are all in use for both formats, and a project is free to name its profile
// whatever its tooling defaults to.
//
// A file that is neither format is an error, not an empty profile. An empty
// profile would quietly mean "nothing here is uncovered", which is a lie the
// operator has no way to notice.
func Load(path, root, modulePath string) (Profile, error) {
	f, err := os.Open(path) //nolint:gosec // the operator's own configured profile
	if err != nil {
		return Profile{}, fmt.Errorf("coverage: opening profile %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	buf := bufio.NewReader(f)
	switch format(buf) {
	case formatGo:
		return ParseGo(buf, modulePath)
	case formatLCOV:
		return ParseLCOV(buf, root)
	}
	return Profile{}, fmt.Errorf("coverage: profile %q is not a Go or lcov profile", path)
}

type profileFormat int

const (
	formatUnknown profileFormat = iota
	formatGo
	formatLCOV
)

// format peeks at the head of the file without consuming it, so the parser
// that follows still sees the whole thing.
func format(buf *bufio.Reader) profileFormat {
	head, _ := buf.Peek(peekBytes)
	for _, line := range strings.Split(string(head), "\n") {
		switch line = strings.TrimSpace(line); {
		case strings.HasPrefix(line, "mode:"):
			return formatGo
		case strings.HasPrefix(line, "SF:"), strings.HasPrefix(line, "TN:"):
			return formatLCOV
		}
	}
	return formatUnknown
}

// peekBytes is enough to reach the first meaningful line past any leading
// blanks, and small enough that bufio's default buffer holds it.
const peekBytes = 2048
