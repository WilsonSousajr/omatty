package forge_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// hostile is text an issue's author controls, carrying an OSC 52 clipboard
// write (ESC ] 52 ; c ; <base64> BEL), a screen clear (CSI 2 J), a C1 CSI and
// a DEL - with a newline and a tab that are the text's own layout.
const hostile = `line one\n\tindented\u001b]52;c;ZXZpbA==\u0007 \u001b[2J \u009b31m \u007f end`

// controlIn names the first control character in s that is not \n or \t, or ""
// when there is none.
func controlIn(s string) string {
	for _, r := range s {
		if r == '\n' || r == '\t' {
			continue
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return string(r)
		}
	}
	return ""
}

// Regression, issue #483: forge text reached the terminal with its escape
// sequences, so an issue body could write the operator's clipboard (OSC 52) or
// redraw omatty's screen the moment the issue was opened. Every string folded
// from gh must come out with no control character but its newlines and tabs.
func TestFoldDetail_StripsControlCharacters_issue483(t *testing.T) {
	raw := `{"number":1,"title":"` + hostile + `","body":"` + hostile + `","author":{"login":"x` + `\u001b[1m"},` +
		`"comments":[{"author":{"login":"y"},"body":"` + hostile + `"}]}`
	d, err := forge.FoldDetail([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{"title": d.Title, "body": d.Body, "author": d.Author, "comment": d.Comments[0].Body} {
		if c := controlIn(s); c != "" {
			t.Errorf("%s kept control character %q: %q", name, c, s)
		}
	}
	if d.Body[:9] != "line one\n" || d.Body[9] != '\t' {
		t.Errorf("the body lost its own newline or tab: %q", d.Body)
	}
}

func TestFoldLists_StripControlCharacters_issue483(t *testing.T) {
	issues, err := forge.FoldIssues([]byte(`[{"number":1,"title":"` + hostile + `","labels":[{"name":"` + hostile + `"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	prs, err := forge.Fold([]byte(`[{"number":2,"title":"` + hostile + `","state":"OPEN"}]`))
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range map[string]string{"issue title": issues[0].Title, "label": issues[0].Labels[0], "pr title": prs[0].Title} {
		if c := controlIn(s); c != "" {
			t.Errorf("%s kept control character %q: %q", name, c, s)
		}
	}
}
