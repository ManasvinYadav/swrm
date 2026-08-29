package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"swrm/internal/engine"
)

// torrentFileSelectedMsg is emitted when the file browser's Enter selects a
// .torrent file. RootModel is the only place that calls into Engine, so this
// carries the intent back up rather than DashboardView calling Engine itself.
type torrentFileSelectedMsg struct{ path string }

// DashboardView composes the header, left file-browser deck, and right
// inspector deck into the persistent 50/50 layout. Nav pills never swap
// this out for a different screen — see hud_view.go.
type DashboardView struct {
	Header      HeaderInput
	FileBrowser FileBrowserView
	Inspector   InspectorView

	width, height, deckHeight int
}

func NewDashboardView() DashboardView {
	return DashboardView{
		Header:      NewHeaderInput(),
		FileBrowser: NewFileBrowserView(),
		Inspector:   NewInspectorView(),
	}
}

// Resize pushes concrete widths/heights into each child directly, rather
// than forwarding the raw WindowSizeMsg — bubbles/filepicker in particular
// assumes on a raw resize that it owns the whole terminal height.
func (d DashboardView) Resize(width, height int) DashboardView {
	d.width, d.height = width, height

	const logoHeight = 7 // 6-line ascii wordmark + 1 blank spacer
	const headerHeight = 3
	const hudHeight = 3 // nav pills are now bordered boxes: top rule + text + bottom rule
	const footerHeight = 1
	// RenderCard renders height_param+2 total lines: its own top-rule line,
	// plus a bottom border line added by lipgloss.Style.Render *after* the
	// Height() field is applied (verified against the vendored lipgloss
	// source — border is appended post-height, not counted within it). Both
	// deck cards are rendered at d.deckHeight, so the budget must reserve 2
	// extra rows or the composed view overflows the terminal by exactly 2
	// lines on every resize, corrupting the header/HUD/footer beneath it.
	const cardChrome = 2
	d.deckHeight = height - logoHeight - headerHeight - hudHeight - footerHeight - cardChrome
	if d.deckHeight < 6 {
		d.deckHeight = 6
	}

	// HeaderInput.View wraps the textinput in Border (2 cols) + Padding(0,2)
	// (4 cols) = 6 cols of chrome; the textinput's own Width must match that
	// same interior budget or the two disagree about how much space is
	// actually available, producing a stray misaligned fill at the edge.
	d.Header.Input.Width = width - 6
	d.FileBrowser.Picker.SetHeight(d.deckHeight - 3)
	return d
}

// Update routes key messages only to the focused child (so, e.g., typing a
// hex hash into the header doesn't leak arrow-key navigation into an
// unfocused file browser), but forwards every other message type (async
// directory listings, cursor blink ticks) to all children unconditionally so
// their internal state stays current regardless of focus.
func (d DashboardView) Update(msg tea.Msg, focus focusTarget) (DashboardView, tea.Cmd) {
	var cmds []tea.Cmd

	_, isKey := msg.(tea.KeyMsg)

	if !isKey || focus == focusHeader {
		header, cmd := d.Header.Update(msg)
		d.Header = header
		cmds = append(cmds, cmd)
	}
	if !isKey || focus == focusFileBrowser {
		fb, cmd, selected := d.FileBrowser.Update(msg)
		d.FileBrowser = fb
		cmds = append(cmds, cmd)
		if selected != "" {
			path := selected
			cmds = append(cmds, func() tea.Msg { return torrentFileSelectedMsg{path: path} })
		}
	}

	return d, tea.Batch(cmds...)
}

func (d DashboardView) View(focus focusTarget, snap engine.Snapshot, summaries []engine.TorrentSummary, logoPhase float64) string {
	logo := lipgloss.NewStyle().Width(d.width).Align(lipgloss.Center).Render(RenderGradientBanner(asciiBanner, logoPhase))
	header := d.Header.View(d.width, focus == focusHeader)

	leftWidth := d.width / 2
	rightWidth := d.width - leftWidth

	left := d.FileBrowser.View(leftWidth, d.deckHeight, focus == focusFileBrowser)
	right := d.Inspector.View(rightWidth, d.deckHeight, snap, summaries, focus == focusInspector, logoPhase)
	deck := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, logo, "", header, deck)
}
