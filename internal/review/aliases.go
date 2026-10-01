package review

import (
	"io"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/infra/gitdiff"
)

// The review model - a diff as files, hunks and lines; comments anchored on
// content (invariant 7); the tree; the prompt Compose writes - moved to
// internal/domain/review (ADR 0001, migration step 3.6a). These aliases keep
// every importer compiling while the importers move over; migration step 8.1
// deletes them. New code imports internal/domain/review directly:
//
//	import "github.com/WilsonSousajr/omatty/internal/domain/review"
//	prompt := review.Compose(diff, comments)

// Anchor is dreview.Anchor.
type Anchor = dreview.Anchor

// Change is dreview.Change.
type Change = dreview.Change

// Comment is dreview.Comment.
type Comment = dreview.Comment

// Comments is dreview.Comments.
type Comments = dreview.Comments

// EntryKind is dreview.EntryKind.
type EntryKind = dreview.EntryKind

// Entry is dreview.Entry.
type Entry = dreview.Entry

// Pairing is dreview.Pairing.
type Pairing = dreview.Pairing

// Placed is dreview.Placed.
type Placed = dreview.Placed

// LineKind is dreview.LineKind.
type LineKind = dreview.LineKind

// Line is dreview.Line.
type Line = dreview.Line

// Hunk is dreview.Hunk.
type Hunk = dreview.Hunk

// FileStatus is dreview.FileStatus.
type FileStatus = dreview.FileStatus

// File is dreview.File.
type File = dreview.File

// Diff is dreview.Diff.
type Diff = dreview.Diff

// Position is dreview.Position.
type Position = dreview.Position

// TreeNode is dreview.TreeNode.
type TreeNode = dreview.TreeNode

// Preview is dreview.Preview (migration step 5.8, #653).
type Preview = dreview.Preview

// Tree is dreview.Tree.
type Tree = dreview.Tree

// The model's enumerations, as dreview declares them.
const (
	ChangeNone      = dreview.ChangeNone
	ChangeModified  = dreview.ChangeModified
	ChangeAdded     = dreview.ChangeAdded
	ChangeDeleted   = dreview.ChangeDeleted
	ChangeRenamed   = dreview.ChangeRenamed
	EntryFile       = dreview.EntryFile
	EntryOrphan     = dreview.EntryOrphan
	EntryHunk       = dreview.EntryHunk
	EntryLine       = dreview.EntryLine
	EntryComment    = dreview.EntryComment
	PairingNone     = dreview.PairingNone
	PairingPaired   = dreview.PairingPaired
	PairingUnpaired = dreview.PairingUnpaired
	PairingUnknown  = dreview.PairingUnknown
	LineContext     = dreview.LineContext
	LineAdded       = dreview.LineAdded
	LineRemoved     = dreview.LineRemoved
	FileModified    = dreview.FileModified
	FileAdded       = dreview.FileAdded
	FileDeleted     = dreview.FileDeleted
	FileRenamed     = dreview.FileRenamed
)

// LineHash is dreview.LineHash.
//
//	x := review.LineHash(...)
func LineHash(l Line) string { return dreview.LineHash(l) }

// AnchorAt is dreview.AnchorAt.
//
//	x := review.AnchorAt(...)
func AnchorAt(d Diff, p Position) Anchor { return dreview.AnchorAt(d, p) }

// ChangeOf is dreview.ChangeOf.
//
//	x := review.ChangeOf(...)
func ChangeOf(s FileStatus) Change { return dreview.ChangeOf(s) }

// NewComments is dreview.NewComments.
//
//	x := review.NewComments(...)
func NewComments() *Comments { return dreview.NewComments() }

// Compose is dreview.Compose.
//
//	x := review.Compose(...)
func Compose(d Diff, comments []Comment) string { return dreview.Compose(d, comments) }

// FileDigest is dreview.FileDigest.
//
//	x := review.FileDigest(...)
func FileDigest(f File) string { return dreview.FileDigest(f) }

// Flatten is dreview.Flatten.
//
//	x := review.Flatten(...)
func Flatten(d Diff, p Placed) []Entry { return dreview.Flatten(d, p) }

// Pair is dreview.Pair.
//
//	x := review.Pair(...)
func Pair(d Diff) Pairing { return dreview.Pair(d) }

// Place is dreview.Place.
//
//	x := review.Place(...)
func Place(d Diff, comments []Comment) Placed { return dreview.Place(d, comments) }

// NewTree is dreview.NewTree.
//
//	x := review.NewTree(...)
func NewTree(paths []string, changes map[string]Change) *Tree { return dreview.NewTree(paths, changes) }

// AnchorFor is dreview.AnchorFor.
//
//	x := review.AnchorFor(...)
func AnchorFor(session, turn Diff, p Position) Anchor { return dreview.AnchorFor(session, turn, p) }

// PlaceIn is dreview.PlaceIn.
//
//	x := review.PlaceIn(...)
func PlaceIn(session, turn Diff, comments []Comment) Placed {
	return dreview.PlaceIn(session, turn, comments)
}

// ParseDiff is gitdiff.ParseDiff: parsing moved to internal/infra/gitdiff
// (migration step 3.6b), the one package that imports go-gitdiff.
//
//	d, err := review.ParseDiff(strings.NewReader(raw))
func ParseDiff(r io.Reader) (Diff, error) { return gitdiff.ParseDiff(r) }
