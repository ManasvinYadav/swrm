package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/anacrolix/torrent"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"swrm/internal/engine"
)

// TestNormalizeMagnetInputAcceptsLowercaseBase32: the input check accepts a
// lowercase Base32 infohash, so the magnet it builds must parse too.
func TestNormalizeMagnetInputAcceptsLowercaseBase32(t *testing.T) {
	uri, err := normalizeMagnetInput(strings.Repeat("abcdefgh", 4))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := torrent.TorrentSpecFromMagnetUri(uri); err != nil {
		t.Fatalf("engine rejects %q: %v", uri, err)
	}
}

// TestDashboardFitsTerminal guards the layout budget: cards used to render
// two columns wider than asked (lipgloss's Width excludes borders), the
// status message and a long magnet in the header each added a line, and a
// busy inspector (wrapped peer rows, the switcher strip) grew past its deck.
// Any of these pushed the dashboard past the terminal's edges.
func TestDashboardFitsTerminal(t *testing.T) {
	eng := newTestEngine(t)
	snap := engine.Snapshot{
		Active: true, Name: strings.Repeat("Long.Torrent.Name.", 10),
		Length: 1 << 30, Completed: 1 << 29, PieceCount: 5000, Pieces: make([]engine.PieceSnapshot, 5000),
	}
	for i := 0; i < 100; i++ {
		snap.Peers = append(snap.Peers, engine.PeerSnapshot{Address: fmt.Sprintf("10.0.0.%d:6881", i), Client: "qBittorrent 4.6.5", DownloadRate: 1 << 20})
	}
	var summaries []engine.TorrentSummary
	for i := 0; i < 6; i++ {
		summaries = append(summaries, engine.TorrentSummary{Hash: fmt.Sprintf("%040x", i), Name: "Some.Torrent.Name", Highlighted: i == 5})
	}

	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 40}, {150, 51}, {200, 60}} {
		width, height := size[0], size[1]
		next, _ := NewRootModel(eng).Update(tea.WindowSizeMsg{Width: width, Height: height})
		next, _ = next.Update(SplashFinishedMsg{})
		m := next.(RootModel)
		m.Message = "Fetching torrent metadata…"
		m.Dashboard.Header.Input.SetValue("magnet:?xt=urn:btih:" + strings.Repeat("a", 300))
		view := m.View()
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w > width || h > height {
			t.Errorf("%dx%d terminal: view is %dx%d", width, height, w, h)
		}

		if height < 30 {
			continue // below the minimum deck height; the rest can't fit anyway
		}
		cw, ch := contentDims(width, height)
		body := m.Dashboard.View(focusInspector, snap, summaries, 0)
		// RootModel.View adds the HUD (3 lines), footer, and message line.
		if w, h := lipgloss.Width(body), lipgloss.Height(body)+5; w > cw || h > ch {
			t.Errorf("%dx%d terminal: busy dashboard is %dx%d, want at most %dx%d", width, height, w, h, cw, ch)
		}
	}
}

// TestETAFollowsSelectedFiles: files the user skipped never download, so
// once everything selected is done the transfer is done.
func TestETAFollowsSelectedFiles(t *testing.T) {
	snap := engine.Snapshot{Length: 4 << 30, Completed: 1 << 30, Selected: 1 << 30, SelectedCompleted: 1 << 30}
	if got := formatETA(snap); got != "done" {
		t.Fatalf("ETA with every selected byte downloaded = %q, want %q", got, "done")
	}
}
