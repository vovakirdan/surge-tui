package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/config"
	"surge-tui/internal/fs"
	"surge-tui/internal/platform"
	"surge-tui/internal/ui/components"
)

const (
	// UI layout constants
	TreeExpandedRatio  = 0.42 // Доля ширины, когда дерево в фокусе
	TreeCollapsedWidth = 26   // Минимальная ширина дерева, когда фокус в редакторе
	TreeMinWidth       = 18   // Минимально допустимая ширина дерева

	// Display constants
	ScrollOffset = 2 // Отступ при прокрутке

	// Colors
	ActiveBorderColor   = "#7C3AED"
	InactiveBorderColor = "#334155"
	LoadingColor        = "#7C3AED"
	ErrorColor          = "#EF4444"
	DimTextColor        = "#94A3B8"
)

// PanelType тип панели на экране
type PanelType int

const (
	FileTreePanel PanelType = iota
	EditorPanel
)

// ProjectScreenReal настоящий экран проекта с деревом файлов
type ProjectScreenReal struct {
	BaseScreen

	config *config.Config

	// Состояние
	projectPath string
	fileTree    *fs.FileTree
	loading     bool
	err         error

	// UI состояние
	focusedPanel  PanelType
	statusInfo    ProjectStatus
	statusMsg     string
	statusAt      time.Time
	confirm       *components.ConfirmDialog
	closeDialog   *components.ConfirmDialog
	newFileDialog *components.InputDialog
	newDirDialog  *components.InputDialog
	renameDialog  *components.InputDialog

	// Размеры панелей
	treeWidth int
	mainWidth int

	// Редактор и вкладки
	tabs           []*editorTab
	activeTab      int
	yankBuffer     string
	tabActiveStyle lipgloss.Style
	tabNormalStyle lipgloss.Style

	// Командная строка редактора
	editorCommand textinput.Model
}

// ProjectStatus информация о статусе проекта
type ProjectStatus struct {
	LastBuild    time.Time
	BuildSuccess bool
	ErrorCount   int
	WarningCount int
	FileCount    int
	DirCount     int
}

// NewProjectScreenReal создает новый экран проекта
func NewProjectScreenReal(projectPath string, cfg *config.Config) *ProjectScreenReal {
	if projectPath == "" {
		// Используем текущую директорию если не указана
		pwd, _ := os.Getwd()
		projectPath = pwd
	}

	cmdInput := textinput.New()
	cmdInput.Prompt = ":"
	cmdInput.CharLimit = 256
	cmdInput.Width = 40
	cmdInput.Blur()

	return &ProjectScreenReal{
		BaseScreen:     NewBaseScreen("Project"),
		config:         cfg,
		projectPath:    projectPath,
		focusedPanel:   FileTreePanel,
		loading:        true,
		confirm:        components.NewConfirmDialog("Delete", "Delete selected entry?"),
		closeDialog:    components.NewConfirmDialog("Close Tab", "Unsaved changes. Close anyway?"),
		newFileDialog:  components.NewInputDialog("New File", "Enter file name"),
		newDirDialog:   components.NewInputDialog("New Directory", "Enter directory name"),
		renameDialog:   components.NewInputDialog("Rename", "Enter new name"),
		editorCommand:  cmdInput,
		activeTab:      -1,
		tabActiveStyle: lipgloss.NewStyle().Background(lipgloss.Color("#7C3AED")).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1).Bold(true),
		tabNormalStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5F5")).Padding(0, 1),
	}
}

// Init инициализирует экран
func (ps *ProjectScreenReal) Init() tea.Cmd {
	return ps.loadFileTree()
}

