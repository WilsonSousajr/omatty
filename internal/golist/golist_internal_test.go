package golist

import (
	"errors"
	"strings"
	"testing"
)

// go list writes a stream of objects, and a truncated or corrupt stream must
// be an error rather than a short slice: a scorer handed half the packages
// would report a clean tree for the half it never saw.
func TestDecode_aCorruptStreamIsAnError(t *testing.T) {
	_, err := decode(strings.NewReader(`{"ImportPath":"m/a"} {not json}`))

	if err == nil {
		t.Fatal("decode() error = nil, want the corrupt stream surfaced")
	}
	if !strings.Contains(err.Error(), "golist:") {
		t.Errorf("error = %q, want it to name the package doing the work", err)
	}
}

// An empty stream is not corrupt - a pattern can legitimately match nothing.
func TestDecode_anEmptyStreamIsNoPackages(t *testing.T) {
	pkgs, err := decode(strings.NewReader(""))

	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 0 {
		t.Errorf("decode() = %v, want no packages", pkgs)
	}
}

// said reads the sentence a person needs out of an ExitError. Anything else -
// the binary missing, the context ending - carries no stderr, and must add
// nothing rather than panic reaching for it.
func TestSaid_addsNothingToAnErrorThatCarriesNoStderr(t *testing.T) {
	if got := said(errors.New("go: not found")); got != "" {
		t.Errorf("said() = %q, want empty for an error with no stderr", got)
	}
}
