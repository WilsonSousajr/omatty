package gate

// boundedOutput collects a step's output while holding at most KeptBytes of
// it, keeping the newest bytes and counting what it discarded so the elision
// can still say how much was lost.
//
// Stdout and Stderr are both set to one of these, which os/exec serves from
// a single pipe and a single goroutine when the two are the same writer -
// the same arrangement CombinedOutput makes, so nothing here needs a lock.
type boundedOutput struct {
	kept    []byte
	dropped int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.kept = append(b.kept, p...)
	if excess := len(b.kept) - KeptBytes; excess > 0 {
		b.dropped += excess
		b.kept = b.kept[:copy(b.kept, b.kept[excess:])]
	}
	return len(p), nil
}

// String is everything still inside the window.
func (b *boundedOutput) String() string { return string(b.kept) }
