// What a session may be called: the placeholder it carries before its first
// prompt names it (#127).

package registry

import "strings"

// placeholderCells is how much of the uuid a placeholder title keeps.
const placeholderCells = 8

// PlaceholderTitle is the name a session carries until its first prompt names
// it: the first eight characters of its uuid. Enough to tell two rows apart,
// short enough not to take the sidebar's whole title column, and the same
// idea discover.titleOf already falls back to for an adopted session (#122,
// #127).
//
//	registry.PlaceholderTitle("abc12345-6789-...") // "abc12345"
func PlaceholderTitle(id string) string {
	if len(id) <= placeholderCells {
		return id
	}
	return id[:placeholderCells]
}

// slugCells caps a slug. Forty: a branch name that still fits a sidebar
// title column and a PR title.
const slugCells = 40

// Slug reduces untrusted text to a name that is safe as a title now and as a
// branch and a path later: lower case, [a-z0-9-] only, runs of '-' collapsed,
// no leading or trailing '-', capped at slugCells. It returns "" when nothing
// survives, which every caller reads as "keep the name you had". One filter,
// applied to model output and to operator input alike: a model-produced name
// is untrusted twice over and passes the same filter as a typed one, never a
// looser one (#127).
//
//	registry.Slug("Fix the horizontal wheel pan!") // "fix-the-horizontal-wheel-pan"
//	registry.Slug("../../etc/passwd")              // "etc-passwd"
func Slug(s string) string {
	out := strings.TrimRight(slugRunes(s), "-")
	if len(out) > slugCells {
		out = strings.TrimRight(out[:slugCells], "-")
	}
	return out
}

// slugRunes keeps [a-z0-9] and turns every other run into one dash, writing
// no leading dash. Every byte written is ASCII, so len is cells.
func slugRunes(s string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	return b.String()
}
