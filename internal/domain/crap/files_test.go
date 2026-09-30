package crap_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/crap"
)

// osFiles is the real filesystem, as tools/crapcheck supplies it.
var osFiles = crap.Files{
	Read: os.ReadFile,
	ModTime: func(path string) (time.Time, error) {
		info, err := os.Stat(path)
		if err != nil {
			return time.Time{}, err
		}
		return info.ModTime(), nil
	},
}

// crap reads through the Files it is given and never the disk itself, so it
// stays pure (ADR 0001, migration step 3.9). A package whose file exists only
// in memory is scored.
func TestScores_readsThroughTheFilesItIsGiven_issue635(t *testing.T) {
	mem := crap.Files{
		Read: func(path string) ([]byte, error) {
			if path != "/mem/f.go" {
				return nil, errors.New("not in memory: " + path)
			}
			return []byte("package f\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn x\n\t}\n\treturn 0\n}\n"), nil
		},
	}
	pkgs := []crap.Package{{ImportPath: "m/f", Dir: "/mem", GoFiles: []string{"f.go"}}}

	scores, err := crap.Scores("m", pkgs, nil, mem)

	if err != nil || len(scores) != 1 || scores[0].Name != "F" || scores[0].Complexity != 2 {
		t.Fatalf("Scores over an in-memory file = %+v, %v; want F at complexity 2", scores, err)
	}
}
