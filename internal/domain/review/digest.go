package review

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// FileDigest is a content hash of everything the session did to one file: its
// status, each hunk's header, and each line's own LineHash. Two diffs of a
// file digest the same when the change is the same, and differently the moment
// any of it moves - which is what "changed since I reviewed it" means (#337).
//
//	if FileDigest(f) != readAt[f.Path] { /* it changed since you read it */ }
//
// Content, not an mtime and not a line number, for invariant 7's reason: the
// agent rewrites the file while you are reading it, so a timestamp says only
// that it was written and a line number says nothing at all. LineHash is
// reused rather than re-derived so a line's identity means one thing in this
// package.
//
// It deliberately parts company with Anchor here: Place's resolve() walks past
// a moved hunk header on purpose, because a comment should follow its line, but
// a header that moved means something above it in this file changed, and that
// is exactly what the reviewer has not read yet.
func FileDigest(f File) string {
	// Appended and hashed once, the way LineHash does it, rather than written
	// into a running hash: a Writer's error return has to be handled, and
	// there is no error a hash can return.
	b := fmt.Appendf(nil, "%d\x00%s\x00%s\x00", f.Status, f.Path, f.OldPath)
	for _, h := range f.Hunks {
		b = fmt.Appendf(b, "%s\x00", h.Header)
		for _, l := range h.Lines {
			b = fmt.Appendf(b, "%s\x00", LineHash(l))
		}
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}
