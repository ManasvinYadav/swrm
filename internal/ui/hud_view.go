package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func renderNavPill(n int, label string, active bool) string {
	text := fmt.Sprintf("[%d] %s", n, label)
	style := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	if active {
		style = lipgloss.NewStyle().Foreground(ColorAccentBlue).Bold(true)
	}
	return style.Render(text)
}

// renderHUD renders the bar directly below the 50/50 deck: nav pills on the
// left, VPN status pill on the right. Pills never swap to a separate
// screen — 1 focuses the header, 2/3 reflect which Inspector section is
// currently emphasized, 4 is a momentary streaming action.
func renderHUD(focus focusTarget, section inspectorSection, vpnActive bool, vpnLabel string) string {
	pills := []string{
		renderNavPill(1, "Search", focus == focusHeader),
		renderNavPill(2, "Transfers", section == sectionGauges),
		renderNavPill(3, "Swarm", section == sectionSwarm),
		renderNavPill(4, "Streaming", false),
	}
	left := pills[0]
	for _, p := range pills[1:] {
		left += "   " + p
	}

	vpnStyle := StyleAccentCyan
	vpnText := fmt.Sprintf("● VPN Active: %s", vpnLabel)
	if !vpnActive {
		vpnStyle = StyleDanger
		vpnText = fmt.Sprintf("● VPN Dropped: %s", vpnLabel)
	}
	right := vpnStyle.Render(vpnText)

	return left + lipgloss.NewStyle().PaddingLeft(4).Render(right)
}

func renderFooter() string {
	return lipgloss.NewStyle().Foreground(ColorTextSecondary).Render(
		"esc back  •  tab switch  •  ↵ submit  •  space pause  •  q quit")
}
