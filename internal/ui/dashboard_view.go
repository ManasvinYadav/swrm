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

	const headerHeight = 3
	const hudHeight = 1
	const footerHeight = 1
	d.deckHeight = height - headerHeight - hudHeight - footerHeight
	if d.deckHeight < 6 {
		d.deckHeight = 6
	}

	leftWidth := width / 2
	rightWidth := width - leftWidth

	d.Header.Input.Width = width - 8
	d.FileBrowser.Picker.SetHeight(d.deckHeight - 3)
	d.Inspector.Bar.Width = rightWidth - 12
	if d.Inspector.Bar.Width < 5 {
		d.Inspector.Bar.Width = 5
	}
	return d
}

// Update routes key messages only to the focused child (so, e.g., typing a
// hex hash into the header doesn't leak arrow-key navigation into an
// unfocused file browser), but forwards every other message type (async
// directory listings, cursor blink ticks, progress bar frames) to all
// children unconditionally so their internal state stays current regardless
// of focus.
func (d DashboardView) Update(msg tea.Msg, focus focusTarget) (DashboardView, tea.Cmd) {
	var cmds []tea.Cmd

	inspector, cmd := d.Inspector.Update(msg)
	d.Inspector = inspector
	cmds = append(cmds, cmd)

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

func (d DashboardView) View(focus focusTarget, snap engine.Snapshot, summaries []engine.TorrentSummary) string {
	header := d.Header.View(d.width, focus == focusHeader)

	leftWidth := d.width / 2
	rightWidth := d.width - leftWidth

	left := d.FileBrowser.View(leftWidth, d.deckHeight, focus == focusFileBrowser)
	right := d.Inspector.View(rightWidth, d.deckHeight, snap, summaries, focus == focusInspector)
	deck := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	return lipgloss.JoinVertical(lipgloss.Left, header, deck)
}
