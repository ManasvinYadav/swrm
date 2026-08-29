package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestSplashFinishesAtFrame80(t *testing.T) {
	m := NewSplashModel()
	m.ticks = 79
	_, cmd := m.Update(tickMsg{})
	if _, ok := cmd().(SplashFinishedMsg); !ok {
		t.Fatal("expected finished message")
	}
	m = NewSplashModel()
	_, cmd = m.Update(tea.KeyMsg{})
	if _, ok := cmd().(SplashFinishedMsg); !ok {
		t.Fatal("key should finish splash")
	}
}
