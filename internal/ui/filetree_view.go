package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	// Every styled span below carries its own explicit Background(ColorSurface)
	// rather than relying on RenderCard's outer background to show through:
	// lipgloss/termenv concatenate raw ANSI codes rather than compositing
	// layers, so each styled span's own reset code (emitted at its end)
	// clobbers whatever background the outer card style set earlier on that
	// line — anything rendered after that reset (even later plain,
	// unstyled text on the same line) falls back to the terminal's own
	// default background instead of ColorSurface unless it's explicitly
	// re-applied too.
	sb.WriteString(StyleSecondary.Background(ColorSurface).Render("Space to toggle, 1-5 to set priority, Enter confirm, Esc abort:") + "\n\n")

	for i, file := range m.Files {
		cursor := lipgloss.NewStyle().Background(ColorSurface).Render("  ")
		if m.Cursor == i {
			cursor = StyleAccentBlue.Background(ColorSurface).Render("> ")
		}

		prio := m.Priorities[i]
		checked := StyleSlate.Background(ColorSurface).Render("[ ]")
		if prio > 0 {
			checked = StyleAccentCyan.Background(ColorSurface).Render(fmt.Sprintf("[%d]", prio))
		}
		file = StylePrimary.Background(ColorSurface).Render(file)
		gap := lipgloss.NewStyle().Background(ColorSurface).Render(" ")

		sb.WriteString(cursor + checked + gap + file + "\n")
	}
	return RenderCard("SELECT FILES", sb.String(), 0, 0, true)
}
