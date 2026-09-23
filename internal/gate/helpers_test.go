package gate_test

import (
	"os"
	"strings"
)

// readFile is here so a test can assert that a step did, or did not, run:
// the step writes a file and the assertion reads it. Named rather than
// inlined so the failure message says which.
func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// writeFile backs the "a file is not a directory" case.
func writeFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o600)
}

// shellQuote wraps s for a single-quoted shell word, so a recorded fixture
// containing tabs, newlines and % reaches printf unchanged.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
