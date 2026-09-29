package forge

import "strconv"

// Label is how the UI names a forge and its unit of change, so no copy in the
// window hard-codes GitHub's words (#449): GitLab says "merge request" and
// writes it "!12", and everything else says "pull request" and "#12".
//
//	forge.GitHub.Ref(349) // "#349"
type Label struct {
	// Forge is the display name, "GitHub"; empty when the forge is not known.
	Forge string
	// Change is the unit of change in prose, "pull request".
	Change string
	// Short is its abbreviation, "PR".
	Short string
	// Sigil is written before a change's number, "#".
	Sigil string
}

// GitHub is the copy omatty drew before it knew any other forge.
var GitHub = Label{Forge: "GitHub", Change: "pull request", Short: "PR", Sigil: "#"}

// Neutral is the label for a project whose forge is not known: GitHub's nouns,
// which read correctly on every forge but GitLab, and no forge to name.
var Neutral = Label{Change: GitHub.Change, Short: GitHub.Short, Sigil: GitHub.Sigil}

// Ref writes one change's number the way its forge does.
func (l Label) Ref(number int) string { return l.Sigil + strconv.Itoa(number) }
