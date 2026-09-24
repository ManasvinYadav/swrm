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
	case hexHashRE.MatchString(s):
		return "magnet:?xt=urn:btih:" + s, nil
	case base32HashRE.MatchString(s):
		// The engine's magnet parser decodes Base32 with the standard
		// (uppercase-only) alphabet, so a lowercase hash that passed the
		// check above would still be rejected on submit.
		return "magnet:?xt=urn:btih:" + strings.ToUpper(s), nil
	default:
		return "", fmt.Errorf("not a magnet URI or a 40-char hex / 32-char Base32 infohash")
	}
}

// HeaderInput is the full-width magnet/hash entry field at the top of the
// dashboard.
type HeaderInput struct {
	Input textinput.Model
}

// headerChromeWidth is HeaderInput.View's Border (2 cols) + Padding(0,2)
// (4 cols).
const headerChromeWidth = 6

// SetWidth sizes the header to exactly width columns, border included. The
// textinput's own Width is only its scrolling text area: the prompt and the
// trailing cursor cell render beside it, so both come out of the budget too
// or a long magnet URI wraps the header onto a second line.
func (h *HeaderInput) SetWidth(width int) {
	h.Input.Width = max(width-headerChromeWidth-lipgloss.Width(h.Input.Prompt)-1, 1)
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
		// Width excludes the border, which lipgloss adds on top.
		style = style.Width(width - 2)
	}
	return style.Render(h.Input.View())
}
