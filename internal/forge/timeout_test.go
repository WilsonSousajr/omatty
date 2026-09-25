package forge_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// A gh that stalls - a laptop waking, a network that changed under it - used
// to hold the project's poll until TCP gave up, minutes later, with the card
// still showing the last verdict and no "?". Now it is an ordinary error
// inside the bound, and it says it timed out. The fake's sleep is a child of
// sh holding stdout open, so this also proves WaitDelay: without it, Output
// would wait for the sleep after sh was killed.
func TestCLI_ListPRsGivesUpOnAStalledGh_issue356(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 5\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cli := forge.NewCLIWithTimeout(bin, 50*time.Millisecond)

	start := time.Now()
	_, err := cli.ListPRs(t.TempDir())

	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("ListPRs took %v against a stalled gh, want it bounded", took)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want it to say the call timed out", err)
	}
}
