package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"swrm/internal/engine"
	"swrm/internal/server"
)

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

	Engine        *engine.Engine
	Pickers       map[metainfo.Hash]*engine.PiecePicker
	pickerCancels map[metainfo.Hash]context.CancelFunc

	StreamSrv        *server.StreamServer
	streamingHash    metainfo.Hash
	hasStreamingHash bool

	MediaPlayer string
	PostCmd     string
	Message     string
	MessageErr  bool
	width       int
	height      int
}

func NewRootModel(eng *engine.Engine, mediaPlayer, postCmd string) RootModel {
	return RootModel{
		state:         stateSplash,
		Splash:        NewSplashModel(),
		Dashboard:     NewDashboardView(),
		Engine:        eng,
		Pickers:       make(map[metainfo.Hash]*engine.PiecePicker),
		pickerCancels: make(map[metainfo.Hash]context.CancelFunc),
		MediaPlayer:   mediaPlayer,
		PostCmd:       postCmd,
	}
}

func (m RootModel) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, m.Splash.Init())
}

func (m *RootModel) Close() error {
	for _, cancel := range m.pickerCancels {
		cancel()
	}
	if m.StreamSrv == nil {
		return nil
	}
	return m.StreamSrv.Close(context.Background())
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

// resetStreaming tears down any in-flight stream server. It does not touch
// picker state — each torrent's picker lives as long as that torrent is
// tracked, independent of which one is being streamed.
func (m *RootModel) resetStreaming() {
	if m.StreamSrv != nil {
		_ = m.StreamSrv.Close(context.Background())
		m.StreamSrv = nil
	}
	m.hasStreamingHash = false
}

// triggerStreaming streams the currently highlighted torrent, reusing (or
// replacing, if a different torrent is now highlighted) any existing stream
// server.
func (m *RootModel) triggerStreaming() {
	h, ok := m.Engine.Highlighted()
	if !ok {
		m.Message, m.MessageErr = "No highlighted torrent to stream", true
		return
	}
	hash := h.T.InfoHash()
	if m.StreamSrv == nil || !m.hasStreamingHash || m.streamingHash != hash {
		m.resetStreaming()
		picker := m.Pickers[hash]
		if picker != nil {
			m.StreamSrv = server.NewStreamServer(h.T, 0, picker)
		} else {
			m.StreamSrv = server.NewStreamServer(h.T, 0)
		}
		if err := m.StreamSrv.Start(); err != nil {
			m.Message, m.MessageErr = err.Error(), true
			m.StreamSrv = nil
			return
		}
		m.streamingHash, m.hasStreamingHash = hash, true
	}

	streamURL := m.StreamSrv.URL()
	player, err := server.LaunchPlayer(streamURL, m.MediaPlayer)
	if err != nil {
		m.Message = fmt.Sprintf("Stream: %s (copied to clipboard)", streamURL)
		m.MessageErr = false
		_ = clipboard.WriteAll(streamURL)
	} else {
		m.Message = fmt.Sprintf("Launched %s", player)
		m.MessageErr = false
		engine.Notify("SWRM", "Stream Buffer Ready")
	}
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
		m.Dashboard = m.Dashboard.Resize(msg.Width, msg.Height)
		return m, nil
	case SplashFinishedMsg:
		m.state = stateDashboard
		return m, tea.Batch(dashboardTick(), m.Dashboard.FileBrowser.Init(), m.syncHeaderFocus())
	case dashboardTickMsg:
		if m.state == stateDashboard {
			snap := m.Engine.Snapshot()
			pct := 0.0
			if snap.Length > 0 {
				pct = float64(snap.Completed) / float64(snap.Length)
			}
			cmds = append(cmds, m.Dashboard.Inspector.Bar.SetPercent(pct), dashboardTick())
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
		hash := msg.torrent.InfoHash()
		files := msg.torrent.Files()
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.DisplayPath()
		}
		ft := NewFileTreeView(names)
		m.FileTree = &ft
		m.fileTreeHash = hash
		m.Engine.HighlightHash(hash)

		picker := engine.NewPiecePicker(msg.torrent)
		picker.UpdateOffset(0)
		ctx, cancel := context.WithCancel(context.Background())
		m.Pickers[hash] = picker
		m.pickerCancels[hash] = cancel
		go picker.StartEndgameMonitor(ctx)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state != stateSplash {
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
		case "1", "2", "3", "4", "d", " ":
			// These characters are also valid inside a pasted magnet URI or
			// hex/Base32 hash (e.g. "d" and "1"-"4" appear in hex), so they
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
				case "4":
					m.triggerStreaming()
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
		case "esc":
			if m.ShowDiagnostics {
				m.ShowDiagnostics = false
				return m, nil
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
						p := m.FileTree.Priorities[i]
						if p == 0 {
							file.SetPriority(torrent.PiecePriorityNone)
						} else {
							file.SetPriority(torrent.PiecePriorityNormal)
							file.Download()
						}
					}
					m.Message = "File priorities applied"
					m.MessageErr = false
				}
			}
			m.FileTree = nil
		}
		cmds = append(cmds, cmd)
	} else {
		m.Dashboard, cmd = m.Dashboard.Update(msg, m.focus)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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

	body := m.Dashboard.View(m.focus, snap, summaries)

	vpnLabel := m.Engine.VpnManager.InterfaceName
	if vpnLabel == "" {
		vpnLabel = "system routing"
	}
	hud := renderHUD(m.focus, m.Dashboard.Inspector.Section, snap.VPNActive, vpnLabel)
	footer := renderFooter()

	view := body + "\n" + hud + "\n" + footer

	if m.Message != "" {
		msgStyle := StyleAccentCyan
		if m.MessageErr {
			msgStyle = StyleDanger
		}
		view += "\n" + msgStyle.Render(m.Message)
	}

	if m.FileTree != nil {
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.FileTree.View())
	} else if m.ShowDiagnostics {
		var stalled []int
		if h, ok := m.Engine.Highlighted(); ok {
			if picker, ok2 := m.Pickers[h.T.InfoHash()]; ok2 {
				stalled = picker.StalledPieces()
			}
		}
		diag := NewDiagnosticsView().View(snap, stalled)
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, diag)
	}

	return view
}
