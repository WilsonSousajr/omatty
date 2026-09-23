// The diff's file header and how it fits its column (#291): the fourth of the
// review column's fitting rules, and a fourth because the first three do not
// describe it.
//
// #283 gives up whole parts by priority, which suits counts. #285 shrinks a
// name from the middle, which suits a session's name. #287 shortens a path
// from the front, which suits a path - and that one is reused here unchanged,
// because the header names a file for the same reason the preview title does.
// What is new is the priority, and it is new because the header's parts are
// not the diff title's:
//
//   - "N uncovered" is never given up. It is the finding - the whole argument
//     for the count (#255) is that a long diff should say where to look
//     without being scrolled - and it was the first thing to go.
//   - the path shortens from the front, keeping the filename.
//   - "+13 -0" is given up last, and only when keeping it would cost the
//     filename itself. The diff body shows added and removed lines directly,
//     so the diffstat is the one part the screen repeats; but three cells are
//     cheap and a shortened path costs legibility, so it is not given up while
//     the name still fits beside it. Measured at the real column widths - 23
//     cells at the default window, 27, 35, 43, 51 - giving the diffstat up
//     first would have cost it at 120 columns and shortened the path anyway.

package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// renameArrow joins the two names of a renamed file, and binaryNote is what a
// file whose lines nobody can read says instead of a diffstat.
const renameArrow, binaryNote = " → ", " (binary)"

// fileHeading is the file's row in the diff: "path +a -b", with both names for
// a rename, a note for a binary whose lines nobody can read, and note - the
// count of added lines no test covers (#255) - fitted to budget in the order
// this file's comment gives.
//
// A binary's "(binary)" takes the diffstat's place and is never given up: it
// is why that file has no body, which is this row's whole finding.
func fileHeading(f review.File, note string, budget int) string {
	if f.Binary {
		return headingName(f, budget-lipgloss.Width(binaryNote)) + binaryNote
	}
	a, r := f.Counts()
	return fitHeading(f, headingStat(a, r), note, budget)
}

// fitHeading lays the name, the diffstat and the count on budget cells. The
// count is subtracted first because it is never given up; the diffstat is kept
// only while the name still fits beside it.
//
// When even the count alone will not fit there is nothing left to give, and
// the row is cut by the caller's fitLine - the same place #283's rule ends.
func fitHeading(f review.File, stat, note string, budget int) string {
	room := budget - lipgloss.Width(note)
	if minName(f) <= room-lipgloss.Width(stat) {
		return headingName(f, room-lipgloss.Width(stat)) + stat + note
	}
	return headingName(f, room) + note
}

// headingName is the file's name in budget cells: one path shortened from the
// front, or both names for a rename.
//
// A rename says "this became that", so both names shorten together and neither
// is spent on the other's directories: the old name is offered half the room
// and whatever it does not use falls to the new one, which is therefore never
// the worse off of the two. Only when the pair cannot hold both filenames does
// the old name give up entirely, to a bare "…" - it still says the file came
// from somewhere, and dropping it would make a rename read as an ordinary
// edit.
func headingName(f review.File, budget int) string {
	if f.Status != review.FileRenamed {
		return previewTitle(f.Path, budget)
	}
	room := budget - lipgloss.Width(renameArrow)
	if room < lipgloss.Width(fileName(f.OldPath))+lipgloss.Width(fileName(f.Path)) {
		return "…" + renameArrow + previewTitle(f.Path, room-1)
	}
	old := previewTitle(f.OldPath, room/2)
	return old + renameArrow + previewTitle(f.Path, room-lipgloss.Width(old))
}

// minName is the fewest cells headingName can draw without giving up the
// filename, which is previewTitle's last rung before it elides the name
// itself. It is what the diffstat is weighed against.
func minName(f review.File) int {
	n := lipgloss.Width(fileName(f.Path))
	if f.Status == review.FileRenamed {
		n += lipgloss.Width("…" + renameArrow)
	}
	return n
}

// fileName is the last segment of a repository-relative path, which always
// uses forward slashes whatever the host filesystem does.
func fileName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// headingStat is " +a -b", the part of the header given up last. Plain text,
// unlike the card's styled diffstat: a header row is cut and panned by cell
// count, and SGR inside it would be measured wrong.
func headingStat(added, removed int) string {
	return " +" + strconv.Itoa(added) + " -" + strconv.Itoa(removed)
}
