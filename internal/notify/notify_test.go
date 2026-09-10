package notify_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/notify"
)

// A title or body with a double quote must not break out of the osascript
// string literal.
func TestOsascriptArgv_EscapesQuotesAndBackslashes_issue38(t *testing.T) {
	argv := notify.OsascriptArgv(`fix "parser"`, `back\slash and "quote"`)

	joined := strings.Join(argv, " ")
	if argv[0] != "osascript" {
		t.Errorf("argv[0] = %q, want osascript", argv[0])
	}
	// The raw unescaped sequences must not survive.
	if strings.Contains(joined, `"parser"`) || strings.Contains(joined, `and "quote"`) {
		t.Errorf("a bare double quote reached the script:\n%s", joined)
	}
	// The escaped forms must be present.
	if !strings.Contains(joined, `\"parser\"`) {
		t.Errorf("the title quotes were not escaped:\n%s", joined)
	}
	if !strings.Contains(joined, `\\slash`) {
		t.Errorf("the backslash was not escaped:\n%s", joined)
	}
}

func TestFake_RecordsNotifications(t *testing.T) {
	f := &notify.Fake{}

	if err := f.Notify("omatty", "parser-fix needs you"); err != nil {
		t.Fatal(err)
	}

	if len(f.Sent) != 1 || f.Sent[0].Title != "omatty" || f.Sent[0].Body != "parser-fix needs you" {
		t.Errorf("Sent = %+v, want one recorded notification", f.Sent)
	}
}

func TestNew_ReturnsANotifierForThisPlatform_issue69(t *testing.T) {
	if notify.New() == nil {
		t.Fatal("New() returned nil")
	}
}

func TestSilent_ReportsSuccess_issue69(t *testing.T) {
	if err := (notify.Silent{}).Notify("omatty", "x"); err != nil {
		t.Errorf("Silent.Notify = %v, want nil", err)
	}
}

var _ notify.Notifier = (*notify.Fake)(nil)
var _ notify.Notifier = notify.Osascript{}
var _ notify.Notifier = notify.Silent{}
