package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRectanglePadsMultiLineContentToEqualWidth(t *testing.T) {
	got := rectangle("ab\n日本\nx")
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 4 {
			t.Errorf("line %d width = %d, want 4 (%q)", i, w, line)
		}
	}
}

func TestRectangleLeavesSingleLineUnchanged(t *testing.T) {
	const content = "a line"
	if got := rectangle(content); got != content {
		t.Errorf("rectangle(%q) = %q, want unchanged", content, got)
	}
}

func TestZonesIDNamespacesScope(t *testing.T) {
	z := NewZones("pane")
	if got, want := z.ID("button"), "pane/button"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
}
