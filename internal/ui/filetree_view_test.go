package ui

import (
	"strings"
	"testing"

	"github.com/anacrolix/torrent"
	tea "github.com/charmbracelet/bubbletea"
)

func TestFileTreeViewPriorityCycling(t *testing.T) {
	m := NewFileTreeView([]string{"a", "b"})
	if got := m.Priorities[0]; got != torrent.PiecePriorityHigh {
		t.Fatalf("default priority = %v, want PiecePriorityHigh (our Normal)", got)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.Priorities[0]; got != torrent.PiecePriorityReadahead {
		t.Fatalf("after right = %v, want PiecePriorityReadahead (our High)", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // already at the top: clamp
	if got := m.Priorities[0]; got != torrent.PiecePriorityReadahead {
		t.Fatalf("clamp at High failed, got %v", got)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if got := m.Priorities[0]; got != torrent.PiecePriorityNormal {
		t.Fatalf("after two lefts = %v, want PiecePriorityNormal (our Low)", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // already at the bottom: clamp
	if got := m.Priorities[0]; got != torrent.PiecePriorityNormal {
		t.Fatalf("clamp at Low failed, got %v", got)
	}

	// Space skips the file entirely; Left/Right must then no-op since there
	// is no priority to adjust until it's re-selected.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := m.Priorities[0]; got != torrent.PiecePriorityNone {
		t.Fatalf("after space = %v, want PiecePriorityNone", got)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := m.Priorities[0]; got != torrent.PiecePriorityNone {
		t.Fatalf("right on a skipped file should no-op, got %v", got)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := m.Priorities[0]; got != torrent.PiecePriorityHigh {
		t.Fatalf("re-selecting a skipped file = %v, want the default PiecePriorityHigh", got)
	}
}

func TestFileTreeViewRendersPriorityLabels(t *testing.T) {
	m := NewFileTreeView([]string{"movie.mp4"})
	view := m.View()
	if !strings.Contains(view, "[Normal]") {
		t.Fatalf("expected the default priority label in the view, got:\n%s", view)
	}
	if strings.Contains(view, "[1]") {
		t.Fatalf("expected no leftover numeric priority badge, got:\n%s", view)
	}

	m.Priorities[0] = torrent.PiecePriorityNone
	if view := m.View(); !strings.Contains(view, "[ ]") {
		t.Fatalf("expected an empty checkbox for a skipped file, got:\n%s", view)
	}
}
