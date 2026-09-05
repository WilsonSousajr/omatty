// Package discover proposes repositories to register, read from Claude Code's
// own transcript store (#91).
//
// `omatty add <dir>` registers one directory at a time, typed from memory.
// Claude already records every directory you have run it in, so omatty can
// offer that list instead. It only ever *proposes*: nothing here writes to the
// registry, so state.json stays the single source of truth (invariant 9).
//
// Transcript content is untrusted (AGENTS.md, Security). Only `cwd` is read
// out of it, and it is validated against the filesystem and against git before
// it is offered.
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
	"time"
)

// maxHeadLines and maxHeadBytes bound the read of one transcript.
//
// Measured against a real store: `cwd` appears by line 7 at the latest, but
// reaching it can cost 249 KB, because a single record - a pasted file, a tool
// result - is routinely hundreds of kilobytes. So the byte cap is generous and
// the line cap does the real work. An unbounded read here is issue #64.
const (
	maxHeadLines = 32
	maxHeadBytes = 1 << 20
)

// Candidate is a repository worth offering. Root is the main checkout, so two
// worktrees of one repository collapse into a single candidate.
type Candidate struct {
	Name     string
	Root     string
	LastUsed time.Time
}

// Git is the slice of vcs.Git discovery needs. It is declared here, not
// imported as the whole interface, so a fake in this package's tests carries
// one method rather than nine.
//
// One, not two: RepoRoot was declared here and never called, so every
// implementer paid for a method the package could not use (#91).
type Git interface {
	MainCheckout(dir string) (string, error)
}

// Propose reads the transcript store and returns the repositories still worth
// registering, most recently used first.
//
// Three filters do the work, and they matter: on the author's machine 34 slug
// directories collapse to 6 candidates. Eleven point at directories that have
// been deleted, five are not repositories, and the rest include worktrees that
// must fold into their parents.
//
// registered is the roots already in state.json, which are left out. Offering
// them again made every row of a second `omatty discover` fail on commit with
// "already registered", and nothing on screen said why - and the store only
// grows, so the second use of the feature is the common one (#91).
//
//	cands, err := discover.Propose(paths.TranscriptsDir(home), vcs.NewCLI(), roots)
func Propose(storeRoot string, git Git, registered []string) ([]Candidate, error) {
	entries, err := os.ReadDir(storeRoot)
	if err != nil {
		return nil, fmt.Errorf("discover: cannot read the transcript store %q: %w", storeRoot, err)
	}
	known := make(map[string]bool, len(registered))
	for _, root := range registered {
		known[root] = true
	}
	byRoot := map[string]Candidate{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		keepNewest(byRoot, filepath.Join(storeRoot, e.Name()), git, known)
	}
	return sorted(byRoot), nil
}

// keepNewest resolves one slug directory to a candidate and keeps whichever of
// the two is more recent, unless the root is already registered.
func keepNewest(byRoot map[string]Candidate, slugDir string, git Git, known map[string]bool) {
	cand, ok := candidateOf(slugDir, git)
	if !ok || known[cand.Root] {
		return
	}
	if seen, dup := byRoot[cand.Root]; dup && seen.LastUsed.After(cand.LastUsed) {
		return
	}
	byRoot[cand.Root] = cand
}

// transcript is one session file with the time it was last written.
type transcript struct {
	Path string
	Used time.Time
}

// candidateOf turns one slug directory into a candidate, applying the three
// filters: the directory must still exist, it must be a git repository, and a
// linked worktree resolves to the repository it was forked from.
func candidateOf(slugDir string, git Git) (Candidate, bool) {
	found := transcripts(slugDir)
	if len(found) == 0 {
		return Candidate{}, false
	}
	dir, ok := firstCwd(found)
	if !ok {
		return Candidate{}, false
	}
	root, ok := resolveRoot(dir, git)
	if !ok {
		return Candidate{}, false
	}
	// found[0] is the newest, which is when the directory was last worked in,
	// even where the cwd came from an older transcript.
	return Candidate{Name: filepath.Base(root), Root: root, LastUsed: found[0].Used}, true
}

// firstCwd is the working directory named by the newest transcript that names
// one at all.
//
// Reading only the newest file dropped the whole repository whenever that one
// was a session still being written, a stub, or one whose first record ran past
// the byte cap - quite possibly the directory worked in seconds ago. An older
// transcript in the same slug names the same cwd, and costs one more read (#91).
func firstCwd(found []transcript) (string, bool) {
	for _, t := range found {
		if dir, ok := readCwd(t.Path); ok {
			return dir, true
		}
	}
	return "", false
}

// resolveRoot validates a recorded cwd against the filesystem and against git.
func resolveRoot(dir string, git Git) (string, bool) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", false // the repository was deleted or moved: the normal case
	}
	root, err := git.MainCheckout(dir)
	if err != nil {
		slog.Debug("discovery skipped a directory git does not own", "dir", dir, "err", err)
		return "", false
	}
	return root, true
}

// transcripts lists a slug directory's session files, newest first.
//
// A directory that cannot be read is logged rather than dropped in silence: a
// permission error and "this repository was deleted" produced the same empty
// result, so an operator seeing "no repositories found" had no way to learn
// which it was (#91).
func transcripts(slugDir string) []transcript {
	entries, err := os.ReadDir(slugDir)
	if err != nil {
		slog.Warn("discovery cannot read a transcript directory", "dir", slugDir, "err", err)
		return nil
	}
	out := make([]transcript, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		out = append(out, transcript{Path: filepath.Join(slugDir, e.Name()), Used: info.ModTime()})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Used.After(out[b].Used) })
	return out
}

// cwdRecord is the one field discovery reads out of a transcript.
//
// watcher.Entry cannot be reused: it deliberately discards cwd, and it drops
// the record types that carry it earliest. Parsing untyped input into a struct
// at the edge, once, is the rule - this is that struct.
type cwdRecord struct {
	Cwd string `json:"cwd"`
}

// readCwd returns the first working directory a transcript names.
//
// The slug cannot be reversed - paths.TranscriptSlug maps every
// non-alphanumeric to '-', so '/', '.', ' ' and '_' all collapse - but every
// transcript records its own cwd, which sidesteps the lossy mapping entirely.
//
// The leading records are queue operations, settings and attachments, none of
// which carry cwd, so this scans rather than reading only the first line.
func readCwd(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()
	scan := bufio.NewScanner(&io.LimitedReader{R: f, N: maxHeadBytes})
	scan.Buffer(nil, maxHeadBytes)
	for i := 0; i < maxHeadLines && scan.Scan(); i++ {
		var rec cwdRecord
		if err := json.Unmarshal(scan.Bytes(), &rec); err == nil && rec.Cwd != "" {
			return rec.Cwd, true
		}
	}
	// A record longer than the byte cap stops the scan on its first line with
	// ErrTooLong, which is indistinguishable from "this transcript names no
	// cwd" unless it is said out loud - and the cap is reachable, since one
	// record is routinely hundreds of kilobytes (#91).
	if err := scan.Err(); err != nil {
		slog.Warn("discovery could not read a transcript head",
			"transcript", path, "byteCap", maxHeadBytes, "err", err)
	}
	return "", false
}

// sorted orders candidates newest first, breaking ties by name so the list is
// stable between runs.
func sorted(byRoot map[string]Candidate) []Candidate {
	out := make([]Candidate, 0, len(byRoot))
	for _, c := range byRoot {
		out = append(out, c)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].LastUsed.Equal(out[b].LastUsed) {
			return out[a].Name < out[b].Name
		}
		return out[a].LastUsed.After(out[b].LastUsed)
	})
	return out
}
