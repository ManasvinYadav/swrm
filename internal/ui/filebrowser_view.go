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
	// filepicker.View() concatenates each row as:
	//   Cursor.Render(" ") + " " + Permission.Render(mode) + FileSize.Render(size) + " " + File/Directory.Render(name)
	// Both " " here are literal, completely unstyled runes baked into the
	// vendored library's own View() — not reachable through Styles.* at
	// all — so they always fall back to the terminal's default background
	// no matter what we set below. Turning off the permission/size columns
	// (irrelevant for a .torrent-only browser anyway) removes the first gap
	// entirely and shrinks the row to a single unavoidable one-column seam
	// before the filename, rather than the two gaps plus a fully unstyled
	// ~10-char permission-string block DefaultStyles() ships with.
	fp.ShowPermissions = false
	fp.ShowSize = false

	// Every row style below carries its own explicit Background(ColorSurface)
	// rather than relying on RenderCard's outer Background to show through.
	// lipgloss/termenv concatenate raw ANSI codes rather than compositing
	// layers: each styled span (a colored filename, the cursor glyph) ends
	// with its own reset code, which clobbers whatever background the OUTER
	// card style set earlier on that line. Anything after that reset falls
	// back to the terminal's own default (near-black) background instead of
	// ColorSurface — visible as a dark patch behind every colored token.
	// Giving each inner style its own background closes that gap. Permission
	// and Symlink are set too even though the columns above are disabled —
	// DisabledSelected's "selected" row can still show them if disabled
	// somehow changes, and it keeps every Styles.* field consistent.
	fp.Styles.Selected = lipgloss.NewStyle().Background(ColorBorder).Foreground(lipgloss.Color("#ffffff")).Bold(true)
	fp.Styles.Directory = lipgloss.NewStyle().Foreground(ColorAccentBlue).Background(ColorSurface)
	fp.Styles.File = StylePrimary.Background(ColorSurface)
	fp.Styles.DisabledFile = StyleSlate.Background(ColorSurface)
	fp.Styles.DisabledSelected = StyleSlate.Background(ColorBorder)
	fp.Styles.Cursor = StyleAccentBlue.Background(ColorSurface)
	fp.Styles.FileSize = StyleSecondary.Background(ColorSurface)
	fp.Styles.Permission = StyleSlate.Background(ColorSurface)
	fp.Styles.Symlink = StyleAccentCyan.Background(ColorSurface)
	fp.Styles.EmptyDirectory = StyleSecondary.Background(ColorSurface).SetString("No .torrent files found.")

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
