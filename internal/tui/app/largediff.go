// A large diff stays quick to review (#773).
//
// The preview stops at 256 KiB; the diff had no cap, so a regenerated
// go.sum, a vendored directory or a 20k-line fixture went through chroma line
// by line and buried the files around it. Such a file now arrives folded to
// its header, which says how large it is and that enter loads it - GitHub's
// "Large diffs are not rendered by default" - and so does a generated one,
// as #338 folds it in the tree.

package app

import dreview "github.com/WilsonSousajr/omatty/internal/domain/review"

// largeFileLines is the most diff lines a file arrives open with.
const largeFileLines = 2000

// diffLines is how many lines f's hunks draw.
func diffLines(f dreview.File) int {
	n := 0
	for _, h := range f.Hunks {
		n += len(h.Lines)
	}
	return n
}

// foldByDefault folds each file of d the operator has not yet folded or
// opened that is large or generated. A file the operator opened stays open
// through every reload: FoldedFiles holding its path at all is the decision.
func (m *Model) foldByDefault(d dreview.Diff) {
	for _, f := range d.Files {
		if _, decided := m.review.FoldedFiles[f.Path]; decided || !m.foldsByDefault(f) {
			continue
		}
		if m.review.FoldedFiles == nil {
			m.review.FoldedFiles = map[string]bool{}
		}
		m.review.FoldedFiles[f.Path] = true
	}
}

// foldsByDefault is whether f arrives folded: too large to draw at once, or
// generated.
func (m *Model) foldsByDefault(f dreview.File) bool {
	return diffLines(f) > largeFileLines || m.isGenerated(f.Path)
}
