package forge

import (
	"errors"
	"strings"
	"testing"
)

// errExit stands in for the process error beneath a failed CLI.
var errExit = errors.New("exit status 1")

// glab's and tea's failures carry their words the way gh's do: as Detail,
// cleaned (#804).
func TestForge_glabAndTeaFailureDetailIsTheirWords_issue804(t *testing.T) {
	_, teaErr := teaVerdict("gitea.example", nil, "tea: connection refused\x1b[2J", errExit)
	for name, err := range map[string]error{
		"glab": apiFailure("gitlab.example", "glab", "glab: timeout\x1b[2J while dialing", errExit),
		"tea":  teaErr,
	} {
		var cli *CLIError
		if !errors.As(err, &cli) {
			t.Fatalf("%s: error = %v, want a *CLIError", name, err)
		}
		if strings.ContainsRune(cli.Detail(), '\x1b') || !strings.Contains(cli.Detail(), name+": ") {
			t.Errorf("%s: Detail = %q, want its words, cleaned", name, cli.Detail())
		}
		if !errors.Is(err, errExit) {
			t.Errorf("%s: the process error is not in the chain: %v", name, err)
		}
	}
}

// With nothing on stderr, Detail is the process error rather than nothing.
func TestForge_aSilentFailureDetailIsTheProcessError_issue804(t *testing.T) {
	err := &CLIError{Call: "gh pr list", Where: `in "/p"`, Err: errExit}
	if err.Detail() != "exit status 1" {
		t.Errorf("Detail = %q, want the process error", err.Detail())
	}
}

// A glab answer sorted by its HTTP status carries its words cleaned too: the
// log is a surface as well (#804).
func TestForge_aStatusAnswerIsCleaned_issue804(t *testing.T) {
	err := apiFailure("gitlab.example", "glab", "glab: bad gateway\x1b]0;owned\x07 (HTTP 502)", errExit)
	if err == nil || strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Errorf("error = %q, want glab's words cleaned", err)
	}
}
