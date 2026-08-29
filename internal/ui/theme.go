package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
)

// Apple Pro / Raycast Dark palette. Each color keeps exactly one job across
// the UI so the dashboard reads as one system. Values marked "spec literal"
// come directly from the design brief; the rest are reasoned extensions
// picked to fit the same system.
const (
	ColorCanvas  = lipgloss.Color("#0b0c0e") // spec literal — app root background only.
	ColorSurface = lipgloss.Color("#16181d") // spec literal — every card's fill.
	ColorBorder  = lipgloss.Color("#252830") // spec literal — default unfocused card border,
	// and the file-browser's selected-row pill background.

	ColorAccentBlue = lipgloss.Color("#0a84ff") // spec literal — structure/focus/primary:
	// focused-panel border, active switcher-strip pill, progress-bar gradient
	// start-stop, section headers, selection/attention.
	ColorAccentCyan = lipgloss.Color("#05d9e8") // spec literal — live/success data: DL/UL
	// readouts, verified-piece heat, "VPN bound" badge, "peer sending" indicator.
	ColorAccentPurple = lipgloss.Color("#bf5af2") // spec literal — heat/emphasis:
	// progress-bar gradient end-stop, urgent/hot-piece heat.
	ColorDanger = lipgloss.Color("#ff2a6d") // spec literal — critical alerts only: VPN dropped.

	ColorTextSecondary = lipgloss.Color("#86868b") // spec literal — muted/secondary text, footer.
	ColorTextPrimary   = lipgloss.Color("#f5f5f7") // reasoned — primary body/heading text.
	ColorAmber         = lipgloss.Color("#ff9f0a") // reasoned — queued/prefetch-piece heat.
	ColorSlate         = lipgloss.Color("#3a3d45") // reasoned — cold-piece heat / disabled chrome.
)

var (
	StylePrimary      = lipgloss.NewStyle().Foreground(ColorTextPrimary)
	StyleSecondary    = lipgloss.NewStyle().Foreground(ColorTextSecondary)
	StyleAccentBlue   = lipgloss.NewStyle().Foreground(ColorAccentBlue)
	StyleAccentCyan   = lipgloss.NewStyle().Foreground(ColorAccentCyan)
	StyleAccentPurple = lipgloss.NewStyle().Foreground(ColorAccentPurple)
	StyleDanger       = lipgloss.NewStyle().Foreground(ColorDanger)
	StyleAmber        = lipgloss.NewStyle().Foreground(ColorAmber)
	StyleSlate        = lipgloss.NewStyle().Foreground(ColorSlate)

	// StyleSelectedRow is the reverse-video bar for an active cursor row in
	// list views (e.g. the in-torrent file selector).
	StyleSelectedRow = lipgloss.NewStyle().Background(ColorAccentBlue).Foreground(lipgloss.Color("#ffffff")).Bold(true)
)

// Badge renders a bracketed HUD status chip: "[text]" in the given style.
func Badge(style lipgloss.Style, text string) string {
	return style.Render("[" + text + "]")
}

// Piece-heat buckets: a BitTorrent transfer is fundamentally pieces cooling
// from "not fetched" to "verified," so color here carries real information.
// Ordering follows the spec's own "cyan, purple, amber, slate" listing read
// most-fetched to least-fetched.
var (
	StylePieceDone = lipgloss.NewStyle().Foreground(ColorAccentCyan)   // verified
	StylePieceHot  = lipgloss.NewStyle().Foreground(ColorAccentPurple) // urgent buffer
	StylePieceWarm = lipgloss.NewStyle().Foreground(ColorAmber)        // queued/prefetch
	StylePieceCold = lipgloss.NewStyle().Foreground(ColorSlate)        // not wanted yet
)

// NewProgressBar builds the animated gradient completion bar used by the
// Inspector card, blending blue to purple per the spec.
func NewProgressBar() progress.Model {
	return progress.New(progress.WithGradient(string(ColorAccentBlue), string(ColorAccentPurple)))
}

// cardTopRule draws a rounded top border — "╭─ TITLE ──────╮" — exactly
// totalWidth cells, carrying the card's title in its edge.
func cardTopRule(title string, totalWidth int, borderColor lipgloss.Color) string {
	if totalWidth < 10 {
		totalWidth = 10
	}
	b := lipgloss.RoundedBorder()
	inner := totalWidth - 2 // minus the two corner runes
	label := strings.ToUpper(title)
	text := " " + label + " "
	if len([]rune(text)) > inner-4 {
		maxLabel := inner - 6
		if maxLabel < 1 {
			maxLabel = 1
		}
		r := []rune(label)
		if len(r) > maxLabel {
			label = string(r[:maxLabel])
		}
		text = " " + label + " "
	}

	const leadWidth = 2
	rightFill := inner - leadWidth - len([]rune(text))
	if rightFill < 0 {
		rightFill = 0
	}

	corner := lipgloss.NewStyle().Foreground(borderColor)
	lead := corner.Render(strings.Repeat(b.Top, leadWidth))
	labelStyled := lipgloss.NewStyle().Bold(true).Foreground(ColorTextPrimary).Render(text)
	rule := corner.Render(strings.Repeat(b.Top, rightFill))

	return corner.Render(b.TopLeft) + lead + labelStyled + rule + corner.Render(b.TopRight)
}

// RenderCard wraps body in the shared rounded card look — surface fill,
// rounded border, title embedded in the top edge — the one titled-panel
// look every bordered surface in the app uses. focused paints the border
// ColorAccentBlue instead of ColorBorder; it's the dashboard's sole visual
// focus indicator. width/height <= 0 leave that dimension auto-sized.
func RenderCard(title, body string, width, height int, focused bool) string {
	borderColor := ColorBorder
	if focused {
		borderColor = ColorAccentBlue
	}
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), false, true, true, true).
		BorderForeground(borderColor).
		Background(ColorSurface).
		Foreground(ColorTextPrimary).
		Padding(1, 2)
	if width > 0 {
		style = style.Width(width)
	}
	if height > 0 {
		style = style.Height(height)
	}
	box := style.Render(body)
	top := cardTopRule(title, lipgloss.Width(box), borderColor)
	return lipgloss.JoinVertical(lipgloss.Left, top, box)
}
