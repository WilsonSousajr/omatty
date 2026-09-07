// What a session may be called: the placeholder it carries before its first
// prompt names it (#127).

package registry

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
