// Package agentcli runs the agent's own binary headless for a one-shot answer
// - today a session's name, asked of its first prompt (#127 step 2) - under a
// timeout and outside the detach holder, so it dies with omatty. Running a
// binary is an adapter's business (ADR 0001, migration step 5.5, #653).
//
//	n := agentcli.NewNamer(agentcli.NamerOpts{Bin: cfg.ClaudeBin})
//	defer n.Close()
//	title, err := n.Name(ctx, firstPrompt)
package agentcli
