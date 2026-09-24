package ui

import (
	"fmt"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"swrm/internal/engine"
)

// The dashboard renders at a comfortable fixed size and centers within
// whatever terminal it's given, rather than stretching every card to fill
// an arbitrarily large window — a 240-column terminal shouldn't turn the
// swarm heatmap into a mile-wide smear.
const (
	maxDashboardWidth  = 150
	maxDashboardHeight = 51
)

func contentDims(width, height int) (int, int) {
	cw, ch := width, height
	if cw > maxDashboardWidth {
		cw = maxDashboardWidth
	}
	if ch > maxDashboardHeight {
		ch = maxDashboardHeight
	}
	return cw, ch
}

type sessionState int

const (
	stateSplash sessionState = iota
	stateDashboard
)

// focusTarget drives the 3-way Tab cycle: Top Magnet Input -> Left File
// Browser -> Right Inspector Card.
type focusTarget int

const (
	focusHeader focusTarget = iota
	focusFileBrowser
	focusInspector
)

type RootModel struct {
	state sessionState
	focus focusTarget

	Splash    SplashModel
	Dashboard DashboardView

	// FileTree/ShowDiagnostics stay as ad-hoc modal flags, not part of the
	// state enum: they're genuinely orthogonal overlays that can interrupt
	// stateDashboard independently of each other and of focus.
	FileTree        *FileTreeView
	fileTreeHash    metainfo.Hash // which torrent FileTree belongs to
	ShowDiagnostics bool
	// pendingFileTrees holds torrents whose metadata resolved while FileTree
	// was already open for another one; each gets its turn when it closes.
	pendingFileTrees []*torrent.Torrent

	Engine *engine.Engine

	Message    string
	MessageErr bool
	width      int
	height     int
	logoPhase  float64
}

func NewRootModel(eng *engine.Engine) RootModel {
	return RootModel{
		state:     stateSplash,
		Splash:    NewSplashModel(),
		Dashboard: NewDashboardView(),
		Engine:    eng,
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, m.Splash.Init())
}

// syncHeaderFocus focuses or blurs the header's textinput to match m.focus,
// so its cursor only blinks (and its own internal focus gate only accepts
// keystrokes) while the header is actually the active pane. Safe to call
// unconditionally — blurring an already-blurred input is a no-op.
func (m *RootModel) syncHeaderFocus() tea.Cmd {
	if m.focus == focusHeader {
		return m.Dashboard.Header.Input.Focus()
	}
	m.Dashboard.Header.Input.Blur()
	return nil
}

type metadataMsg struct {
	torrent *torrent.Torrent
	err     error
}

func waitForMetadata(t *torrent.Torrent) tea.Cmd {
	return func() tea.Msg {
		<-t.GotInfo()
		if t.Info() == nil {
			return metadataMsg{err: fmt.Errorf("torrent metadata unavailable")}
		}
		return metadataMsg{torrent: t}
	}
}

// dashboardTickMsg drives the once-per-second refresh: with search gone,
// there is no other periodic tick, and DL/UL/ETA/heatmap plus the animated
// progress bar all need one to stay live between keypresses.
type dashboardTickMsg struct{}

func dashboardTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return dashboardTickMsg{} })
}

// logoTickMsg drives the header wordmark's gradient shimmer on its own fast
// cadence, independent of the once-per-second data refresh above — the
// gradient sweep needs to actually read as motion, not a once-a-second jump.
type logoTickMsg struct{}

func logoTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return logoTickMsg{} })
}

