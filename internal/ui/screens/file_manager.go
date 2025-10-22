package screens

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"surge-tui/internal/fs"
	"surge-tui/internal/ui/components"
)

type fileManagerMode int

const (
	fmModeBrowse fileManagerMode = iota
	fmModeNewFile
	fmModeNewDir
	fmModeRename
)

// FileManagerScreen предоставляет операции над файлами проекта.
type FileManagerScreen struct {
	BaseScreen

	rootPath string
	tree     *fs.FileTree
	loading  bool
	err      error

	mode     fileManagerMode
	input    textinput.Model
	status   string
	statusAt time.Time

	confirm *components.ConfirmDialog
}

// NewFileManagerScreen создаёт менеджер файлов с указанным корнем.
func NewFileManagerScreen(rootPath string) *FileManagerScreen {
	if rootPath == "" {
		cwd, _ := os.Getwd()
		rootPath = cwd
	}

	ti := textinput.New()
	ti.Placeholder = "name"
	ti.CharLimit = 256
	ti.Width = 32

	return &FileManagerScreen{
		BaseScreen: NewBaseScreen("File Manager"),
		rootPath:   rootPath,
		loading:    true,
		input:      ti,
		confirm:    components.NewConfirmDialog("Delete", "Delete selected entry?"),
	}
}

func (fm *FileManagerScreen) Init() tea.Cmd {
	return fm.reload()
}

func (fm *FileManagerScreen) OnEnter() tea.Cmd {
	if fm.tree == nil && !fm.loading {
		return fm.reload()
	}
	fm.ensureSelection()
	return nil
}

func (fm *FileManagerScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if fm.confirm != nil && fm.confirm.Visible {
		if cmd := fm.confirm.Update(msg); cmd != nil {
			return fm, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return fm, nil
		}
	}

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		fm.SetSize(m.Width, m.Height-1)
		fm.input.Width = fm.Width() - 8
		return fm, nil
	case tea.KeyMsg:
		if fm.loading {
			return fm, nil
		}
		if fm.mode != fmModeBrowse {
			return fm.handleInputKey(m)
		}
		return fm.handleBrowseKey(m)
	case fileTreeLoadedMsg:
		fm.loading = false
		fm.err = nil
		fm.tree = m.tree
		fm.ensureSelection()
		return fm, nil
	case fileTreeErrorMsg:
		fm.loading = false
		fm.err = m.err
		return fm, nil
	case deleteConfirmedMsg:
		if m.confirmed {
			if err := fm.performDelete(m.path); err != nil {
				fm.setStatus(err.Error())
			}
			return fm, fm.reload()
		}
		return fm, nil
	}
	return fm, nil
}

func (fm *FileManagerScreen) View() string {
	if fm.Width() == 0 {
		return "Initializing..."
	}
	if fm.loading {
		return centerText(fm.Width(), fm.Height(), fmt.Sprintf("🔄 Loading...\n\n%s", fm.rootPath))
	}
	if fm.err != nil {
		return centerText(fm.Width(), fm.Height(), fmt.Sprintf("❌ Error:\n%v", fm.err))
	}

	tree := fm.renderTree()
	info := fm.renderInfo()
	base := lipgloss.JoinHorizontal(lipgloss.Top, tree, info)

	if fm.mode != fmModeBrowse {
		modal := fm.renderInputModal()
		return overlay(base, modal)
	}
	if fm.confirm != nil {
		if view := fm.confirm.View(); view != "" {
			modal := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(view)
			return overlay(base, modal)
		}
	}
	return base
}

func (fm *FileManagerScreen) ShortHelp() string {
	return "↑↓ Navigate • Space Expand • Enter Open • N new file • Shift+N new dir • R rename • Del delete"
}

func (fm *FileManagerScreen) FullHelp() []string {
	help := fm.BaseScreen.FullHelp()
	help = append(help,
		"",
		"File Manager:",
		"  ↑/↓ or j/k - Navigate",
		"  Space - Expand/collapse directory",
		"  Enter - Open directory (File Manager) / open file (Editor)",
		"  N - New file in selected directory",
		"  Shift+N - New folder",
		"  R - Rename",
		"  Delete - Delete with confirmation",
		"  H - Toggle hidden files",
		"  S - Toggle .sg filter",
		"  Ctrl+R - Refresh",
	)
	return help
}

// --- commands and state ---

