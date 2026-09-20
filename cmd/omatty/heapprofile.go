package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
)

// heapProfileEnv names the file a heap profile is written to when the TUI
// exits. Empty or unset writes nothing, which is every ordinary run.
//
// A diagnostic knob rather than a feature. omatty holds one terminal
// emulator per session for as long as it runs, and when the question is
// which of them is holding what, the only honest answer comes from a
// profile - the memory investigation that added this had to infer it from
// vmmap and a benchmark because the binary could not be asked.
//
//	OMATTY_HEAP_PROFILE=/tmp/omatty.heap omatty   # quit normally, then:
//	go tool pprof -top /tmp/omatty.heap
const heapProfileEnv = "OMATTY_HEAP_PROFILE"

// writeHeapProfile writes the profile the environment asked for, if it asked
// for one. A failure is logged and nothing else: a diagnostic must never be
// the reason omatty fails to exit, and stdout is not ours to write to
// (invariant 5).
func writeHeapProfile() {
	path := os.Getenv(heapProfileEnv)
	if path == "" {
		return
	}
	if err := heapProfileTo(path); err != nil {
		slog.Warn("writing the heap profile", "path", path, "err", err)
	}
}

// heapProfileTo writes the live heap to path.
//
// The GC runs first so the profile reports what is still reachable rather
// than what merely has not been swept yet - without it a run that has just
// drawn a few hundred frames reports its garbage as if it were live, which
// is the opposite of the question being asked.
func heapProfileTo(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("omatty: creating heap profile %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("omatty: writing heap profile %q: %w", path, err)
	}
	return nil
}
