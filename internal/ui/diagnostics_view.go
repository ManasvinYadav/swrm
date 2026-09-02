package ui

import (
	"fmt"
	"strings"

	"github.com/anacrolix/torrent"
	"swrm/internal/engine"
)

// DiagnosticsView is the swarm-health & dead-torrent diagnostics modal,
// opened with 'd'. It renders a piece-state breakdown and peer connectivity
// — everything anacrolix/torrent's public API actually exposes. It
// intentionally does not report per-tracker announce status: the library
// doesn't export that surface, and fabricating it would be worse than
// leaving it out.
type DiagnosticsView struct{}

func NewDiagnosticsView() DiagnosticsView { return DiagnosticsView{} }

func (d DiagnosticsView) View(snap engine.Snapshot) string {
	var sb strings.Builder
	sb.WriteString(StyleSecondary.Render("(d or Esc to close)") + "\n\n")

	if !snap.Active {
		sb.WriteString(StyleSecondary.Render("No active transfer to diagnose.\n"))
		return RenderCard("DIAGNOSTICS", sb.String(), 0, 0, true)
	}
	if snap.PieceCount == 0 {
		sb.WriteString(StyleSecondary.Render("Fetching torrent metadata…\n"))
		return RenderCard("DIAGNOSTICS", sb.String(), 0, 0, true)
	}

	var complete, urgent, prefetch, none int
	for _, p := range snap.Pieces {
		switch {
		case p.Complete:
			complete++
		case p.Priority == torrent.PiecePriorityHigh || p.Priority == torrent.PiecePriorityReadahead:
			urgent++
		case p.Priority == torrent.PiecePriorityNormal:
			prefetch++
		default:
			none++
		}
	}
	sb.WriteString(StyleAccentBlue.Render("Piece state:") + "\n")
	sb.WriteString(fmt.Sprintf("  %s verified   %s urgent   %s prefetch   %s not wanted   (%d total)\n\n",
		StylePieceDone.Render(fmt.Sprintf("%d", complete)),
		StylePieceHot.Render(fmt.Sprintf("%d", urgent)),
		StylePieceWarm.Render(fmt.Sprintf("%d", prefetch)),
		StylePieceCold.Render(fmt.Sprintf("%d", none)),
		snap.PieceCount,
	))

	sb.WriteString(StyleAccentBlue.Render("Swarm connectivity:") + "\n")
	sb.WriteString(fmt.Sprintf("  %d peer(s) connected", len(snap.Peers)))
	if len(snap.Peers) > 0 {
		active := 0
		for _, p := range snap.Peers {
			if p.DownloadRate > 0 {
				active++
			}
		}
		sb.WriteString(fmt.Sprintf(", %d actively sending data", active))
	}
	sb.WriteString("\n")
	if !snap.VPNActive {
		sb.WriteString(StyleDanger.Render("  VPN interface is DOWN — transfers halted for leak prevention.\n"))
	}

	return RenderCard("DIAGNOSTICS", sb.String(), 0, 0, true)
}