func (fm *FileManagerScreen) reload() tea.Cmd {
	fm.loading = true
	fm.err = nil
	fm.tree = nil
	return func() tea.Msg {
		tree, err := fs.NewFileTree(fm.rootPath)
		if err != nil {
			return fileTreeErrorMsg{err: err}
		}
		return fileTreeLoadedMsg{tree: tree}
	}
}

func (fm *FileManagerScreen) SetRoot(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	fm.rootPath = path
	return fm.reload()
}

func (fm *FileManagerScreen) ensureSelection() {
	if fm.tree == nil {
		return
	}
	fm.tree.SetSelected(fm.tree.Selected)
}

func (fm *FileManagerScreen) handleBrowseKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		fm.tree.SetSelected(fm.tree.Selected - 1)
		return fm, nil
	case "down", "j":
		fm.tree.SetSelected(fm.tree.Selected + 1)
		return fm, nil
	case "space", " ":
		fm.tree.ToggleExpanded(fm.tree.Selected)
		return fm, nil
	case "enter":
		node := fm.tree.GetSelected()
		if node == nil {
			return fm, nil
		}
		if node.IsDir {
			fm.rootPath = node.Path
			return fm, fm.reload()
		}
		return fm, func() tea.Msg { return OpenFileMsg{FilePath: node.Path} }
	case "n":
		fm.beginInput(fmModeNewFile, "file name")
		return fm, nil
	case "N":
		fm.beginInput(fmModeNewDir, "directory name")
		return fm, nil
	case "r":
		node := fm.tree.GetSelected()
		if node != nil {
			fm.beginInput(fmModeRename, node.Name)
		}
		return fm, nil
	case "delete", "ctrl+d":
		node := fm.tree.GetSelected()
		if node == nil {
			return fm, nil
		}
		if fm.confirm != nil {
			fm.confirm.Description = fmt.Sprintf("Delete %s?", node.Name)
			ch := fm.confirm.Show()
			path := node.Path
			return fm, func() tea.Msg {
				confirmed := <-ch
				return deleteConfirmedMsg{confirmed: confirmed, path: path}
			}
		}
		return fm, nil
	case "h":
		fm.tree.SetShowHidden(!fm.tree.ShowHidden)
		return fm, fm.reload()
	case "s":
		fm.tree.SetFilterSurge(!fm.tree.FilterSurge)
		return fm, fm.reload()
	case "ctrl+r":
		return fm, fm.reload()
	}
	return fm, nil
}

func (fm *FileManagerScreen) beginInput(mode fileManagerMode, placeholder string) {
	fm.mode = mode
	fm.input.Placeholder = placeholder
	if mode == fmModeRename {
		fm.input.SetValue(placeholder)
		fm.input.SetCursor(len(placeholder))
	} else {
		fm.input.SetValue("")
		fm.input.SetCursor(0)
	}
	fm.input.Focus()
}

func (fm *FileManagerScreen) handleInputKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		value := strings.TrimSpace(fm.input.Value())
		if value == "" {
			fm.setStatus("Name cannot be empty")
			fm.cancelInput()
			return fm, nil
		}
		if err := fm.performInput(value); err != nil {
			fm.setStatus(err.Error())
		} else {
			fm.setStatus("Done")
		}
		fm.cancelInput()
		return fm, fm.reload()
	case tea.KeyEsc:
		fm.cancelInput()
		return fm, nil
	}
	var cmd tea.Cmd
	fm.input, cmd = fm.input.Update(msg)
	return fm, cmd
}

func (fm *FileManagerScreen) cancelInput() {
	fm.mode = fmModeBrowse
	fm.input.SetValue("")
	fm.input.Blur()
}

func (fm *FileManagerScreen) performInput(name string) error {
	switch fm.mode {
	case fmModeNewFile:
		return fm.createEntry(name, false)
	case fmModeNewDir:
		return fm.createEntry(name, true)
	case fmModeRename:
		return fm.renameEntry(name)
	default:
		return nil
	}
}

func (fm *FileManagerScreen) selectedDirPath() string {
	if fm.tree == nil {
		return fm.rootPath
	}
	node := fm.tree.GetSelected()
	if node == nil {
		return fm.rootPath
	}
	if node.IsDir {
		return node.Path
	}
	if node.Parent != nil {
		return node.Parent.Path
	}
	return fm.rootPath
}

