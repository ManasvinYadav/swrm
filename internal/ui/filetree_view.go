package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type FileTreeView struct {
	Files      []string
	Cursor     int
	Priorities map[int]int // 0 = skip, 1-5 = priorities
	Done       bool
	Aborted    bool
}

func NewFileTreeView(files []string) FileTreeView {
	priorities := make(map[int]int)
	for i := range files {
		priorities[i] = 1 // Default include with normal priority
	}
	return FileTreeView{
		Files:      files,
		Cursor:     0,
		Priorities: priorities,
	}
}

func (m FileTreeView) Update(msg tea.Msg) (FileTreeView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down", "j":
			if m.Cursor < len(m.Files)-1 {
				m.Cursor++
			}
		case " ":
			if m.Priorities[m.Cursor] > 0 {
				m.Priorities[m.Cursor] = 0
			} else {
				m.Priorities[m.Cursor] = 1
			}
		case "1", "2", "3", "4", "5":
			val := int(msg.String()[0] - '0')
			m.Priorities[m.Cursor] = val
		case "enter":
			m.Done = true
		case "esc":
			m.Aborted = true
			m.Done = true
		}
	}
	return m, nil
}

func (m FileTreeView) View() string {
	var sb strings.Builder
	sb.WriteString(StyleSecondary.Render("Space to toggle, 1-5 to set priority, Enter confirm, Esc abort:\n\n"))

	for i, file := range m.Files {
		cursor := "  "
		if m.Cursor == i {
			cursor = StyleAccentBlue.Render("▶ ")
		}

		prio := m.Priorities[i]
		checked := StyleSlate.Render("[ ]")
		if prio > 0 {
			checked = StyleAccentCyan.Render(fmt.Sprintf("[%d]", prio))
		}

		sb.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checked, file))
	}
	return RenderCard("SELECT FILES", sb.String(), 0, 0, true)
}
