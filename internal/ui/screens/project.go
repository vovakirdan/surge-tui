package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/fs"
	"surge-tui/internal/ui/components"
)

const (
	// UI layout constants
	TreePanelRatio   = 3.0 / 5.0 // 60% для дерева файлов
	StatusPanelRatio = 2.0 / 5.0 // 40% для статуса

	// Display constants
	MaxDisplayLines = 6 // Резерв строк для заголовков и рамок
	ScrollOffset    = 2 // Отступ при прокрутке

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
	StatusPanel
)

type projectInputMode int

const (
	projectInputNone projectInputMode = iota
	projectInputNewFile
	projectInputNewDir
	projectInputRename
)

// ProjectScreenReal настоящий экран проекта с деревом файлов
type ProjectScreenReal struct {
	BaseScreen

	// Состояние
	projectPath string
	fileTree    *fs.FileTree
	loading     bool
	err         error

	// UI состояние
	focusedPanel PanelType
	statusInfo   ProjectStatus
	inputMode    projectInputMode
	input        textinput.Model
	statusMsg    string
	statusAt     time.Time
	confirm      *components.ConfirmDialog

	// Размеры панелей
	treeWidth   int
	statusWidth int
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
func NewProjectScreenReal(projectPath string) *ProjectScreenReal {
	if projectPath == "" {
		// Используем текущую директорию если не указана
		pwd, _ := os.Getwd()
		projectPath = pwd
	}

	ti := textinput.New()
	ti.Placeholder = "name"
	ti.CharLimit = 256
	ti.Width = 32

	return &ProjectScreenReal{
		BaseScreen:   NewBaseScreen("Project"),
		projectPath:  projectPath,
		focusedPanel: FileTreePanel,
		loading:      true,
		input:        ti,
		confirm:      components.NewConfirmDialog("Delete", "Delete selected entry?"),
	}
}

// Init инициализирует экран
func (ps *ProjectScreenReal) Init() tea.Cmd {
	return ps.loadFileTree()
}

// Update обрабатывает сообщения
func (ps *ProjectScreenReal) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if ps.confirm != nil && ps.confirm.Visible {
		if cmd := ps.confirm.Update(msg); cmd != nil {
			return ps, cmd
		}
		if _, ok := msg.(tea.KeyMsg); ok {
			return ps, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if ps.inputMode != projectInputNone {
			return ps.handleInputKey(msg)
		}
		return ps.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ps.handleResize(msg)
		ps.input.Width = ps.treeWidth - 4
		return ps, nil
	case fileTreeLoadedMsg:
		ps.loading = false
		ps.fileTree = msg.tree
		ps.updateStats()
		return ps, nil
	case fileTreeErrorMsg:
		ps.loading = false
		ps.err = msg.err
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
	rightPanel := ps.renderStatusPanel()
	base := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	if ps.inputMode != projectInputNone {
		modal := ps.renderInputModal()
		return joinOverlay(base, modal)
	}

	if ps.confirm != nil {
		if view := ps.confirm.View(); view != "" {
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

	switch msg.String() {
	case "left", "right":
		ps.switchPanel()
		return ps, nil
	case "ctrl+r":
		return ps, ps.loadFileTree()
	case "h":
		if ps.fileTree != nil {
			ps.fileTree.SetShowHidden(!ps.fileTree.ShowHidden)
		}
		return ps, ps.loadFileTree()
	case "s":
		if ps.fileTree != nil {
			ps.fileTree.SetFilterSurge(!ps.fileTree.FilterSurge)
		}
		return ps, ps.loadFileTree()
	case "n":
		ps.beginInput(projectInputNewFile, "file name")
		return ps, nil
	case "N":
		ps.beginInput(projectInputNewDir, "directory name")
		return ps, nil
	case "r":
		if node := ps.fileTree.GetSelected(); node != nil {
			ps.beginInput(projectInputRename, node.Name)
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
	}

	// Навигация в дереве файлов
	if ps.focusedPanel == FileTreePanel && ps.fileTree != nil {
		switch msg.String() {
		case "up", "k":
			ps.fileTree.SetSelected(ps.fileTree.Selected - 1)
			return ps, nil
		case "down", "j":
			ps.fileTree.SetSelected(ps.fileTree.Selected + 1)
			return ps, nil
		case " ", "space":
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

	// Распределяем ширину панелей
	ps.treeWidth = int(float64(ps.Width()) * TreePanelRatio)
	ps.statusWidth = ps.Width() - ps.treeWidth
}

// switchPanel переключает фокус между панелями
func (ps *ProjectScreenReal) switchPanel() {
	if ps.focusedPanel == FileTreePanel {
		ps.focusedPanel = StatusPanel
	} else {
		ps.focusedPanel = FileTreePanel
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

	return func() tea.Msg {
		return OpenFileMsg{FilePath: selected.Path}
	}
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
	return "↑↓ Navigate • Enter Open • Space Expand • n New • r Rename • Del Delete"
}

// FullHelp возвращает полную справку
func (ps *ProjectScreenReal) FullHelp() []string {
	help := ps.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Project Screen:",
		"  ↑/↓ or j/k - Navigate files",
		"  ←/→ - Switch between tree and status panels",
		"  Enter - Open selected file / expand directory",
		"  Space - Expand/collapse directory",
		"  n - New file",
		"  Shift+N - New directory",
		"  r - Rename selected entry",
		"  Delete - Delete with confirmation",
		"  h - Toggle hidden files display",
		"  s - Toggle .sg files only filter",
		"  Ctrl+R - Refresh file tree",
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