func (fm *FileManagerScreen) createEntry(name string, dir bool) error {
	if strings.ContainsAny(name, "/\\") {
		return errors.New("Name cannot contain separators")
	}
	targetDir := fm.selectedDirPath()
	path := filepath.Join(targetDir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", name)
	}
	if dir {
		return os.Mkdir(path, 0o755)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func (fm *FileManagerScreen) renameEntry(name string) error {
	node := fm.tree.GetSelected()
	if node == nil {
		return errors.New("Nothing selected")
	}
	if strings.ContainsAny(name, "/\\") {
		return errors.New("Name cannot contain separators")
	}
	newPath := filepath.Join(filepath.Dir(node.Path), name)
	if _, err := os.Stat(newPath); err == nil && !strings.EqualFold(newPath, node.Path) {
		return fmt.Errorf("%s already exists", name)
	}
	return os.Rename(node.Path, newPath)
}

func (fm *FileManagerScreen) performDelete(path string) error {
	return os.RemoveAll(path)
}

func (fm *FileManagerScreen) setStatus(msg string) {
	fm.status = msg
	fm.statusAt = time.Now()
}

// --- rendering helpers ---

func (fm *FileManagerScreen) renderTree() string {
	width := int(float64(fm.Width()) * 0.6)
	if width < 30 {
		width = fm.Width() - 2
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(width).Padding(1, 2)

	var rows []string
	if fm.tree != nil {
		for i, node := range fm.tree.FlatList {
			prefix := strings.Repeat("  ", node.Level)
			icon := "📄"
			if node.IsDir {
				if node.Expanded {
					icon = "📂"
				} else {
					icon = "📁"
				}
			}
			text := fmt.Sprintf("%s%s %s", prefix, icon, node.Name)
			if i == fm.tree.Selected {
				text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FACC15")).Render(text)
			}
			rows = append(rows, text)
		}
	}

	body := strings.Join(rows, "\n")
	filters := fm.filtersInfo()
	status := fm.statusLine()

	header := fmt.Sprintf("📁 %s", filepath.Base(fm.rootPath))
	return style.Render(fmt.Sprintf("%s\n%s\n\n%s\n\n%s", header, filters, body, status))
}

func (fm *FileManagerScreen) renderInfo() string {
	width := fm.Width() - int(float64(fm.Width())*0.6)
	if width < 24 {
		width = 24
	}
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(width).Padding(1, 2)

	if fm.tree == nil {
		return style.Render("No selection")
	}
	node := fm.tree.GetSelected()
	if node == nil {
		return style.Render("Select entry")
	}

	details := []string{
		fmt.Sprintf("Name: %s", node.Name),
		fmt.Sprintf("Path: %s", node.Path),
		fmt.Sprintf("Type: %s", map[bool]string{true: "Directory", false: "File"}[node.IsDir]),
		fmt.Sprintf("Size: %d bytes", node.Size),
	}
	if node.Parent != nil {
		details = append(details, fmt.Sprintf("Parent: %s", node.Parent.Path))
	}

	return style.Render(strings.Join(details, "\n"))
}

func (fm *FileManagerScreen) renderInputModal() string {
	prompt := fm.input.View()
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Enter: confirm • Esc: cancel")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(fmt.Sprintf("%s\n%s", prompt, hint))
}

func (fm *FileManagerScreen) filtersInfo() string {
	if fm.tree == nil {
		return "Filters: —"
	}
	var flags []string
	if fm.tree.ShowHidden {
		flags = append(flags, "Hidden")
	}
	if fm.tree.FilterSurge {
		flags = append(flags, ".sg only")
	}
	if len(flags) == 0 {
		return "Filters: none"
	}
	return "Filters: " + strings.Join(flags, ", ")
}

func (fm *FileManagerScreen) statusLine() string {
	if fm.status == "" {
		return ""
	}
	if time.Since(fm.statusAt) > 3*time.Second {
		fm.status = ""
		return ""
	}
	return fm.status
}

// --- utilities ---

func centerText(width, height int, text string) string {
	return lipgloss.NewStyle().Width(width).Height(height).Align(lipgloss.Center, lipgloss.Center).Render(text)
}

func overlay(base, modal string) string {
	return lipgloss.JoinVertical(lipgloss.Left, base, modal)
}

// delete confirmed message

type deleteConfirmedMsg struct {
	confirmed bool
	path      string
}
