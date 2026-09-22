package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// The shape of the workload the memory investigation measured: thirteen live
// sessions over three projects in a conventional window. Every number in the
// benchmarks below is per frame, and a frame is drawn on every message - the
// 1 s tick, thirteen tailer polls a second, and every PTY chunk from any pane.
const (
	benchSessions = 13
	benchProjects = 3
	benchWidth    = 120
	benchHeight   = 40
)

// benchPaneLine is one line of a Claude pane as the emulator hands it over:
// full width, and carrying the SGR that makes lipgloss.Width walk it grapheme
// by grapheme instead of counting bytes.
func benchPaneLine(i int) string {
	body := fmt.Sprintf("│ %2d  tool use: read file internal/ui/render.go and report", i)
	return "\x1b[38;5;245m" + body + "\x1b[0m" + strings.Repeat(" ", 92-len([]rune(body)))
}

// benchPane is a full emulator screen: 37 rows of styled, full-width text.
func benchPane() string {
	rows := make([]string, 37)
	for i := range rows {
		rows[i] = benchPaneLine(i)
	}
	return strings.Join(rows, "\n")
}

// benchModel is a Model at the measured workload, sized and ready to draw.
func benchModel(b *testing.B) *Model {
	b.Helper()
	st := registry.State{}
	terms := map[string]termwrap.Terminal{}
	pane := benchPane()
	for p := range benchProjects {
		st.Projects = append(st.Projects, registry.Project{Name: fmt.Sprintf("project-%d", p), Root: fmt.Sprintf("/tmp/p%d", p)})
	}
	for s := range benchSessions {
		id := fmt.Sprintf("session-%02d", s)
		st.Sessions = append(st.Sessions, registry.Session{
			ID: id, Project: fmt.Sprintf("project-%d", s%benchProjects),
			Title: fmt.Sprintf("a session with a reasonably long title %d", s),
			Dir:   fmt.Sprintf("/tmp/p%d", s%benchProjects),
		})
		terms[id] = termwrap.NewFake(pane)
	}
	m := NewModel(Deps{State: st, Terms: terms})
	m.width, m.height = benchWidth, benchHeight
	return m
}

// BenchmarkFrame is the whole frame: sidebar, hairlines, pane and header.
func BenchmarkFrame(b *testing.B) {
	m := benchModel(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.frame()
	}
}

// BenchmarkRenderTerminal is the pane alone - the emulator's whole grid split
// into lines and forced to the pane's exact size.
func BenchmarkRenderTerminal(b *testing.B) {
	m := benchModel(b)
	w, h := PaneSize(m.width, m.height, false)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.renderTerminal(w, h)
	}
}

// BenchmarkFitLine is the innermost call, once per line of every column.
func BenchmarkFitLine(b *testing.B) {
	styled := benchPaneLine(7)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = fitLine(styled, 92)
	}
}
