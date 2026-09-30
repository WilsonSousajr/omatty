// Package gate schedules a project's gate for many sessions at once: the
// Runner bounds how many run together, supersedes a session's stale run, and
// keeps one session's panic to that session (invariant 6). Running a step is
// internal/infra/gateexec's business, injected as a RunFunc (ADR 0001,
// migration step 5.3, #653); the vocabulary - Step, Verdict, Report - is
// internal/domain/gate's.
//
//	r := gate.NewRunner(cfg.Gate.MaxParallel, gateexec.Run)
//	r.Start(sess.ID, sess.Dir, proj.Gate)
package gate
