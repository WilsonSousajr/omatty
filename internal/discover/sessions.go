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

	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// maxTitleCells bounds a proposed title in display columns. A row is one line
// of a pane that is about fifty columns on an eighty-column window, so a pasted
// essay would push the detail column off the screen entirely.
const maxTitleCells = 60

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
	want, held := resolvedRoot(projectRoot, git), idSet(known)
	var found []SessionCandidate
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		found = append(found, inProject(filepath.Join(storeRoot, e.Name()), git, want, held)...)
	}
	sortSessions(found)
	return found, nil
}

// idSet is the ids to leave out, as a set.
func idSet(known []string) map[string]bool {
	held := make(map[string]bool, len(known))
	for _, id := range known {
		held[id] = true
	}
	return held
}

// sortSessions orders candidates newest first, breaking ties by id so the list
// is stable between runs - the guarantee sorted() makes for projects, and one
// adoption needs just as much. Two transcripts written in the same second is
// ordinary, claude opens sessions back to back, and byCwd's map iteration is
// unordered on top of that; without a tie-break the row under the cursor when
// the operator presses enter need not be the row they were looking at.
func sortSessions(found []SessionCandidate) {
	sort.Slice(found, func(a, b int) bool {
		if found[a].LastUsed.Equal(found[b].LastUsed) {
			return found[a].ID < found[b].ID
		}
		return found[a].LastUsed.After(found[b].LastUsed)
	})
}

// resolvedRoot is the project root as resolveRoot would report it, which is the
// form every transcript's cwd is compared against.
//
// Both sides have to be resolved the same way or the comparison is between two
// different things. registry.AddProject stores what `rev-parse --show-toplevel`
// returned, and inside a linked worktree that is the worktree itself; resolveRoot
// folds a worktree back to the repository it was forked from. A project
// registered from inside a worktree therefore matched no transcript at all, and
// adoption reported "no unregistered sessions in this project" forever - for
// the sessions that ran in that very directory (#91, #122).
//
// A root git cannot resolve is used as it stands: that is a project whose
// directory has been deleted or is not a repository, and the comparison below
// will simply match nothing, which is the honest answer.
func resolvedRoot(projectRoot string, git Git) string {
	if root, ok := resolveRoot(projectRoot, git); ok {
		return root
	}
	return projectRoot
}

// inProject is one slug directory's contribution: the transcripts whose own
// working directory resolves to projectRoot.
//
// One git call per distinct working directory, which is normally one for the
// whole slug directory - that is what a slug is.
func inProject(slugDir string, git Git, projectRoot string, held map[string]bool) []SessionCandidate {
	var out []SessionCandidate
	for dir, group := range byCwd(transcripts(slugDir)) {
		if root, ok := resolveRoot(dir, git); !ok || root != projectRoot {
			continue
		}
		out = append(out, candidatesIn(group, dir, held)...)
	}
	return out
}

// byCwd groups a slug directory's transcripts by the working directory each one
// records, dropping any that names none.
//
// Normally every transcript in the directory answers with the same path and
// this is one group. But the slug is lossy - TranscriptSlug maps every
// non-alphanumeric to '-', so "/a/b-c" and "/a/b/c" both become "-a-b-c" and
// claude writes both directories' transcripts into one slug directory, which is
// what readCwd exists to work around. Reading a single cwd for the whole
// directory stamped the newest transcript's path onto all of them, so a session
// that ran in one repository was offered under another's project and registered
// with a Dir that is not where it ran - and Dir is the directory
// `claude --resume` is launched in (#91, #122).
func byCwd(found []transcript) map[string][]transcript {
	out := make(map[string][]transcript, 1)
	for _, t := range found {
		if dir, ok := readCwd(t.Path); ok {
			out[dir] = append(out[dir], t)
		}
	}
	return out
}

// candidatesIn turns each transcript in one working directory into a candidate.
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
		// A prompt is a bare string, or a list of blocks when it carries an
		// attachment. Which of those is a typed prompt is watcher.PromptText's
		// question, not this struct's.
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
	// The same cap readCwd hits, and the same argument: one record is routinely
	// hundreds of kilobytes, so a head that blows it stops the scan with
	// ErrTooLong - indistinguishable from "this transcript opens with no typed
	// prompt" unless it is said out loud. The row then falls back to the raw
	// uuid, and Debug is dropped by the default handler, so the operator saw an
	// unreadable row and no trace of why (#91, #122).
	if err := scan.Err(); err != nil {
		slog.Warn("adoption could not read a transcript head",
			"transcript", path, "byteCap", maxHeadBytes, "err", err)
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
	// watcher owns both content shapes and the injected-prefix test. Reading
	// only the string form here skipped every prompt that carried an
	// attachment - the list-of-blocks shape #62 exists for - so those sessions
	// fell back to the unreadable uuid titleOf is written to avoid (#61, #122).
	return watcher.PromptText(rec.Message.Content)
}

// flatten makes an untrusted prompt safe and short enough for one row:
// non-printing characters become spaces, runs of whitespace collapse, and the
// result is cut to maxTitleCells.
//
// This is the security-relevant half, and it outlives the picker: AdoptSession
// persists the title to state.json, so whatever survives here is drawn in a
// sidebar row on every run from then on.
//
// The test is unicode.IsGraphic rather than !unicode.IsControl. IsControl
// covers only Cc, which let the Cf format characters through - U+202E
// RIGHT-TO-LEFT OVERRIDE above all, which reverses the visible order of
// everything after it, and the zero-width joiners with it.
func flatten(s string) string {
	mapped := strings.Map(func(r rune) rune {
		if !unicode.IsGraphic(r) {
			return ' '
		}
		return r
	}, s)
	return truncate(strings.Join(strings.Fields(mapped), " "))
}

// truncate cuts to maxTitleCells, marking that it did.
//
// Display cells, not runes: the bound exists because a row is one line of a
// fifty-column pane, and sixty CJK runes is a hundred and twenty columns - so
// counting runes let through exactly the overflow the constant was written to
// prevent, and fitLine clipped the row instead.
func truncate(s string) string {
	if runewidth.StringWidth(s) <= maxTitleCells {
		return s
	}
	return runewidth.Truncate(s, maxTitleCells, "…")
}
