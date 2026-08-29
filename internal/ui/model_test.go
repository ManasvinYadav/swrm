package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestDiagnosticsKeyGuardedByFileTree guards against a regression of the
// modal key-guard bug: 'd' (and by the same fix, s/[/]/{/}/0) must not act
// while the file-selection modal is open.
func TestDiagnosticsKeyGuardedByFileTree(t *testing.T) {
	ft := NewFileTreeView([]string{"a.txt"})
	// focus must be non-header: "d" is also a valid hex-hash character, so
	// it only acts as the diagnostics shortcut when the header doesn't have
	// keyboard focus (see RootModel.Update's combined "1","2","3","4","d",
	// " " case).
	m := RootModel{state: stateDashboard, focus: focusInspector, Dashboard: NewDashboardView(), FileTree: &ft}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	rm := next.(RootModel)
	if rm.ShowDiagnostics {
		t.Fatal("expected 'd' to be ignored while the file-tree modal is open")
	}

	rm.FileTree = nil
	next2, _ := rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	rm2 := next2.(RootModel)
	if !rm2.ShowDiagnostics {
		t.Fatal("expected 'd' to toggle diagnostics once the modal is closed")
	}

	next3, _ := rm2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm3 := next3.(RootModel)
	if rm3.ShowDiagnostics {
		t.Fatal("expected Esc to close the diagnostics modal")
	}
}