func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.Splash, _ = m.Splash.Update(msg)
		if m.FileTree != nil {
			*m.FileTree, _ = m.FileTree.Update(msg)
		}
		cw, ch := contentDims(msg.Width, msg.Height)
		m.Dashboard = m.Dashboard.Resize(cw, ch)
		return m, nil
	case SplashFinishedMsg:
		// Each key pressed during the splash sends one of these, and so can
		// its final tick. Starting the tick loops below once per message
		// would stack duplicate loops (a double-speed logo, extra redraws)
		// for the rest of the session.
		if m.state != stateSplash {
			return m, nil
		}
		m.state = stateDashboard
		return m, tea.Batch(dashboardTick(), logoTick(), m.Dashboard.FileBrowser.Init(), m.syncHeaderFocus())
	case dashboardTickMsg:
		if m.state == stateDashboard {
			cmds = append(cmds, dashboardTick())
		}
		return m, tea.Batch(cmds...)
	case logoTickMsg:
		if m.state == stateDashboard {
			m.logoPhase += 0.02
			cmds = append(cmds, logoTick())
		}
		return m, tea.Batch(cmds...)
	case torrentFileSelectedMsg:
		t, err := m.Engine.AddTorrentFile(msg.path)
		if err != nil {
			m.Message, m.MessageErr = fmt.Sprintf("Add torrent file: %v", err), true
			return m, nil
		}
		m.Message, m.MessageErr = "Fetching torrent metadata…", false
		return m, waitForMetadata(t)
	case metadataMsg:
		if msg.err != nil {
			m.Message = msg.err.Error()
			m.MessageErr = true
			return m, nil
		}
		if m.FileTree != nil {
			// Another torrent's selection modal is still open. Replacing it
			// would drop that torrent's selection, and a torrent that never
			// gets one downloads nothing, so queue this one behind it.
			if hash := msg.torrent.InfoHash(); hash != m.fileTreeHash && !m.fileTreePending(hash) {
				m.pendingFileTrees = append(m.pendingFileTrees, msg.torrent)
			}
			return m, nil
		}
		m.openFileTree(msg.torrent)
		return m, nil
	case tea.KeyMsg:
		if m.ShowDiagnostics && m.FileTree == nil {
			// The diagnostics panel replaces the whole view, so it's modal:
			// keys mustn't reach the hidden dashboard behind it, where Space
			// would pause a torrent or Enter submit the header unseen.
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "d", "esc":
				m.ShowDiagnostics = false
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// Like the shortcuts below, "q" can be part of a magnet URI (a
			// dn= name, or a lowercase Base32 hash), so it only quits when
			// the header isn't taking text input.
			if m.state != stateSplash && (m.focus != focusHeader || m.FileTree != nil) {
				return m, tea.Quit
			}
		case "tab":
			if m.state == stateDashboard && m.FileTree == nil {
				m.focus = (m.focus + 1) % 3
				cmds = append(cmds, m.syncHeaderFocus())
			}
		case "shift+tab":
			if m.state == stateDashboard && m.FileTree == nil {
				m.focus = (m.focus + 2) % 3
				cmds = append(cmds, m.syncHeaderFocus())
			}
		case "down":
			// Up/Down alias Tab/Shift+Tab so the 3-way focus cycle isn't
			// Tab-only. Gated off when the file browser has focus since it
			// already needs up/down for its own list navigation — left/right
			// are similarly already claimed there (directory nav) and by the
			// header (text cursor) and inspector (torrent switcher), so this
			// alias only applies where up/down are otherwise idle.
			if m.state == stateDashboard && m.FileTree == nil && m.focus != focusFileBrowser {
				m.focus = (m.focus + 1) % 3
				return m, m.syncHeaderFocus()
			}
		case "up":
			if m.state == stateDashboard && m.FileTree == nil && m.focus != focusFileBrowser {
				m.focus = (m.focus + 2) % 3
				return m, m.syncHeaderFocus()
			}
		case "1", "2", "3", "d", " ":
			// These characters are also valid inside a pasted magnet URI or
			// hex/Base32 hash (e.g. "d" and "1"-"3" appear in hex), so they
			// only act as global shortcuts when the header does NOT have
			// focus; with header focus they fall through untouched to be
			// typed normally.
			if m.state == stateDashboard && m.FileTree == nil && m.focus != focusHeader {
				switch msg.String() {
				case "1":
					m.focus = focusHeader
				case "2":
					m.Dashboard.Inspector.Section = sectionGauges
				case "3":
					m.Dashboard.Inspector.Section = sectionSwarm
				case "d":
					m.ShowDiagnostics = true
				case " ":
					_ = m.Engine.TogglePauseHighlighted()
				}
				return m, m.syncHeaderFocus()
			}
		case "left":
			if m.state == stateDashboard && m.FileTree == nil && m.focus == focusInspector {
				m.Engine.HighlightPrev()
				return m, nil
			}
		case "right":
			if m.state == stateDashboard && m.FileTree == nil && m.focus == focusInspector {
				m.Engine.HighlightNext()
				return m, nil
			}
		case "enter":
			if m.state == stateDashboard && m.FileTree == nil && m.focus == focusHeader {
				raw := m.Dashboard.Header.Input.Value()
				uri, err := normalizeMagnetInput(raw)
				if err != nil {
					if raw != "" {
						m.Message, m.MessageErr = err.Error(), true
					}
				} else {
					t, addErr := m.Engine.AddMagnet(uri)
					if addErr != nil {
						m.Message, m.MessageErr = fmt.Sprintf("Add magnet: %v", addErr), true
					} else {
						m.Dashboard.Header.Input.SetValue("")
						m.Message, m.MessageErr = "Fetching torrent metadata…", false
						cmds = append(cmds, waitForMetadata(t))
					}
				}
			}
		}
	}

	if m.state == stateSplash {
		m.Splash, cmd = m.Splash.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.FileTree != nil {
		*m.FileTree, cmd = m.FileTree.Update(msg)
		if m.FileTree.Done {
			if !m.FileTree.Aborted {
				if h, ok := m.Engine.Get(m.fileTreeHash); ok {
					for i, file := range h.T.Files() {
						file.SetPriority(m.FileTree.Priorities[i])
					}
					m.Message = "File priorities applied"
					m.MessageErr = false
				}
			}
			m.FileTree = nil
			if len(m.pendingFileTrees) > 0 {
				next := m.pendingFileTrees[0]
				m.pendingFileTrees = m.pendingFileTrees[1:]
				m.openFileTree(next)
			}
		}
		cmds = append(cmds, cmd)
	} else {
		m.Dashboard, cmd = m.Dashboard.Update(msg, m.focus)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// openFileTree opens the file-selection modal for t, whose info has
// resolved, and highlights it.
func (m *RootModel) openFileTree(t *torrent.Torrent) {
	files := t.Files()
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.DisplayPath()
	}
	ft := NewFileTreeView(names)
	ft.SetSize(m.width, m.height)
	m.FileTree = &ft
	m.fileTreeHash = t.InfoHash()
	m.Engine.HighlightHash(t.InfoHash())
}

func (m RootModel) fileTreePending(hash metainfo.Hash) bool {
	for _, t := range m.pendingFileTrees {
		if t.InfoHash() == hash {
			return true
		}
	}
	return false
}

func (m RootModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing SWRM Engine..."
	}

	if m.state == stateSplash {
		return m.Splash.View()
	}

	snap := m.Engine.Snapshot()
	summaries := m.Engine.Summaries()

	body := m.Dashboard.View(m.focus, snap, summaries, m.logoPhase)

	vpnLabel := m.Engine.VpnManager.InterfaceName
	if vpnLabel == "" {
		vpnLabel = "system routing"
	}
	hud := renderHUD(m.focus, m.Dashboard.Inspector.Section, snap.VPNActive, vpnLabel)
	footer := renderFooter()

	// The message line is always there, empty or not (DashboardView.Resize
	// budgets for it), so a message appearing doesn't shift the layout.
	msgLine := ""
	if m.Message != "" {
		msgStyle := StyleAccentCyan
		if m.MessageErr {
			msgStyle = StyleDanger
		}
		msgLine = msgStyle.Render(m.Message)
	}
	view := body + "\n" + hud + "\n" + footer + "\n" + msgLine

	// No WithWhitespaceBackground here: forcing a canvas fill across the
	// Place() padding paints the *entire terminal* with ColorCanvas, hiding
	// the user's own terminal background everywhere outside the explicit UI
	// cards. Every card already paints its own Background(ColorSurface)
	// fill via RenderCard, so the space Place() adds around the centered
	// content should stay transparent to whatever background the user's
	// terminal actually has.
	if m.FileTree != nil {
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.FileTree.View())
	} else if m.ShowDiagnostics {
		diag := NewDiagnosticsView().View(snap)
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, diag)
	} else {
		// The dashboard itself is rendered at a capped, comfortable size
		// (contentDims) rather than stretched to fill the real terminal, so
		// it always centers within whatever window the user actually has.
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, view)
	}

	return view
}
