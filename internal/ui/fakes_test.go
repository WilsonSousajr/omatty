package ui_test

import (
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// FakeStat stands in for the git-backed reader behind ui.RepoStatFunc (#180).
type FakeStat struct {
	Stats map[string]review.Stat // by session id
	Err   error
	Asked []string
	Roots []string
}

func (f *FakeStat) Stat(sess registry.Session, root string) (review.Stat, error) {
	f.Asked = append(f.Asked, sess.ID)
	f.Roots = append(f.Roots, root)
	return f.Stats[sess.ID], f.Err
}

// FakeNamer stands in for the transcript reader behind ui.NameFunc. A named
// type, per AGENTS.md, so a failure message says what produced the title.
type FakeNamer struct {
	Titles map[string]string // by session id
	Err    error
	Asked  []string
	Block  chan struct{} // when non-nil, Name waits on it: the never-blocks test
}

func (f *FakeNamer) Name(sess registry.Session) (string, error) {
	f.Asked = append(f.Asked, sess.ID)
	if f.Block != nil {
		<-f.Block
	}
	return f.Titles[sess.ID], f.Err
}

// FakeRename records what was persisted per session, so a test can tell "the
// sidebar shows it" from "state.json holds it" (#41).
type FakeRename struct {
	Titles map[string]string
	Err    error
	Calls  int
}

func (f *FakeRename) Rename(id, title string) error {
	f.Calls++
	if f.Titles == nil {
		f.Titles = map[string]string{}
	}
	f.Titles[id] = title
	return f.Err
}

// FakeModelNamer stands in for the headless naming call behind
// ui.ModelNameFunc, keyed by the prompt-derived title it was asked to improve.
type FakeModelNamer struct {
	Names map[string]string
	Err   error
	Asked []string
}

func (f *FakeModelNamer) Name(title string) (string, error) {
	f.Asked = append(f.Asked, title)
	return f.Names[title], f.Err
}