// Update обрабатывает сообщения
func (ps *ProjectScreenReal) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if ps.closeDialog != nil && ps.closeDialog.Visible {
		if cmd := ps.closeDialog.Update(msg); cmd != nil {
			return ps, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return ps, nil
		}
	}

	if ps.confirm != nil && ps.confirm.Visible {
		if cmd := ps.confirm.Update(msg); cmd != nil {
			return ps, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return ps, nil
		}
	}

	if ps.newFileDialog != nil && ps.newFileDialog.Visible {
		if cmd := ps.newFileDialog.Update(msg); cmd != nil {
			return ps, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return ps, nil
		}
	}

	if ps.newDirDialog != nil && ps.newDirDialog.Visible {
		if cmd := ps.newDirDialog.Update(msg); cmd != nil {
			return ps, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return ps, nil
		}
	}

	if ps.renameDialog != nil && ps.renameDialog.Visible {
		if cmd := ps.renameDialog.Update(msg); cmd != nil {
			return ps, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return ps, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if ps.focusedPanel == EditorPanel && len(ps.tabs) > 0 {
			return ps.handleEditorKey(msg)
		}
		return ps.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ps.handleResize(msg)
		return ps, nil
	case fileTreeLoadedMsg:
		ps.loading = false
		ps.fileTree = msg.tree
		ps.updateStats()
		ps.recalculateLayout()
		return ps, nil
	case fileTreeErrorMsg:
		ps.loading = false
		ps.err = msg.err
		return ps, nil
	case closeTabConfirmedMsg:
		if msg.confirmed {
			ps.forceCloseTab(msg.index)
		}
		return ps, nil
	case deleteConfirmedMsg:
		if msg.confirmed {
			if err := ps.performDelete(msg.path); err != nil {
				ps.setStatus(err.Error())
			} else {
				ps.setStatus("Deleted")
			}
			return ps, ps.loadFileTree()
		}
		return ps, nil
	case newFileConfirmedMsg:
		if msg.value != nil && *msg.value != "" {
			if err := ps.createEntry(*msg.value, false); err != nil {
				ps.setStatus(err.Error())
			} else {
				ps.setStatus("File created")
			}
			return ps, ps.loadFileTree()
		}
		return ps, nil
	case newDirConfirmedMsg:
		if msg.value != nil && *msg.value != "" {
			if err := ps.createEntry(*msg.value, true); err != nil {
				ps.setStatus(err.Error())
			} else {
				ps.setStatus("Directory created")
			}
			return ps, ps.loadFileTree()
		}
		return ps, nil
	case renameConfirmedMsg:
		if msg.value != nil && *msg.value != "" {
			if err := ps.renameEntry(*msg.value); err != nil {
				ps.setStatus(err.Error())
			} else {
				ps.setStatus("Renamed")
			}
			return ps, ps.loadFileTree()
		}
		return ps, nil
	}

	return ps, nil
}

// View отрисовывает экран
func (ps *ProjectScreenReal) View() string {
	if ps.Width() == 0 {
		return "Initializing..."
	}

	if ps.loading {
		return ps.renderLoading()
	}

	if ps.err != nil {
		return ps.renderError()
	}

	// Разделяем экран на левую и правую панели
	leftPanel := ps.renderFileTreePanel()
	base := leftPanel
	if ps.mainWidth > 0 {
		rightPanel := ps.renderWorkspacePanel()
		base = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	}

	if ps.confirm != nil {
		if view := ps.confirm.View(); view != "" {
			return joinOverlay(base, view)
		}
	}

	if ps.closeDialog != nil {
		if view := ps.closeDialog.View(); view != "" {
			return joinOverlay(base, view)
		}
	}

	if ps.newFileDialog != nil {
		if view := ps.newFileDialog.View(); view != "" {
			return joinOverlay(base, view)
		}
	}

	if ps.newDirDialog != nil {
		if view := ps.newDirDialog.View(); view != "" {
			return joinOverlay(base, view)
		}
	}

	if ps.renameDialog != nil {
		if view := ps.renameDialog.View(); view != "" {
			return joinOverlay(base, view)
		}
	}

	return base
}

