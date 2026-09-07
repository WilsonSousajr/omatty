package registry_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func TestPlaceholderTitle_IsTheFirstEightCharacters_issue127(t *testing.T) {
	if got := registry.PlaceholderTitle("abc12345-6789-0000"); got != "abc12345" {
		t.Errorf("PlaceholderTitle() = %q, want abc12345", got)
	}
	if got := registry.PlaceholderTitle("abc"); got != "abc" {
		t.Errorf("PlaceholderTitle() on a short id = %q, want abc", got)
	}
}
