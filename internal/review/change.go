package review

// Change is the kind of change a session made to a tree row: the mark the
// file tree draws beside it, and the hue it draws it in (#196). It is a
// separate type from FileStatus because a directory carries one too, rolled
// up from whatever changed beneath it, and a directory has no diff status.
//
//	changes := map[string]review.Change{"a.go": review.ChangeAdded}
type Change int

// The kinds, in the order the letters read: none, M, A, D, R.
const (
	ChangeNone Change = iota
	ChangeModified
	ChangeAdded
	ChangeDeleted
	ChangeRenamed
)

// ChangeOf maps a diff's file status to the tree's mark. The diff omatty
// already loads carries the status for every touched file, and untracked
// files arrive from it as all-addition diffs, so no second git call is
// needed to know what kind of change a row is (#196).
//
//	review.ChangeOf(review.FileDeleted) // review.ChangeDeleted
func ChangeOf(s FileStatus) Change {
	switch s {
	case FileAdded:
		return ChangeAdded
	case FileDeleted:
		return ChangeDeleted
	case FileRenamed:
		return ChangeRenamed
	default:
		return ChangeModified
	}
}
