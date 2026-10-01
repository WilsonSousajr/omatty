package main

import (
	"strings"
	"testing"
)

// --stats reads and prints; it must never write. The flag sits beside
// --detect, which has the same contract.
func TestGateCommand_statsWritesNothing_issue332(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"omatty", "--stats"}, strings.NewReader(""), nil); err != nil {
		t.Fatalf("gateCommand(--stats) error = %v", err)
	}

	if got := configuredGate(t, store); got != nil {
		t.Errorf("gate = %v, want --stats to have written nothing", got)
	}
}

// A project that is not registered is a usage error naming it, as it is for
// every other form of this command.
func TestGateCommand_statsNeedsAKnownProject_issue332(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"ghost", "--stats"}, strings.NewReader(""), nil); err == nil {
		t.Error("--stats on an unknown project should fail")
	}
}
