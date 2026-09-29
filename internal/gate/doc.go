// Package gate runs a project's own verification commands and says, per step,
// whether they passed.
//
// A gate is the line a project already has in its contributing guide - gofmt,
// vet, lint, the test suite, a coverage threshold. omatty does not invent it,
// interpret it, or decide what belongs in it; it runs what the project says
// and reports the outcome next to the session that caused it.
//
// Invariant 12 is the rule this package exists to keep: a step passes if and
// only if its process exits 0. Nothing here reads a step's output to decide
// pass or fail, because output moves with tool version, locale, colour and
// verbosity while the exit code is the fact the tool is asserting. Output is
// carried for a human to read and for Compose to send back, never consulted.
//
// The one place that costs something is a tool that is not installed. Steps
// run under `sh -c`, since gate lines carry pipes, arguments and script paths,
// so exec.Cmd.Err never fires - sh exists even when the tool does not - and an
// absent tool arrives as the shell's exit 127. That is a convention rather
// than a guarantee, and a real command may exit 127 for its own reasons, so
// this package does not read it as one. It resolves a step's leading word with
// exec.LookPath before running anything and reports Missing instead. Telling a
// session its lint is failing when golangci-lint is merely absent would send
// it off to fix code that was never broken.
package gate
