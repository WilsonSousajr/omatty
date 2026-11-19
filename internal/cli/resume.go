// `omatty --resume <conversation>`: the command claude's exit hint names once
// the launcher sets CLAUDE_CODE_RESUME_HINT_COMMAND (#815). This half only
// resolves the conversation to its row; cmd opens the TUI on it.

package cli

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// ResumeTarget is the ID of the row that owns the conversation in args, which
// must be exactly that one id. A conversation no row owns is refused with the
// id and the way to register it, rather than opening an omatty that does not
// show it.
//
//	id, err := cli.ResumeTarget(st, []string{"3f2a…"})
func ResumeTarget(st session.State, args []string) (string, error) {
	if len(args) != 1 || args[0] == "" {
		return "", fmt.Errorf("omatty --resume: got %d arguments %q, want one conversation id", len(args), args)
	}
	sess, ok := st.SessionByConversation(args[0])
	if !ok {
		return "", fmt.Errorf("omatty --resume: no session owns conversation %q "+
			"(a conversation omatty did not start is registered with `omatty adopt`)", args[0])
	}
	return sess.ID, nil
}
