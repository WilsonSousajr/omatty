// Package review loads a session's diff and turn - through git, parsed by
// go-gitdiff - reads previews and .gitattributes, and reverts. The model it
// loads into (files, hunks, content anchors, the prompt) is
// internal/domain/review; the loading moves to service/review and infra in
// ADR 0001's migration (steps 3.6b, 3.6c and 5.8).
package review