// handleKeyPress обрабатывает нажатия клавиш
func (ps *ProjectScreenReal) handleKeyPress(msg tea.KeyMsg) (Screen, tea.Cmd) {
	if ps.loading || ps.err != nil {
		// В состоянии загрузки только разрешаем выход
		return ps, nil
	}

	key := platform.CanonicalKeyForLookup(msg.String())

	switch key {
	case "ctrl+left":
		ps.focusedPanel = FileTreePanel
		ps.recalculateLayout()
		return ps, nil
	case "ctrl+right":
		if len(ps.tabs) > 0 {
			ps.focusedPanel = EditorPanel
			ps.recalculateLayout()
		}
		return ps, nil
	case "left", "right":
		ps.switchPanel()
		return ps, nil
	case "ctrl+r":
		return ps, ps.loadFileTree()
	case "h":
		if ps.fileTree != nil {
			ps.fileTree.SetShowHidden(!ps.fileTree.ShowHidden)
			ps.updateStats()
			if ps.fileTree.ShowHidden {
				ps.setStatus("Hidden entries visible")
			} else {
				ps.setStatus("Hidden entries hidden")
			}
		}
		return ps, nil
	case "s":
		if ps.fileTree != nil {
			ps.fileTree.SetFilterSurge(!ps.fileTree.FilterSurge)
			ps.updateStats()
			if ps.fileTree.FilterSurge {
				ps.setStatus("Filter: .sg only")
			} else {
				ps.setStatus("Filter: all files")
			}
		}
		return ps, nil
	case "n":
		if ps.newFileDialog != nil {
			ch := ps.newFileDialog.Show()
			return ps, func() tea.Msg {
				value := <-ch
				return newFileConfirmedMsg{value: value}
			}
		}
		return ps, nil
	case "N":
		if ps.newDirDialog != nil {
			ch := ps.newDirDialog.Show()
			return ps, func() tea.Msg {
				value := <-ch
				return newDirConfirmedMsg{value: value}
			}
		}
		return ps, nil
	case "r":
		if node := ps.fileTree.GetSelected(); node != nil && ps.renameDialog != nil {
			ch := ps.renameDialog.ShowWithValue(node.Name)
			return ps, func() tea.Msg {
				value := <-ch
				return renameConfirmedMsg{value: value}
			}
		}
		return ps, nil
	case "delete", "ctrl+d":
		if node := ps.fileTree.GetSelected(); node != nil && ps.confirm != nil {
			ps.confirm.Description = fmt.Sprintf("Delete %s?", node.Name)
			ch := ps.confirm.Show()
			path := node.Path
			return ps, func() tea.Msg {
				confirmed := <-ch
				return deleteConfirmedMsg{confirmed: confirmed, path: path}
			}
		}
		return ps, nil
	case "alt+enter":
		return ps, ps.openSelectedInEditor()
	}

	// Навигация в дереве файлов
	if ps.focusedPanel == FileTreePanel && ps.fileTree != nil {
		switch key {
		case "up", "k":
			ps.fileTree.SetSelected(ps.fileTree.Selected - 1)
			return ps, nil
		case "down", "j":
			ps.fileTree.SetSelected(ps.fileTree.Selected + 1)
			return ps, nil
		case "space":
			ps.fileTree.ToggleExpanded(ps.fileTree.Selected)
			ps.updateStats()
			return ps, nil
		case "enter":
			return ps, ps.openSelectedEntry()
		}
	}

	return ps, nil
}

// handleResize обрабатывает изменение размера
func (ps *ProjectScreenReal) handleResize(msg tea.WindowSizeMsg) {
	ps.SetSize(msg.Width, msg.Height-1) // -1 для статус-бара приложения
	ps.recalculateLayout()
}

// switchPanel переключает фокус между панелями
func (ps *ProjectScreenReal) switchPanel() {
	if ps.focusedPanel == FileTreePanel {
		if len(ps.tabs) > 0 {
			ps.focusedPanel = EditorPanel
		}
	} else {
		ps.focusedPanel = FileTreePanel
	}
	ps.recalculateLayout()
}

func (ps *ProjectScreenReal) recalculateLayout() {
	width := ps.Width()
	if width <= 0 {
		ps.treeWidth = 0
		ps.mainWidth = 0
		return
	}

	if len(ps.tabs) == 0 {
		ps.treeWidth = width
		ps.mainWidth = 0
		return
	}

	expanded := int(float64(width) * TreeExpandedRatio)
	if expanded < TreeMinWidth {
		expanded = TreeMinWidth
	}
	if expanded > width-TreeMinWidth {
		expanded = width - TreeMinWidth
	}

	collapsed := TreeCollapsedWidth
	if collapsed < TreeMinWidth {
		collapsed = TreeMinWidth
	}
	if collapsed > width-TreeMinWidth {
		collapsed = max(width-TreeMinWidth, TreeMinWidth)
	}

	treeWidth := collapsed
	if ps.focusedPanel == FileTreePanel {
		treeWidth = expanded
	}
	if treeWidth < TreeMinWidth {
		treeWidth = TreeMinWidth
	}
	if treeWidth > width-TreeMinWidth {
		treeWidth = width - TreeMinWidth
		if treeWidth < TreeMinWidth {
			treeWidth = max(TreeMinWidth, width/2)
		}
	}

	ps.treeWidth = treeWidth
	ps.mainWidth = width - treeWidth
	if ps.mainWidth < 0 {
		ps.mainWidth = 0
	}
}

