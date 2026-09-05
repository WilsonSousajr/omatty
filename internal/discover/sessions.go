// Adoption's half of discovery: the sessions inside one registered project
// that omatty does not yet track (#122).
//
// M4's Propose answers "which repositories have I used claude in". This answers
// "which claude sessions are in this repository", which is the question an
// operator has once the project is registered and the sessions they started
// outside omatty are not on the sidebar.
//
// Nothing here writes to the registry either: it proposes, and state.json stays
// the single source of truth (invariant 9).

package discover

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// maxTitleRunes bounds a proposed title. A row is one line of a pane that is
// about fifty columns on an eighty-column window, so a pasted essay would push
// the detail column off the screen entirely.
const maxTitleRunes = 60

// SessionCandidate is one claude session worth offering for adoption.
//
// Dir is the working directory the transcript recorded, not the project root:
// they differ for a session that ran in a linked worktree, and Dir is where the
// adopted session must actually be started.
type SessionCandidate struct {
	ID       string
	Title    string
	Dir      string
	LastUsed time.Time
}

// ProposeSessions returns the sessions in projectRoot that omatty does not
// already hold, most recently used first.
//
//	cands, err := discover.ProposeSessions(paths.TranscriptsDir(home), git, p.Root, ids)
//
// known is the session ids already in state.json, which are left out. Offering
// one again would make its row fail on commit with "already registered" and say
// nothing about why - the same reasoning that keeps registered roots out of
// project discovery (#91).
func ProposeSessions(
	storeRoot string, git Git, projectRoot string, known []string,
) ([]SessionCandidate, error) {
	entries, err := os.ReadDir(storeRoot)
	if err != nil {
		return nil, fmt.Errorf("discover: cannot read the transcript store %q: %w", storeRoot, err)
	}
	held := make(map[string]bool, len(known))
	for _, id := range known {
		held[id] = true
	}
	var found []SessionCandidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found = append(found, inProject(filepath.Join(storeRoot, e.Name()), git, projectRoot, held)...)
	}
	sort.Slice(found, func(a, b int) bool { return found[a].LastUsed.After(found[b].LastUsed) })
	return found, nil
}

// inProject is one slug directory's contribution: nothing at all unless the
// directory's recorded cwd resolves to projectRoot.
//
// The cwd is read once for the directory rather than once per transcript. Every
// transcript in a slug directory shares a working directory - that is what the
// slug is - so resolving per file would cost a git call each.
func inProject(slugDir string, git Git, projectRoot string, held map[string]bool) []SessionCandidate {
	found := transcripts(slugDir)
	if len(found) == 0 {
		return nil
	}
	dir, ok := firstCwd(found)
	if !ok {
		return nil
	}
	if root, ok := resolveRoot(dir, git); !ok || root != projectRoot {
		return nil
	}
	return candidatesIn(found, dir, held)
}

// candidatesIn turns each transcript in one directory into a candidate.
func candidatesIn(found []transcript, dir string, held map[string]bool) []SessionCandidate {
	out := make([]SessionCandidate, 0, len(found))
	for _, t := range found {
		id := sessionID(t.Path)
		if held[id] {
			continue
		}
		out = append(out, SessionCandidate{
			ID: id, Title: titleOf(t.Path, id), Dir: dir, LastUsed: t.Used,
		})
	}
	return out
}

// sessionID is the uuid claude named the transcript after, which is the id
// `--resume` takes.
func sessionID(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// titleOf labels a row with what the session was about, falling back to the id
// for a session that never got a typed prompt - which still exists and is still
// adoptable, so an empty cell would be worse than an unreadable one.
func titleOf(path, id string) string {
	if prompt := firstPrompt(path); prompt != "" {
		return prompt
	}
	return id
}

// promptRecord is the slice of a transcript line adoption reads.
//
// watcher.Entry cannot be reused, for the reason readCwd's own struct exists:
// it keeps whether an entry was a prompt but discards the text, which is
// precisely what a title needs. Parsing untyped input into a struct at the
// edge, once, is the rule - this is that struct.
type promptRecord struct {
	Type    string `json:"type"`
	IsMeta  bool   `json:"isMeta"`
	Message struct {
		// A prompt is a bare string; a tool result is a list of blocks. Only
		// the string form is a typed prompt, so anything else is skipped
		// rather than decoded into a shape a title cannot use.
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// firstPrompt is the first thing the operator actually typed, flattened for
// display, or "" when the head of the transcript holds none.
//
// The content is untrusted (AGENTS.md, Security): it is read to make a display
// string and for nothing else, and it is flattened before it can reach a
// rendered row.
func firstPrompt(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	scan := bufio.NewScanner(&io.LimitedReader{R: f, N: maxHeadBytes})
	scan.Buffer(nil, maxHeadBytes)
	for i := 0; i < maxHeadLines && scan.Scan(); i++ {
		if text, ok := typedPrompt(scan.Bytes()); ok {
			return flatten(text)
		}
	}
	if err := scan.Err(); err != nil {
		slog.Debug("adoption could not read a transcript head", "transcript", path, "err", err)
	}
	return ""
}

// typedPrompt reports the text of one line if it is a prompt the operator
// typed.
//
// Two things disqualify a user-role entry: isMeta, which is context claude
// injected, and a body claude wrote itself - a slash command, its output, a
// caveat. The head of a real transcript is mostly those, so without the check
// every session would be titled "<command-name>/clear" (#61, #122).
func typedPrompt(line []byte) (string, bool) {
	var rec promptRecord
	if json.Unmarshal(line, &rec) != nil || rec.Type != "user" || rec.IsMeta {
		return "", false
	}
	var text string
	if json.Unmarshal(rec.Message.Content, &text) != nil {
		return "", false // a list of blocks: a tool result, not a prompt
	}
	if text == "" || watcher.IsInjectedPrompt(text) {
		return "", false
	}
	return text, true
}

// flatten makes an untrusted prompt safe and short enough for one row: control
// characters become spaces, runs of whitespace collapse, and the result is cut
// to maxTitleRunes.
//
// The control-character pass is the security-relevant half. An escape sequence
// in a prompt would otherwise reach the renderer and could move the cursor or
// paint outside its cell.
func flatten(s string) string {
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return truncate(strings.Join(strings.Fields(mapped), " "))
}

// truncate cuts to maxTitleRunes on rune boundaries, marking that it did.
func truncate(s string) string {
	r := []rune(s)
	if len(r) <= maxTitleRunes {
		return s
	}
	return string(r[:maxTitleRunes]) + "…"
}
