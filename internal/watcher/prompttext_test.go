package watcher_test

import (
	"encoding/json"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// PromptText is the one copy of what "the operator typed this" means, and both
// discover and the tailer read it. It was exported for that reason (#122) and
// then went untested inside a package measuring 92.2%, which is the hole the
// C.R.A.P. gate exists to find (#262).
//
// The shapes are claude's own: a bare JSON string for a plain prompt, and a
// list of blocks when the prompt carried an attachment (#62) or when Claude
// Code wrote the entry itself (#61).
func TestPromptText(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
		wantOK  bool
	}{
		{
			name:    "a bare string is a typed prompt",
			content: `"fix the failing test"`,
			want:    "fix the failing test",
			wantOK:  true,
		},
		{
			name:    "a prompt with an attachment is a list of blocks",
			content: `[{"type":"text","text":"look at this"},{"type":"image"}]`,
			want:    "look at this",
			wantOK:  true,
		},
		{
			name:    "the first text block is the prompt even behind an image",
			content: `[{"type":"image"},{"type":"text","text":"and this one"}]`,
			want:    "and this one",
			wantOK:  true,
		},
		{
			name:    "a tool result is not a prompt",
			content: `[{"type":"tool_result","text":"exit status 1"}]`,
			wantOK:  false,
		},
		{
			name:    "an empty string is not a prompt",
			content: `""`,
			wantOK:  false,
		},
		{
			name:    "a list with no text block says nothing",
			content: `[{"type":"image"}]`,
			wantOK:  false,
		},
		{
			name:    "content that is neither shape says nothing",
			content: `{"role":"user"}`,
			wantOK:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := watcher.PromptText(json.RawMessage(c.content))

			if ok != c.wantOK || got != c.want {
				t.Errorf("PromptText(%s) = (%q, %v), want (%q, %v)", c.content, got, ok, c.want, c.wantOK)
			}
		})
	}
}

// The rule that keeps omatty's own injected text out of a session's status: a
// user-role entry Claude Code wrote itself is not a typed prompt, and must not
// move the session to thinking (#61) or title it (#122).
//
// Every prefix, in both content shapes, because discover once understood only
// the bare-string form and gave a uuid for a title to every session whose
// first prompt carried an attachment - which is why both shapes live in this
// one function.
func TestPromptText_injectedEntriesAreNotPrompts(t *testing.T) {
	injected := []string{
		"<task-notification>background task finished</task-notification>",
		"<command-name>/clear</command-name>",
		"<local-command-stdout>ok</local-command-stdout>",
	}
	for _, text := range injected {
		t.Run(text[:12], func(t *testing.T) {
			bare, err := json.Marshal(text)
			if err != nil {
				t.Fatal(err)
			}
			blocks, err := json.Marshal([]map[string]string{{"type": "text", "text": text}})
			if err != nil {
				t.Fatal(err)
			}

			for shape, content := range map[string]json.RawMessage{"string": bare, "blocks": blocks} {
				if got, ok := watcher.PromptText(content); ok {
					t.Errorf("PromptText(%s form) = (%q, true), want it refused as injected", shape, got)
				}
			}
		})
	}
}

// A prompt that merely mentions one of the markers is still a prompt: the test
// is a prefix, deliberately, because claude writes the marker at the start of
// the entry it generated and nowhere else.
func TestPromptText_aMarkerInsideTheTextIsStillAPrompt(t *testing.T) {
	const typed = "why does <command-name> show up in the transcript?"

	got, ok := watcher.PromptText(json.RawMessage(`"` + typed + `"`))

	if !ok || got != typed {
		t.Errorf("PromptText() = (%q, %v), want the typed prompt", got, ok)
	}
}
