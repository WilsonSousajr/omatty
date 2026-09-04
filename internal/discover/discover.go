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
// two methods rather than nine.
type Git interface {
	RepoRoot(dir string) (string, error)
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
//	cands, err := discover.Propose(paths.TranscriptsDir(home), vcs.NewCLI())
func Propose(storeRoot string, git Git) ([]Candidate, error) {
	entries, err := os.ReadDir(storeRoot)
	if err != nil {
		return nil, fmt.Errorf("discover: cannot read the transcript store %q: %w", storeRoot, err)
	}
	byRoot := map[string]Candidate{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		keepNewest(byRoot, filepath.Join(storeRoot, e.Name()), git)
	}
	return sorted(byRoot), nil
}

// keepNewest resolves one slug directory to a candidate and keeps whichever of
// the two is more recent. A slug that resolves to nothing is skipped in
// silence: a deleted repository is the normal case, not an error.
func keepNewest(byRoot map[string]Candidate, slugDir string, git Git) {
	cand, ok := candidateOf(slugDir, git)
	if !ok {
		return
	}
	if seen, dup := byRoot[cand.Root]; dup && seen.LastUsed.After(cand.LastUsed) {
		return
	}
	byRoot[cand.Root] = cand
}

// candidateOf turns one slug directory into a candidate, applying the three
// filters: the directory must still exist, it must be a git repository, and a
// linked worktree resolves to the repository it was forked from.
func candidateOf(slugDir string, git Git) (Candidate, bool) {
	transcript, used, ok := newestTranscript(slugDir)
	if !ok {
		return Candidate{}, false
	}
	dir, ok := readCwd(transcript)
	if !ok {
		return Candidate{}, false
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return Candidate{}, false // the repository was deleted or moved
	}
	root, err := git.MainCheckout(dir)
	if err != nil {
		return Candidate{}, false // not a repository at all
	}
	return Candidate{Name: filepath.Base(root), Root: root, LastUsed: used}, true
}

// newestTranscript is the most recently modified .jsonl in a slug directory,
// with its modification time. That time is when the directory was last worked
// in, which is what orders the list.
func newestTranscript(slugDir string) (string, time.Time, bool) {
	entries, err := os.ReadDir(slugDir)
	if err != nil {
		return "", time.Time{}, false
	}
	var newest string
	var at time.Time
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || filepath.Ext(e.Name()) != ".jsonl" || !info.ModTime().After(at) {
			continue
		}
		newest, at = filepath.Join(slugDir, e.Name()), info.ModTime()
	}
	return newest, at, newest != ""
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
