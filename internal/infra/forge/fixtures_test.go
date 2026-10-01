package forge_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// emailShaped is anything that reads as an address. An ssh clone URL's
// git@host is the one shape that is not a person.
var (
	emailShaped = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,}`)
	cloneUser   = regexp.MustCompile(`^git@`)
)

// The recorded fixtures are public data with every address taken out:
// AGENTS.md allows no emails in testdata, and each fixture README says so
// (#588).
func TestFixtures_CarryNoEmailAddress_issue588(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "forge")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, addr := range emailShaped.FindAllString(string(b), -1) {
			if !cloneUser.MatchString(addr) {
				t.Errorf("%s carries an email address: %q", path, addr)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
