package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/charmbracelet/lipgloss"
	"swrm/internal/engine"
)

type inspectorSection int

const (
	sectionGauges inspectorSection = iota
	sectionSwarm
)

// InspectorView is the right-deck "Active inspector and transfer card": a
// torrent switcher strip (when more than one torrent is tracked), the
// animated gradient progress bar, transfer gauges, the swarm heatmap, and
// the connected-peer list.
type InspectorView struct {
	Section inspectorSection
}

func NewInspectorView() InspectorView {
	return InspectorView{Section: sectionGauges}
}

// renderSwitcherStrip renders one pill per torrent, at most maxWidth cells
// wide. When they don't all fit, it shows the run of neighbours around the
// highlighted pill that does, so Left/Right never moves the highlight
// off-screen.
func renderSwitcherStrip(summaries []engine.TorrentSummary, maxWidth int) string {
	if len(summaries) < 2 {
		return ""
	}
	pills := make([]string, len(summaries))
	hi := 0
	for i, s := range summaries {
		name := s.Name
		if name == "" {
			name = "fetching…"
		}
		tag := ""
		if s.Paused {
			tag = "⏸ "
		}
		label := fmt.Sprintf(" %s%s [%s] ", tag, truncate(name, 20), s.Hash[:8])
		style := lipgloss.NewStyle().Foreground(ColorTextSecondary).
			Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder)
		if s.Highlighted {
			style = lipgloss.NewStyle().Background(ColorAccentBlue).Foreground(lipgloss.Color("#ffffff")).Bold(true).
				Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccentBlue)
			hi = i
		}
		pills[i] = style.Render(label)
	}
	lo, end := hi, hi+1
	used := lipgloss.Width(pills[hi])
	for grew := true; grew; {
		grew = false
		if end < len(pills) && used+lipgloss.Width(pills[end]) <= maxWidth {
			used += lipgloss.Width(pills[end])
			end++
			grew = true
		}
		if lo > 0 && used+lipgloss.Width(pills[lo-1]) <= maxWidth {
			used += lipgloss.Width(pills[lo-1])
			lo--
			grew = true
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, pills[lo:end]...) + "\n\n"
}

// switcherStripHeight is how many lines renderSwitcherStrip's output takes:
// a row of bordered pills plus the blank spacer line after it.
const switcherStripHeight = 4

// formatRate renders an actual transfer rate, where zero legitimately means
// "no throughput right now."
func formatRate(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	}
	if bytesPerSec >= 1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	}
	return fmt.Sprintf("%.0f B/s", bytesPerSec)
}

func formatETA(snap engine.Snapshot) string {
	done, total := snap.Progress()
	if total <= 0 {
		return "—"
	}
	remaining := total - done
	if remaining <= 0 {
		return "done"
	}
	if snap.DownloadRate <= 0 {
		return "—"
	}
	d := time.Duration(float64(remaining) / snap.DownloadRate * float64(time.Second))
	h := int(d.Hours())
	mnt := int(d.Minutes()) % 60
	sec := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, mnt)
	case mnt > 0:
		return fmt.Sprintf("%dm %ds", mnt, sec)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

// generateBitfield renders the swarm availability heatmap: one glyph per
// piece, colored by the piece-heat buckets, wrapped to maxWidth columns.
func generateBitfield(pieces []engine.PieceSnapshot, maxWidth int) string {
	if len(pieces) == 0 || maxWidth <= 0 {
		return ""
	}
	var sb strings.Builder
	for i, p := range pieces {
		if i > 0 && i%maxWidth == 0 {
			sb.WriteString("\n")
		}
		switch {
		case p.Complete:
			sb.WriteString(StylePieceDone.Render("█"))
		case p.Priority == torrent.PiecePriorityHigh || p.Priority == torrent.PiecePriorityReadahead:
			sb.WriteString(StylePieceHot.Render("▒"))
		case p.Priority == torrent.PiecePriorityNormal:
			sb.WriteString(StylePieceWarm.Render("░"))
		default:
			sb.WriteString(StylePieceCold.Render("·"))
		}
	}
	return sb.String()
}

