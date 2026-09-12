package gate_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

func TestVerdict_String(t *testing.T) {
	cases := map[gate.Verdict]string{
		gate.Pending:   "pending",
		gate.Running:   "running",
		gate.Pass:      "pass",
		gate.Fail:      "fail",
		gate.Missing:   "missing",
		gate.Cancelled: "cancelled",
	}
	for verdict, want := range cases {
		if got := verdict.String(); got != want {
			t.Errorf("Verdict(%d).String() = %q, want %q", int(verdict), got, want)
		}
	}
}

// An unnamed verdict must still print something a log can carry, rather than
// an empty string that reads as "no verdict".
func TestVerdict_String_unknownValueSaysSo(t *testing.T) {
	if got := gate.Verdict(99).String(); !strings.Contains(got, "99") {
		t.Errorf("Verdict(99).String() = %q, want it to carry the number", got)
	}
}
