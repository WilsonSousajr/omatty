// Package status derives each session's live status from two sources: hook
// events over a unix socket (fast, and the only source that can tell "waiting
// for you" from "tool running") and the transcript JSONL (the truth on
// attach, self-healing, and the only source of age and tokens).
//
// Invariant 2: status comes from these structured sources, never from the
// rendered terminal.
package status
