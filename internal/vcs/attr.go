package vcs

import (
	"bytes"
	"os/exec"
	"strings"
)

// Attr reports, for each of paths, whether git resolves attr to "set" on it.
// One process for the whole list, whatever its length, because a tree listing
// is thousands of paths and a call each would be thousands of forks (#338).
//
//	gen, err := vcs.NewCLI().Attr(sess.Dir, "linguist-generated", paths)
//
// Asked of git rather than parsed here: `.gitattributes` exists at every level
// of a tree, plus `.git/info/attributes` and the global file, with later rules
// overriding earlier ones. git is the only authority on what a name resolves
// to, and a reimplementation would be right most of the time - which for a
// rule that hides files from a review is the wrong kind of nearly.
//
// A path git says nothing about is absent from the map rather than false, so
// "unspecified" and "explicitly unset" both read as not set, which is what
// every caller means.
func (c *CLI) Attr(dir, attr string, paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return map[string]bool{}, nil
	}
	// -z makes the input NUL-separated as well as the output. Feeding
	// newline-separated paths to `--stdin -z` is not an error: git reads the
	// whole list as one path name and reports it unspecified, so every file
	// comes back not-generated and nothing says why.
	out, err := c.captureStdin(dir, strings.Join(paths, "\x00")+"\x00",
		"check-attr", "--stdin", "-z", attr)
	if err != nil {
		return nil, err
	}
	return foldAttr(out, attr), nil
}

// foldAttr reads check-attr's NUL-separated output: path, attribute, value,
// repeating.
//
// -z rather than the line format because a path may contain a space, a colon
// or a newline, and the line format `<path>: <attr>: <value>` cannot be split
// back apart when it does. That is the whole reason for the flag: a file named
// "my docs/note one.md" is a file, not three fields.
func foldAttr(out, attr string) map[string]bool {
	fields := strings.Split(out, "\x00")
	set := map[string]bool{}
	for i := 0; i+2 < len(fields); i += 3 {
		if fields[i+1] == attr && attrIsSet(fields[i+2]) {
			set[fields[i]] = true
		}
	}
	return set
}

// attrIsSet reads check-attr's value column. Both spellings occur and mean the
// same thing: a bare `linguist-generated` in .gitattributes reports "set",
// while `linguist-generated=true` - which is the form GitHub documents, and so
// the form most repositories actually carry - reports "true". Accepting only
// one of them would silently miss most of the repositories this exists for.
//
// Everything else is a no: "unspecified" (no rule matched), "unset" (a rule
// turned it off with a leading -), and any other value a project invented.
func attrIsSet(value string) bool { return value == "set" || value == "true" }

// captureStdin runs git with stdin fed from in. Only check-attr --stdin needs
// it, and it stays beside its one caller rather than joining capture's family
// in git.go, where every other member is shared.
func (c *CLI) captureStdin(dir, in string, args ...string) (string, error) {
	if err := checkDir(dir); err != nil {
		return "", err
	}
	cmd := exec.Command(c.bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", &CommandError{Args: args, Dir: dir, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return string(out), nil
}
