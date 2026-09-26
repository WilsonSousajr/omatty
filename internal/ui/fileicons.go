// File-type icons in the tree, opt-in (#431).
//
// Only with [ui] icons = "nerd" (#425): a Nerd Font glyph in a terminal
// without one is a tofu box, worse than no icon, which is why M5 and M8 cut
// icons and M15 takes them back only behind the key. With the key unset the
// tree is drawn exactly as before - treeText gets an empty icon.

package ui

import (
	"path"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// Folder and fallback glyphs, Font Awesome's as every Nerd Font patches them.
const (
	iconFolderOpen = ""
	iconFolderShut = ""
	iconFile       = ""
)

// iconsByName are files known by their whole name, which outrank their
// extension: go.sum is Go's, not an unknown ".sum".
var iconsByName = map[string]string{
	"go.mod": "", "go.sum": "", "Makefile": "", "Dockerfile": "",
	".gitignore": "", "LICENSE": "",
}

// iconsByExt are Seti's and Devicons' glyphs for the extensions a repository
// like this one holds; anything else takes iconFile.
var iconsByExt = map[string]string{
	".go": "", ".md": "", ".json": "", ".toml": "",
	".yml": "", ".yaml": "", ".sh": "", ".py": "",
	".js": "", ".ts": "", ".rs": "", ".html": "", ".css": "",
}

// treeIcon is n's glyph and a space, or "" unless Nerd Font icons are on.
func (m *Model) treeIcon(n review.TreeNode, collapsed bool) string {
	if !m.nerdIcons {
		return ""
	}
	return fileIcon(n, collapsed) + " "
}

// fileIcon is the glyph for a row: a folder open or shut, a file by its name,
// then by its extension, then the fallback.
func fileIcon(n review.TreeNode, collapsed bool) string {
	if n.IsDir && collapsed {
		return iconFolderShut
	}
	if n.IsDir {
		return iconFolderOpen
	}
	if icon, ok := iconsByName[n.Name]; ok {
		return icon
	}
	if icon, ok := iconsByExt[strings.ToLower(path.Ext(n.Name))]; ok {
		return icon
	}
	return iconFile
}
