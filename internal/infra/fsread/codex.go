package fsread

import (
	"path/filepath"
	"strings"
)

// CodexRollout is where codex wrote a conversation's rollout under its
// store (~/.codex, or $CODEX_HOME): sessions/<YYYY>/<MM>/<DD>/
// rollout-<launch time>-<id>.jsonl. The date and time are codex's own, so
// the file is found by the id it ends in; ok is false until codex has
// written it, which is at the conversation's first prompt (#152, #528).
//
//	path, ok := fsread.CodexRollout(filepath.Join(home, ".codex"), conversation)
func CodexRollout(store, conversation string) (string, bool) {
	if conversation == "" || strings.ContainsAny(conversation, `*?[\/`) {
		return "", false // an id, never a pattern or a path
	}
	matches, _ := filepath.Glob(filepath.Join(store, "sessions", "*", "*", "*", "rollout-*-"+conversation+".jsonl"))
	if len(matches) == 0 {
		return "", false
	}
	return matches[len(matches)-1], true
}