// loadFileTree загружает дерево файлов асинхронно
func (ps *ProjectScreenReal) loadFileTree() tea.Cmd {
	ps.loading = true
	ps.err = nil
	ps.fileTree = nil
	return func() tea.Msg {
		tree, err := fs.NewFileTree(ps.projectPath)
		if err != nil {
			return fileTreeErrorMsg{err: err}
		}
		return fileTreeLoadedMsg{tree: tree}
	}
}

// openSelectedEntry обрабатывает Enter по выбранному элементу.
func (ps *ProjectScreenReal) openSelectedEntry() tea.Cmd {
	selected := ps.fileTree.GetSelected()
	if selected == nil {
		return nil
	}

	if selected.IsDir {
		ps.fileTree.ToggleExpanded(ps.fileTree.Selected)
		ps.updateStats()
		return nil
	}

	ps.openFileTab(selected.Path)
	return nil
}

func (ps *ProjectScreenReal) openSelectedInEditor() tea.Cmd {
	selected := ps.fileTree.GetSelected()
	if selected == nil || selected.IsDir {
		return nil
	}
	ps.openFileTab(selected.Path)
	return nil
}

// updateStats обновляет статистику проекта
func (ps *ProjectScreenReal) updateStats() {
	if ps.fileTree == nil {
		return
	}

	ps.statusInfo.FileCount = 0
	ps.statusInfo.DirCount = 0

	ps.countNodes(ps.fileTree.Root)
}

// countNodes рекурсивно считает файлы и директории
func (ps *ProjectScreenReal) countNodes(node *fs.FileNode) {
	if node == nil {
		return
	}

	if node.IsDir {
		ps.statusInfo.DirCount++
		for _, child := range node.Children {
			ps.countNodes(child)
		}
	} else {
		ps.statusInfo.FileCount++
	}
}

// Title возвращает заголовок экрана
func (ps *ProjectScreenReal) Title() string {
	if ps.projectPath != "" {
		return "Project: " + filepath.Base(ps.projectPath)
	}
	return "Project"
}

// ShortHelp возвращает краткую справку
func (ps *ProjectScreenReal) ShortHelp() string {
	return platform.ReplacePrimaryModifier("↑↓ Navigate • Enter Open • Space Expand • Ctrl+→ Editor • Alt+←/→ Tabs")
}

// FullHelp возвращает полную справку
func (ps *ProjectScreenReal) FullHelp() []string {
	help := ps.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Project Screen:",
		"  ↑/↓ or j/k - Navigate files",
		platform.ReplacePrimaryModifier("  Ctrl+→ - Focus editor • Ctrl+← - Focus tree"),
		"  Enter - Open selected file / expand directory",
		"  Space - Expand/collapse directory",
		"  n - New file",
		"  Shift+N - New directory",
		"  r - Rename selected entry",
		"  Delete - Delete with confirmation",
		"  h - Toggle hidden files display",
		"  s - Toggle .sg files only filter",
		platform.ReplacePrimaryModifier("  Ctrl+R - Refresh file tree"),
		"  Alt+←/→ - Switch editor tab • Alt+Shift+←/→ - Reorder tabs",
		"  yy / dd / p - Copy, cut, paste current line",
		"  :w save • :q quit tab • :q! force quit",
		"  i / Esc - Enter/exit insert mode (Vim style)",
	}...)
	return help
}

// Сообщения для экрана

type fileTreeLoadedMsg struct {
	tree *fs.FileTree
}

type fileTreeErrorMsg struct {
	err error
}

type closeTabConfirmedMsg struct {
	index     int
	confirmed bool
}

type newFileConfirmedMsg struct {
	value *string
}

type newDirConfirmedMsg struct {
	value *string
}

type renameConfirmedMsg struct {
	value *string
}
