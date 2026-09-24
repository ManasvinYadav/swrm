package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	tea "github.com/charmbracelet/bubbletea"
	"swrm/internal/engine"
)

// TestDiagnosticsKeyGuardedByFileTree guards against a regression of the
// modal key-guard bug: 'd' (and by the same fix, s/[/]/{/}/0) must not act
// while the file-selection modal is open.
func TestDiagnosticsKeyGuardedByFileTree(t *testing.T) {
	ft := NewFileTreeView([]string{"a.txt"})
	// focus must be non-header: "d" is also a valid hex-hash character, so
	// it only acts as the diagnostics shortcut when the header doesn't have
	// keyboard focus (see RootModel.Update's combined "1","2","3","4","d",
	// " " case).
	m := RootModel{state: stateDashboard, focus: focusInspector, Dashboard: NewDashboardView(), FileTree: &ft}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	rm := next.(RootModel)
	if rm.ShowDiagnostics {
		t.Fatal("expected 'd' to be ignored while the file-tree modal is open")
	}

	rm.FileTree = nil
	next2, _ := rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	rm2 := next2.(RootModel)
	if !rm2.ShowDiagnostics {
		t.Fatal("expected 'd' to toggle diagnostics once the modal is closed")
	}

	next3, _ := rm2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm3 := next3.(RootModel)
	if rm3.ShowDiagnostics {
		t.Fatal("expected Esc to close the diagnostics modal")
	}
}

func keyRunes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// newTestEngine starts a real engine on system routing with a random port.
func newTestEngine(t *testing.T) *engine.Engine {
	t.Helper()
	vm, err := engine.NewVpnManager("", nil)
	if err != nil {
		t.Fatal(err)
	}
	eng, err := engine.NewEngine(vm, t.TempDir(), engine.Options{DownloadDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(eng.Close)
	return eng
}

// writeTestTorrent writes a one-file .torrent; distinct names give distinct
// infohashes.
func writeTestTorrent(t *testing.T, name string) string {
	t.Helper()
	info := metainfo.Info{
		PieceLength: 16 << 10,
		Name:        name,
		Files:       []metainfo.FileInfo{{Length: 16 << 10, Path: []string{"a.bin"}}},
		Pieces:      make([]byte, 20),
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name+".torrent")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := (&metainfo.MetaInfo{InfoBytes: infoBytes}).Write(f); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestQuitKeyTypedIntoFocusedHeader: "q" can be part of a magnet URI, so it
// must reach the magnet input rather than quit while that has focus.
func TestQuitKeyTypedIntoFocusedHeader(t *testing.T) {
	m := RootModel{state: stateDashboard, focus: focusHeader, Dashboard: NewDashboardView()}
	next, _ := m.Update(keyRunes("q"))
	if got := next.(RootModel).Dashboard.Header.Input.Value(); got != "q" {
		t.Fatalf("header input = %q after typing q, want %q", got, "q")
	}

	m.focus = focusInspector
	if _, cmd := m.Update(keyRunes("q")); cmd == nil {
		t.Fatal("expected q to quit when the header doesn't have focus")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected q to quit when the header doesn't have focus")
	}

	// Ctrl+C always quits, even mid-splash.
	m.state = stateSplash
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Fatal("expected ctrl+c to quit during the splash")
	} else if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected ctrl+c to quit during the splash")
	}
}

// TestDiagnosticsPanelIsModal: 'd' toggles the panel (as the README and the
// panel's own "(d or Esc to close)" hint say), and while it covers the
// screen no other key reaches the dashboard hidden behind it.
func TestDiagnosticsPanelIsModal(t *testing.T) {
	m := RootModel{state: stateDashboard, focus: focusInspector, Dashboard: NewDashboardView()}
	update := func(msg tea.Msg) {
		next, _ := m.Update(msg)
		m = next.(RootModel)
	}

	update(keyRunes("d"))
	if !m.ShowDiagnostics {
		t.Fatal("expected 'd' to open diagnostics")
	}
	update(tea.KeyMsg{Type: tea.KeyTab})
	update(keyRunes("1"))
	if m.focus != focusInspector || !m.ShowDiagnostics {
		t.Fatalf("keys leaked through the diagnostics panel: focus = %v, open = %v", m.focus, m.ShowDiagnostics)
	}
	update(keyRunes("d"))
	if m.ShowDiagnostics {
		t.Fatal("expected a second 'd' to close diagnostics")
	}
}

// TestSplashFinishedStartsDashboardOnce: a second SplashFinishedMsg (one is
// sent per key pressed during the splash) mustn't start a second set of
// tick loops.
func TestSplashFinishedStartsDashboardOnce(t *testing.T) {
	m := RootModel{state: stateSplash, Dashboard: NewDashboardView()}
	next, cmd := m.Update(SplashFinishedMsg{})
	if cmd == nil || next.(RootModel).state != stateDashboard {
		t.Fatal("expected the first SplashFinishedMsg to start the dashboard")
	}
	if _, cmd := next.Update(SplashFinishedMsg{}); cmd != nil {
		t.Fatal("a second SplashFinishedMsg started the dashboard's tick loops again")
	}
}

// TestMetadataQueuesBehindOpenFileTree: metadata resolving for a second
// torrent while the first's file-selection modal is open used to replace
// that modal, silently leaving the first torrent with no files selected, so
// it never downloaded anything.
func TestMetadataQueuesBehindOpenFileTree(t *testing.T) {
	eng := newTestEngine(t)
	first, err := eng.AddTorrentFile(writeTestTorrent(t, "first"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := eng.AddTorrentFile(writeTestTorrent(t, "second"))
	if err != nil {
		t.Fatal(err)
	}
	m := RootModel{state: stateDashboard, focus: focusInspector, Dashboard: NewDashboardView(), Engine: eng}
	update := func(msg tea.Msg) {
		next, _ := m.Update(msg)
		m = next.(RootModel)
	}

	update(metadataMsg{torrent: first})
	update(metadataMsg{torrent: second})
	update(metadataMsg{torrent: second}) // e.g. the same .torrent picked twice
	if m.FileTree == nil || m.fileTreeHash != first.InfoHash() {
		t.Fatal("expected the first torrent's modal to stay open while the second's metadata arrives")
	}

	update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := first.Files()[0].Priority(); got != torrent.PiecePriorityHigh {
		t.Fatalf("first torrent's selection wasn't applied: file priority %v", got)
	}
	if m.FileTree == nil || m.fileTreeHash != second.InfoHash() {
		t.Fatal("expected the second torrent's modal to open once the first's closed")
	}

	update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.FileTree != nil {
		t.Fatal("expected no third modal: a duplicate metadata message shouldn't queue twice")
	}
}
