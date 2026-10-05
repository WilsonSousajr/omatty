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
	pattern := filepath.Join(globLiteral(store), "sessions", "*", "*", "*", "rollout-*-"+conversation+".jsonl")
	matches, _ := filepath.Glob(pattern)
	if len(matches) == 0 {
		return "", false
	}
	return matches[len(matches)-1], true
}

// globLiteral escapes path so filepath.Glob matches it as written: a store
// under a directory named "[work]" is not a character class (#152's review).
func globLiteral(path string) string {
	var b strings.Builder
	for _, r := range path {
		if strings.ContainsRune(`*?[\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
