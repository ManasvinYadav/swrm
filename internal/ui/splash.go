package ui

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var asciiBanner = []string{
	`███████╗██╗    ██╗██████╗ ███╗   ███╗`,
	`██╔════╝██║    ██║██╔══██╗████╗ ████║`,
	`███████╗██║ █╗ ██║██████╔╝██╔████╔██║`,
	`╚════██║██║███╗██║██╔══██╗██║╚██╔╝██║`,
	`███████║╚███╔███╔╝██║  ██║██║ ╚═╝ ██║`,
	`╚══════╝ ╚══╝╚══╝ ╚═╝  ╚═╝╚═╝     ╚═╝`,
}

type SplashModel struct {
	ticks  int
	width  int
	height int
	done   bool
}

func NewSplashModel() SplashModel {
	return SplashModel{ticks: 0}
}

func (m SplashModel) Init() tea.Cmd {
	return tickCmd()
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

type SplashFinishedMsg struct{}

func (m SplashModel) Update(msg tea.Msg) (SplashModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.done = true
		return m, func() tea.Msg { return SplashFinishedMsg{} }
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		m.ticks++
		if m.ticks >= 80 {
			m.done = true
			return m, func() tea.Msg { return SplashFinishedMsg{} }
		}
		return m, tickCmd()
	}
	return m, nil
}

func (m SplashModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var sb strings.Builder

	// Ticks 0-6: Scanline vertical reveal
	visibleRows := m.ticks
	if visibleRows > len(asciiBanner) {
		visibleRows = len(asciiBanner)
	}

	for i := 0; i < visibleRows; i++ {
		row := asciiBanner[i]
		// Ticks 7-20: Color interpolation
		if m.ticks >= 7 {
			colorStep := int(math.Abs(math.Sin(float64(m.ticks+i)*0.2)) * 5)
			var style lipgloss.Style
			switch colorStep {
			case 0:
				style = StyleAccentCyan
			case 1:
				style = StyleAccentBlue
			case 2:
				style = StyleAccentPurple
			case 3:
				style = StyleDanger
			case 4:
				style = StyleAmber
			case 5:
				style = StylePrimary
			}
			sb.WriteString(style.Render(row) + "\n")
		} else {
			sb.WriteString(StyleAccentCyan.Render(row) + "\n")
		}
	}

	sb.WriteString("\n")

	// Ticks 21-40: Typewriter effect
	if m.ticks >= 21 {
		msg := "▶ terminal bittorrent & stream engine... █"
		charsToShow := m.ticks - 21
		if charsToShow > len(msg) {
			charsToShow = len(msg)
		}

		// Handle runes to avoid splitting multi-byte chars
		runes := []rune(msg)
		if charsToShow > len(runes) {
			charsToShow = len(runes)
		}
		sb.WriteString(StyleAccentBlue.Render(string(runes[:charsToShow])) + "\n")
	}

	// Ticks 41-60: Diagnostic logs
	if m.ticks >= 41 {
		sb.WriteString("\n")
		sb.WriteString(StyleSecondary.Render("[OK] Core Engine Initialized") + "\n")
		sb.WriteString(StyleSecondary.Render("[OK] Local .torrent Index Ready") + "\n")
		sb.WriteString(StyleSecondary.Render("[OK] Binding Virtual Network Interface") + "\n")
	}

	// Center the block
	block := sb.String()
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}
