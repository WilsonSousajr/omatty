package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The whole point of the knob is that the file is readable by `go tool
// pprof` afterwards, so assert the shape it writes: a heap profile is a
// gzipped protobuf and starts with the gzip magic.
func TestHeapProfileTo_writesAProfileGoToolPprofCanRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heap.out")

	if err := heapProfileTo(path); err != nil {
		t.Fatalf("heapProfileTo(%q) = %v, want nil", path, err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the profile back: %v", err)
	}
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		t.Errorf("profile starts %x, want the gzip magic 1f8b that pprof expects", b[:min(2, len(b))])
	}
}

// An unwritable path must be reported, not swallowed: a knob that silently
// writes nothing is worse than no knob.
func TestHeapProfileTo_unwritablePathIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "heap.out")

	if err := heapProfileTo(path); err == nil {
		t.Errorf("heapProfileTo(%q) = nil, want an error naming the path", path)
	}
}
