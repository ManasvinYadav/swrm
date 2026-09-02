package ui

import (
	"fmt"
	"strings"

	"github.com/anacrolix/torrent"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// filePriorityLevels are the three selectable "wanted" tiers a file can
// cycle through with Left/Right, ordered Low -> Normal -> High. This is a
// distinct concept from torrent.PiecePriorityNone (Space, to skip a file
// entirely): the request strategy treats "not wanted" categorically
// differently from "wanted, but at a lower priority than another file", so
// skip stays a separate toggle rather than a fourth step on this scale.
var filePriorityLevels = []torrent.PiecePriority{
	torrent.PiecePriorityNormal,    // Low
	torrent.PiecePriorityHigh,      // Normal
	torrent.PiecePriorityReadahead, // High
}

func priorityLabel(p torrent.PiecePriority) string {
	switch p {
	case torrent.PiecePriorityNormal:
		return "Low"
	case torrent.PiecePriorityHigh:
		return "Normal"
	case torrent.PiecePriorityReadahead:
		return "High"
	default:
		return "Normal"
	}
}

// stepPriority moves p by delta steps along filePriorityLevels, clamped to
// its ends.
func stepPriority(p torrent.PiecePriority, delta int) torrent.PiecePriority {
	idx := 0
	for i, lvl := range filePriorityLevels {
		if lvl == p {
			idx = i
			break
		}
	}
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(filePriorityLevels) {
		idx = len(filePriorityLevels) - 1
	}
	return filePriorityLevels[idx]
}

type FileTreeView struct {
	Files      []string
	Cursor     int
	Priorities map[int]torrent.PiecePriority // PiecePriorityNone = skip; otherwise one of filePriorityLevels
	Done       bool
	Aborted    bool
}

func NewFileTreeView(files []string) FileTreeView {
	priorities := make(map[int]torrent.PiecePriority)
	for i := range files {
		priorities[i] = torrent.PiecePriorityHigh // Default include at Normal priority
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
			if m.Priorities[m.Cursor] != torrent.PiecePriorityNone {
				m.Priorities[m.Cursor] = torrent.PiecePriorityNone
			} else {
				m.Priorities[m.Cursor] = torrent.PiecePriorityHigh
			}
		case "left":
			// No-op on a skipped file: there's no priority to lower until
			// it's selected again with Space.
			if cur := m.Priorities[m.Cursor]; cur != torrent.PiecePriorityNone {
				m.Priorities[m.Cursor] = stepPriority(cur, -1)
			}
		case "right":
			if cur := m.Priorities[m.Cursor]; cur != torrent.PiecePriorityNone {
				m.Priorities[m.Cursor] = stepPriority(cur, 1)
			}
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
	sb.WriteString(StyleSecondary.Background(ColorSurface).Render("Space to toggle, ←/→ to set priority, Enter confirm, Esc abort:") + "\n\n")

	for i, file := range m.Files {
		cursor := lipgloss.NewStyle().Background(ColorSurface).Render("  ")
		if m.Cursor == i {
			cursor = StyleAccentBlue.Background(ColorSurface).Render("> ")
		}

		prio := m.Priorities[i]
		checked := StyleSlate.Background(ColorSurface).Render("[ ]")
		if prio != torrent.PiecePriorityNone {
			checked = StyleAccentCyan.Background(ColorSurface).Render(fmt.Sprintf("[%s]", priorityLabel(prio)))
		}
		file = StylePrimary.Background(ColorSurface).Render(file)
		gap := lipgloss.NewStyle().Background(ColorSurface).Render(" ")

		sb.WriteString(cursor + checked + gap + file + "\n")
	}
	return RenderCard("SELECT FILES", sb.String(), 0, 0, true)
}
