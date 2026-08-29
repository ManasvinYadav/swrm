package ui

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileBrowserView is the left-deck local filesystem browser, filtered to
// .torrent files only.
type FileBrowserView struct {
	Picker filepicker.Model
}

func NewFileBrowserView() FileBrowserView {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".torrent"}
	fp.FileAllowed = true
	fp.DirAllowed = false
	// We own height explicitly via Resize/SetHeight — AutoHeight assumes the
	// picker owns the whole terminal height, which breaks the 50/50 deck.
	fp.AutoHeight = false

	fp.Styles.Selected = lipgloss.NewStyle().Background(ColorBorder).Foreground(lipgloss.Color("#ffffff")).Bold(true)
	fp.Styles.Directory = lipgloss.NewStyle().Foreground(ColorAccentBlue)
	fp.Styles.File = StylePrimary
	fp.Styles.DisabledFile = StyleSlate
	fp.Styles.DisabledSelected = StyleSlate
	fp.Styles.Cursor = StyleAccentBlue
	fp.Styles.FileSize = StyleSecondary
	fp.Styles.EmptyDirectory = StyleSecondary.SetString("No .torrent files found.")

	if home, err := os.UserHomeDir(); err == nil {
		dl := filepath.Join(home, "Downloads")
		if info, statErr := os.Stat(dl); statErr == nil && info.IsDir() {
			fp.CurrentDirectory = dl
		} else if cwd, cerr := os.Getwd(); cerr == nil {
			fp.CurrentDirectory = cwd
		}
	} else if cwd, cerr := os.Getwd(); cerr == nil {
		fp.CurrentDirectory = cwd
	}
	return FileBrowserView{Picker: fp}
}

func (m FileBrowserView) Init() tea.Cmd {
	return m.Picker.Init()
}

// Update forwards msg to the underlying picker and reports the selected
// .torrent file's path, or "" if nothing was selected this update.
func (m FileBrowserView) Update(msg tea.Msg) (FileBrowserView, tea.Cmd, string) {
	var cmd tea.Cmd
	m.Picker, cmd = m.Picker.Update(msg)
	if didSelect, path := m.Picker.DidSelectFile(msg); didSelect {
		return m, cmd, path
	}
	return m, cmd, ""
}

func (m FileBrowserView) View(width, height int, focused bool) string {
	return RenderCard("Inbuilt .torrent file browser", m.Picker.View(), width, height, focused)
}
