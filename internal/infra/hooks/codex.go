// Codex's hooks, which travel as `-c` flags rather than a settings file
// (#152). Codex reads hook declarations from every config layer, including
// the session-flags layer `-c` builds, but runs a new hook only once it is
// trusted, and trusting one writes to the user's ~/.codex/config.toml. The
// trust record is read from the session-flags layer too, so omatty passes it
// beside the hooks: the hash codex would have stored, computed here. Nothing
// is written to $CODEX_HOME (invariant 3). The research is
// docs/research/agents/codex.md (#528).

package hooks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

// codexSessionFlags is the source codex keys a `-c` hook's trust under.
const codexSessionFlags = "/<session-flags>/config.toml"

// codexHookTimeout is invariant 11's 5 s, as for claude's hooks.json.
const codexHookTimeout = 5

// codexClampedTimeout is what codex clamps Interrupt and SessionEnd to; it
// hashes the clamped value, so a hook declared longer is never trusted.
const codexClampedTimeout = 3

// RenderCodexArgs is the `-c` arguments that declare one `omatty hook
// --agent codex` per event, and their trust. It is the codex profile's
// RenderArgs.
//
//	args, err := hooks.RenderCodexArgs("/usr/local/bin/omatty", []string{"Stop"})
func RenderCodexArgs(binPath string, eventNames []string) ([]string, error) {
	line := HookCommand(binPath, "codex")
	quoted, err := tomlString(line)
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, 2*len(eventNames)+2)
	trust := make([]string, 0, len(eventNames))
	for _, event := range eventNames {
		timeout := codexTimeout(event)
		args = append(args, "-c", fmt.Sprintf(`hooks.%s=[{hooks=[{type="command",command=%s,timeout=%d}]}]`, event, quoted, timeout))
		trust = append(trust, fmt.Sprintf(`"%s:%s:0:0"={trusted_hash="%s"}`, codexSessionFlags, snakeCase(event), CodexTrustHash(event, line, timeout)))
	}
	return append(args, "-c", "hooks.state={"+strings.Join(trust, ",")+"}"), nil
}

// codexTimeout is the timeout codex will normalise an event's hook to.
func codexTimeout(event string) int {
	if event == "Interrupt" || event == "SessionEnd" {
		return codexClampedTimeout
	}
	return codexHookTimeout
}

// codexHookIdentity is the normalised hook codex hashes for its trust
// record (codex-rs hooks/src/engine/discovery.rs, hook_hash). The fields are
// in key order because the hash is over sorted keys.
type codexHookIdentity struct {
	EventName string           `json:"event_name"`
	Hooks     []codexHookEntry `json:"hooks"`
}

type codexHookEntry struct {
	Async   bool   `json:"async"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
	Type    string `json:"type"`
}

// CodexTrustHash is the trusted_hash codex stores for a command hook on
// event: SHA-256 over the compact JSON of its normalised identity.
//
//	hooks.CodexTrustHash("Stop", line, 5) // "sha256:…"
func CodexTrustHash(event, command string, timeout int) string {
	id := codexHookIdentity{EventName: snakeCase(event),
		Hooks: []codexHookEntry{{Command: command, Timeout: timeout, Type: "command"}}}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// codex's serde writes '<', '>' and '&' as they are; Go's default
	// > would hash a different hook than the one codex reads.
	enc.SetEscapeHTML(false)
	_ = enc.Encode(id) // a struct of strings, ints and a bool cannot fail
	sum := sha256.Sum256(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// snakeCase is codex's label for an event in a trust key and a hash:
// "UserPromptSubmit" -> "user_prompt_submit".
func snakeCase(event string) string {
	var b strings.Builder
	for i, r := range event {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// tomlString quotes s as a TOML basic string. A JSON string is one, as long
// as '<', '>' and '&' are left alone, which keeps the hashed command and the
// declared one the same text.
func tomlString(s string) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return "", fmt.Errorf("quoting hook command %q for codex's -c: %w", s, err)
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

// ParseCodexPayload reads a codex hook payload, which has claude's field
// names, and also requires transcript_path. codex's background memories
// thread fires the pane's own hooks with transcript_path null, and its
// SessionStart would otherwise re-bind the row to a thread with no
// transcript (#152). The scanner refuses a routable field that is not a
// string, so null drops the payload here, before the socket.
//
//	p, ok := hooks.ParseCodexPayload(os.Stdin)
func ParseCodexPayload(stdin io.Reader) (status.HookPayload, bool) {
	var transcript string
	p, ok := parseRouted(stdin, func(name string, p *status.HookPayload) *string {
		if name == "transcript_path" {
			return &transcript
		}
		return routableField(name, p)
	})
	return p, ok && transcript != ""
}
