package agentcli

import "os/exec"

// Installed reports whether bin resolves to an executable, as a bare name on
// PATH or as a path. ctrl+o n offers only an agent it can start (#524). It is
// here because os/exec is a capability only the allowlisted packages hold,
// and this one already runs the agent's binary.
//
//	ok := agentcli.Installed("codex")
func Installed(bin string) bool {
	if bin == "" {
		return false
	}
	_, err := exec.LookPath(bin)
	return err == nil
}
