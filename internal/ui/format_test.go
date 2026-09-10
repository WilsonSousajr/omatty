package ui_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

func TestAgeString_Table_issue37(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		ago  time.Duration
		want string
	}{
		{0, "<1m"},
		{30 * time.Second, "<1m"},
		{4 * time.Minute, "4m"},
		{59 * time.Minute, "59m"},
		{2 * time.Hour, "2h"},
		{23 * time.Hour, "23h"},
		{3 * 24 * time.Hour, "3d"},
	}
	for _, tt := range tests {
		if got := ui.AgeString(now, now.Add(-tt.ago)); got != tt.want {
			t.Errorf("AgeString(%v ago) = %q, want %q", tt.ago, got, tt.want)
		}
	}
}

func TestAgeString_ZeroTimeIsEmpty_issue37(t *testing.T) {
	if got := ui.AgeString(time.Now(), time.Time{}); got != "" {
		t.Errorf("AgeString(zero) = %q, want empty", got)
	}
}

func TestKString_Table_issue39(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{950, "950"},
		{1000, "1.0k"},
		{12345, "12.3k"},
		{999999, "1000.0k"},
		{1200000, "1.2M"},
	}
	for _, tt := range tests {
		if got := ui.KString(tt.n); got != tt.want {
			t.Errorf("KString(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
