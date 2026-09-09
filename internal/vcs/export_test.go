package vcs

// ParseShortstat is the parser behind CLI.Shortstat, for the table test: the
// integration test drives git for real, but git will not print a
// deletions-only or a binary-only line on demand (#180).
func ParseShortstat(line string) (Shortstat, error) { return parseShortstat(line) }
