package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func TestModel_AStatusEventPushesOneLaneCellAndAUsageUpdateDoesNot_issue128(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	status(m, "s1", watcher.PromptSubmitted, time.Now())
	status(m, "s1", watcher.UsageUpdated, time.Now())

	lane := m.LaneOf("s1")

	if lipgloss.Width(lane) != ui.LaneCells() || strings.Count(lane, "▄") != 1 {
		t.Errorf("lane %q after one status event and one usage update; want one thinking cell in the lane", lane)
	}
}

func TestModel_TheLaneKeepsTheLastLaneCellsStatuses_issue128(t *testing.T) {
	m, _ := modelWithFakes(t)
	for range 12 {
		status(m, "s1", watcher.ToolStarted, time.Now())
		status(m, "s1", watcher.ToolFinished, time.Now())
	}
	if lane := m.LaneOf("s1"); lipgloss.Width(lane) != ui.LaneCells() || strings.Contains(lane, "  ") {
		t.Errorf("lane %q, want every cell filled", lane)
	}
}

// Every status has a cell, so a new status cannot silently render as nothing.
func TestLaneBlock_CoversEveryStatus_issue128(t *testing.T) {
	for _, s := range []watcher.Status{
		watcher.StatusIdle, watcher.StatusThinking, watcher.StatusTool, watcher.StatusWaiting,
		watcher.StatusDone, watcher.StatusError, watcher.StatusExited,
	} {
		if _, ok := ui.LaneBlocks()[s]; !ok {
			t.Errorf("status %q has no lane cell; it would render as nothing", s)
		}
	}
}

func TestModel_AWaitingSessionsLaneShowsTheFullCell_issue128(t *testing.T) {
	m, _ := modelWithFakes(t)
	status(m, "s1", watcher.PermissionRequested, time.Now())
	if !strings.Contains(m.LaneOf("s1"), "█") {
		t.Errorf("lane %q does not show the full cell for waiting", m.LaneOf("s1"))
	}
}

func TestModel_ASessionWithNoEventsDrawsAnEmptyLane_issue128(t *testing.T) {
	m, _ := modelWithFakes(t)
	if lane := m.LaneOf("s2"); lane != strings.Repeat(" ", ui.LaneCells()) {
		t.Errorf("lane %q for a session with no events, want blanks", lane)
	}
}