// renderPeerList renders at most maxRows lines, the last of them an "… and
// N more" summary when not every peer fits.
func renderPeerList(peers []engine.PeerSnapshot, maxRows int) string {
	if len(peers) == 0 {
		return StyleSecondary.Render("No connected peers.")
	}
	shown := peers
	if len(peers) > maxRows {
		shown = peers[:max(maxRows-1, 0)]
	}
	rows := make([]string, 0, maxRows)
	for _, p := range shown {
		client := p.Client
		if client == "" {
			client = "Unknown"
		}
		client = truncate(client, 18)
		health := StyleAccentCyan.Render("●")
		if p.DownloadRate == 0 && p.UploadRate == 0 {
			health = StyleSlate.Render("●")
		}
		// Pad the rates before styling them: %-10s on an already-styled
		// string counts its escape codes as width and never pads.
		rows = append(rows, fmt.Sprintf("%s %-20s %-18s UL: %s DL: %s",
			health, p.Address, client,
			StyleAmber.Render(fmt.Sprintf("%-10s", formatRate(p.UploadRate))),
			StyleAmber.Render(fmt.Sprintf("%-10s", formatRate(p.DownloadRate))),
		))
	}
	if len(shown) < len(peers) {
		rows = append(rows, StyleSecondary.Render(fmt.Sprintf("… and %d more", len(peers)-len(shown))))
	}
	return strings.Join(rows, "\n")
}

func (m InspectorView) View(width, height int, snap engine.Snapshot, summaries []engine.TorrentSummary, focused bool, logoPhase float64) string {
	innerWidth := width - cardChromeWidth
	var sb strings.Builder
	strip := renderSwitcherStrip(summaries, innerWidth)
	sb.WriteString(strip)

	if !snap.Active {
		sb.WriteString(StyleSecondary.Render("No active transfer. Add a magnet or .torrent file to begin."))
		return RenderCard("Active inspector and transfer card", sb.String(), width, height, focused)
	}
	if snap.PieceCount == 0 {
		sb.WriteString(StyleAccentPurple.Bold(true).Render("⧗ Fetching torrent metadata…"))
		return RenderCard("Active inspector and transfer card", sb.String(), width, height, focused)
	}

	progressPct := 0.0
	if done, total := snap.Progress(); total > 0 {
		progressPct = float64(done) / float64(total)
	}

	pausedTag := ""
	if snap.Paused {
		pausedTag = " [PAUSED]"
	}
	// Shorten the name, not the size and paused tag after it, when the line
	// is too long for the card.
	sizeTag := fmt.Sprintf(" [ %.1f MB ]", float64(snap.Length)/(1024*1024))
	name := truncate(snap.Name, max(innerWidth-len([]rune(sizeTag))-len(pausedTag), 1))
	nameLine := StyleAccentBlue.Bold(true).Render(name + sizeTag)
	if pausedTag != "" {
		nameLine += StyleDanger.Render(pausedTag)
	}

	// Leave room for the " 100%" label after the bar.
	barWidth := innerWidth - 5
	if barWidth < 5 {
		barWidth = 5
	}
	bar := fmt.Sprintf("%s %.0f%%", RenderAnimatedBar(barWidth, progressPct, logoPhase), progressPct*100)

	gaugesLabel := "Transfer:"
	if m.Section == sectionGauges {
		gaugesLabel = StyleAccentBlue.Bold(true).Underline(true).Render(gaugesLabel)
	} else {
		gaugesLabel = StyleAccentBlue.Bold(true).Render(gaugesLabel)
	}
	gaugesLine := fmt.Sprintf("↑ DL: %s | ↓ UL: %s | ETA: %s | Peers: %d",
		StyleAccentCyan.Render(formatRate(snap.DownloadRate)),
		StyleAccentCyan.Render(formatRate(snap.UploadRate)),
		StyleAmber.Render(formatETA(snap)), len(snap.Peers),
	)

	sb.WriteString(fmt.Sprintf("%s\n%s\n\n%s\n%s\n\n", nameLine, bar, gaugesLabel, gaugesLine))

	swarmLabel := "Swarm availability heatmap:"
	if m.Section == sectionSwarm {
		swarmLabel = StyleAccentBlue.Bold(true).Underline(true).Render(swarmLabel)
	} else {
		swarmLabel = StyleAccentBlue.Bold(true).Render(swarmLabel)
	}
	sb.WriteString(swarmLabel + "\n")

	bf := generateBitfield(snap.Pieces, innerWidth)
	bfLines := strings.Split(bf, "\n")
	const maxHeatmapRows = 2
	if len(bfLines) > maxHeatmapRows {
		bfLines = bfLines[:maxHeatmapRows]
	}
	sb.WriteString(strings.Join(bfLines, "\n") + "\n\n")

	sb.WriteString(StyleAccentBlue.Bold(true).Render("Connected peers:") + "\n")
	// Everything above the peer list: name, bar, blank, transfer label and
	// gauges, blank, heatmap label, up to maxHeatmapRows of heatmap, blank,
	// peers label — plus the switcher strip when it's showing.
	const fixedLines = 9 + maxHeatmapRows
	peerRows := height - cardPaddingHeight - fixedLines
	if strip != "" {
		peerRows -= switcherStripHeight
	}
	if peerRows < 1 {
		peerRows = 1
	}
	sb.WriteString(renderPeerList(snap.Peers, peerRows))

	return RenderCard("Active inspector and transfer card", sb.String(), width, height, focused)
}
