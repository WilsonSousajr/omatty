package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Migration step 6.2 (#653; ADR 0001, "tui/theme"): every colour and style
// the TUI draws with lives in internal/tui/theme. One built here is a second
// theme waiting to drift from the first - the strays this step gathered up
// were exactly that - so the package holds none of its own.
func TestApp_buildsNoStyleOrColourOfItsOwn_issue653(t *testing.T) {
	stray := regexp.MustCompile(`lipgloss\.(NewStyle|Color)\(`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if loc := stray.FindIndex(b); loc != nil {
			t.Errorf("%s builds a style or colour (%q); it belongs in internal/tui/theme", f, b[loc[0]:loc[1]])
		}
	}
}
