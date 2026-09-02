package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func renderNavPill(n int, label string, active bool) string {
	text := fmt.Sprintf("[%d] %s", n, label)
	borderColor, fg := ColorBorder, ColorTextSecondary
	if active {
		borderColor, fg = ColorAccentBlue, ColorAccentBlue
	}
	style := lipgloss.NewStyle().
		Foreground(fg).
		Bold(active).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)
	return style.Render(text)
}

// renderHUD renders the bar directly below the 50/50 deck: nav pills on the
// left, VPN status pill on the right. Pills never swap to a separate
// screen — 1 focuses the header, 2/3 reflect which Inspector section is
// currently emphasized.
func renderHUD(focus focusTarget, section inspectorSection, vpnActive bool, vpnLabel string) string {
	pills := []string{
		renderNavPill(1, "Search", focus == focusHeader),
		renderNavPill(2, "Transfers", section == sectionGauges),
		renderNavPill(3, "Swarm", section == sectionSwarm),
	}
	// Pills are bordered boxes (3 lines tall), so joining them with plain
	// string "+" would only append after the last line rather than laying
	// them out side by side — lipgloss.JoinHorizontal is required, same as
	// the switcher strip.
	spaced := make([]string, 0, len(pills)*2-1)
	for i, p := range pills {
		if i > 0 {
			spaced = append(spaced, "  ")
		}
		spaced = append(spaced, p)
	}
	left := lipgloss.JoinHorizontal(lipgloss.Center, spaced...)

	vpnStyle := StyleAccentCyan
	vpnText := fmt.Sprintf("● VPN Active: %s", vpnLabel)
	if !vpnActive {
		vpnStyle = StyleDanger
		vpnText = fmt.Sprintf("● VPN Dropped: %s", vpnLabel)
	}
	right := vpnStyle.Render(vpnText)

	return lipgloss.JoinHorizontal(lipgloss.Center, left, lipgloss.NewStyle().PaddingLeft(4).Render(right))
}

func renderFooter() string {
	return lipgloss.NewStyle().Foreground(ColorTextSecondary).Render(
		"esc back  •  tab switch  •  ↵ submit  •  space pause  •  q quit")
}
