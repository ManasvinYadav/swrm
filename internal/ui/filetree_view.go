package ui

import (
	"fmt"
	"strings"

	"github.com/anacrolix/torrent"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	// Terminal size (0 = unknown, no limit) and the index of the first file
	// row shown, so a torrent with more files than the terminal has rows
	// scrolls instead of running off the screen.
	width, height, offset int
}

// fileTreeChromeRows is every row View renders besides the file rows: the
// card's title rule, top/bottom padding and bottom border (4), the
// instructions line and the blank line after it (2), and the scroll
// position line at the bottom (1).
const fileTreeChromeRows = 7

// fileTreeRowChromeWidth is every column a file row spends besides its name:
// the card's border and padding, the cursor, the widest "[Normal]" label,
// and the gap after it.
const fileTreeRowChromeWidth = cardChromeWidth + 2 + 8 + 1

// SetSize tells the view how much of the terminal it may use.
func (m *FileTreeView) SetSize(width, height int) {
	m.width, m.height = width, height
	m.scrollToCursor()
}

func (m FileTreeView) visibleRows() int {
	if m.height <= 0 {
		return len(m.Files)
	}
	return max(m.height-fileTreeChromeRows, 1)
}

// scrollToCursor moves the visible window just enough to contain Cursor.
func (m *FileTreeView) scrollToCursor() {
	rows := m.visibleRows()
	if m.Cursor < m.offset {
		m.offset = m.Cursor
	}
	if m.Cursor >= m.offset+rows {
		m.offset = m.Cursor - rows + 1
	}
	m.offset = max(min(m.offset, len(m.Files)-rows), 0)
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
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}
			m.scrollToCursor()
		case "down", "j":
			if m.Cursor < len(m.Files)-1 {
				m.Cursor++
			}
			m.scrollToCursor()
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

	end := min(m.offset+m.visibleRows(), len(m.Files))
	for i := m.offset; i < end; i++ {
		file := m.Files[i]
		if m.width > 0 {
			file = ansi.Truncate(file, max(m.width-fileTreeRowChromeWidth, 1), "…")
		}
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
	if m.offset > 0 || end < len(m.Files) {
		sb.WriteString(StyleSecondary.Background(ColorSurface).Render(fmt.Sprintf("%d–%d of %d files", m.offset+1, end, len(m.Files))))
	}
	return RenderCard("SELECT FILES", sb.String(), 0, 0, true)
}
