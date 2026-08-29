package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Apple Pro / Raycast Dark palette. Each color keeps exactly one job across
// the UI so the dashboard reads as one system. Values marked "spec literal"
// come directly from the design brief; the rest are reasoned extensions
// picked to fit the same system.
const (
	ColorCanvas  = lipgloss.Color("#0b0c0e") // spec literal — app root background only.
	ColorSurface = lipgloss.Color("#16181d") // spec literal — every card's fill.
	// ColorBorder was originally #252830 (the spec literal), then #454b59
	// after a first brightening pass — but #454b59 still measures only
	// 2.03:1 against ColorSurface, under WCAG's 3:1 floor for non-text UI
	// boundaries (SC 1.4.11). Brightened again to clear that floor with a
	// real margin (3.28:1 vs Surface, 3.62:1 vs Canvas).
	ColorBorder = lipgloss.Color("#616a80") // default unfocused card border,
	// and the file-browser's selected-row pill background.

	// ColorAccentBlue was originally Apple's system blue #0a84ff (4.87:1
	// against ColorSurface) — legible, but Raycast's own published accent
	// blue (#55b3ff) is both lighter and the actual token this theme is
	// meant to echo, reaching 7.85:1 against the same surface. Matched to
	// Raycast's real value rather than Apple's for both fidelity and a11y.
	ColorAccentBlue = lipgloss.Color("#55b3ff") // structure/focus/primary:
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
	// ColorSlate was originally #3a3d45 — nearly unreadable against
	// ColorSurface for disabled file-browser entries and cold pieces.
	// Brightened to an actually-legible muted gray.
	ColorSlate = lipgloss.Color("#6b7280") // reasoned — cold-piece heat / disabled chrome.
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

func hexRGB(hex string) [3]int {
	h := strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseInt(h[0:2], 16, 32)
	g, _ := strconv.ParseInt(h[2:4], 16, 32)
	b, _ := strconv.ParseInt(h[4:6], 16, 32)
	return [3]int{int(r), int(g), int(b)}
}

func lerpRGB(a, b [3]int, t float64) lipgloss.Color {
	r := int(float64(a[0]) + t*float64(b[0]-a[0]))
	g := int(float64(a[1]) + t*float64(b[1]-a[1]))
	bl := int(float64(a[2]) + t*float64(b[2]-a[2]))
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, bl))
}

var brandGradientStops = [][3]int{hexRGB(string(ColorAccentCyan)), hexRGB(string(ColorAccentBlue)), hexRGB(string(ColorAccentPurple)), hexRGB(string(ColorAccentCyan))}

// brandGradientColorAt returns the signature cyan -> blue -> purple -> cyan
// brand gradient color at position t, wrapping cyclically so an animated
// phase offset sweeps smoothly with no hard reset at the loop point.
func brandGradientColorAt(t float64) lipgloss.Color {
	t -= math.Floor(t)
	seg := t * float64(len(brandGradientStops)-1)
	idx := int(seg)
	if idx > len(brandGradientStops)-2 {
		idx = len(brandGradientStops) - 2
	}
	return lerpRGB(brandGradientStops[idx], brandGradientStops[idx+1], seg-float64(idx))
}

// RenderGradientBanner paints lines with one smooth horizontal sweep of the
// signature brand gradient shared across every line, so a block-letter logo
// reads as one cohesive wordmark instead of per-line flat colors. phase
// slowly advances (driven by the dashboard's existing 1s tick) for a subtle
// ambient shimmer rather than a static print.
func RenderGradientBanner(lines []string, phase float64) string {
	maxLen := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > maxLen {
			maxLen = n
		}
	}
	if maxLen == 0 {
		return ""
	}

	var out strings.Builder
	for li, line := range lines {
		runes := []rune(line)
		for i, r := range runes {
			t := float64(i)/float64(maxLen) + phase
			out.WriteString(lipgloss.NewStyle().Foreground(brandGradientColorAt(t)).Render(string(r)))
		}
		if li != len(lines)-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

// RenderAnimatedBar draws the Inspector card's completion bar using the same
// animated brand gradient (and phase) as the header wordmark, rather than
// bubbles/progress's fixed two-color fill — a static gradient reads as
// unanimated when the underlying download percentage barely moves between
// ticks, so the bar's *color* sweeps continuously instead of relying on the
// percentage itself for visible motion.
func RenderAnimatedBar(width int, percent, phase float64) string {
	if width < 1 {
		width = 1
	}
	if percent < 0 {
		percent = 0
	} else if percent > 1 {
		percent = 1
	}
	filled := int(float64(width)*percent + 0.5)

	var out strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			t := float64(i)/float64(width) + phase
			out.WriteString(lipgloss.NewStyle().Foreground(brandGradientColorAt(t)).Render("█"))
		} else {
			out.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render("░"))
		}
	}
	return out.String()
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
