package ui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	hexHashRE    = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
	base32HashRE = regexp.MustCompile(`^[A-Za-z2-7]{32}$`)
)

// normalizeMagnetInput turns a raw header value into a magnet URI the engine
// can consume: a full magnet URI passes through as-is, a bare 40-char hex or
// 32-char Base32 infohash is turned into one.
func normalizeMagnetInput(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(strings.ToLower(s), "magnet:?"):
		return s, nil
	case hexHashRE.MatchString(s), base32HashRE.MatchString(s):
		return "magnet:?xt=urn:btih:" + s, nil
	default:
		return "", fmt.Errorf("not a magnet URI or a 40-char hex / 32-char Base32 infohash")
	}
}

// HeaderInput is the full-width magnet/hash entry field at the top of the
// dashboard.
type HeaderInput struct {
	Input textinput.Model
}

func NewHeaderInput() HeaderInput {
	ti := textinput.New()
	ti.Prompt = "⌘ Enter "
	ti.Placeholder = "magnet URI or hash... magnet:?xt=urn:btih:..."
	ti.PromptStyle = StyleAccentBlue
	ti.PlaceholderStyle = StyleSecondary
	ti.TextStyle = StylePrimary
	// Live visual feedback only — Validate never blocks keystrokes, it just
	// sets Input.Err. The authoritative check happens again on submit.
	ti.Validate = func(s string) error {
		if s == "" {
			return nil
		}
		_, err := normalizeMagnetInput(s)
		return err
	}
	ti.Focus()
	return HeaderInput{Input: ti}
}

func (h HeaderInput) Update(msg tea.Msg) (HeaderInput, tea.Cmd) {
	var cmd tea.Cmd
	h.Input, cmd = h.Input.Update(msg)
	return h, cmd
}

func (h HeaderInput) View(width int, focused bool) string {
	borderColor := ColorBorder
	if focused {
		borderColor = ColorAccentBlue
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(ColorSurface).
		Padding(0, 2)
	if width > 0 {
		style = style.Width(width)
	}
	return style.Render(h.Input.View())
}
