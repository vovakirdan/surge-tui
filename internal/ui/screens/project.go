package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/fs"
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

// OpenFileMsg сообщение об открытии файла
type OpenFileMsg struct {
	FilePath string
}

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

	return &ProjectScreenReal{
		BaseScreen:   NewBaseScreen("Project"),
		projectPath:  projectPath,
		focusedPanel: FileTreePanel,
		loading:      true,
	}
}

// Init инициализирует экран
func (ps *ProjectScreenReal) Init() tea.Cmd {
	return ps.loadFileTree()
}

// Update обрабатывает сообщения
func (ps *ProjectScreenReal) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ps.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ps.handleResize(msg)
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

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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
	case "h": // Переключение показа скрытых файлов
		if ps.fileTree != nil {
			ps.fileTree.SetShowHidden(!ps.fileTree.ShowHidden)
			ps.updateStats()
		}
		return ps, nil
	case "s": // Фильтр по .sg файлам
		if ps.fileTree != nil {
			ps.fileTree.SetFilterSurge(!ps.fileTree.FilterSurge)
			ps.updateStats()
		}
		return ps, nil
	case "r", "ctrl+r": // Обновить дерево
		return ps, ps.loadFileTree()
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
		case "enter":
			return ps, ps.openSelectedFile()
		case " ", "space":
			ps.fileTree.ToggleExpanded(ps.fileTree.Selected)
			ps.updateStats()
			return ps, nil
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
	return func() tea.Msg {
		tree, err := fs.NewFileTree(ps.projectPath)
		if err != nil {
			return fileTreeErrorMsg{err: err}
		}
		return fileTreeLoadedMsg{tree: tree}
	}
}

// openSelectedFile открывает выбранный файл
func (ps *ProjectScreenReal) openSelectedFile() tea.Cmd {
	selected := ps.fileTree.GetSelected()
	if selected == nil {
		return nil
	}

	if selected.IsDir {
		// Для директорий переключаем состояние разворота
		ps.fileTree.ToggleExpanded(ps.fileTree.Selected)
		ps.updateStats()
		return nil
	}

	// Для файлов отправляем сообщение об открытии
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

// renderLoading отрисовывает экран загрузки
func (ps *ProjectScreenReal) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(ps.Width()).
		Height(ps.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(LoadingColor))

	return style.Render("🔄 Loading project...\n\n" + ps.projectPath)
}

// renderError отрисовывает экран ошибки
func (ps *ProjectScreenReal) renderError() string {
	style := lipgloss.NewStyle().
		Width(ps.Width()).
		Height(ps.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(ErrorColor))

	return style.Render(fmt.Sprintf("❌ Error loading project\n\n%s\n\n%v\n\nPress 'r' to retry", ps.projectPath, ps.err))
}

// renderFileTreePanel отрисовывает панель дерева файлов
func (ps *ProjectScreenReal) renderFileTreePanel() string {
	// Стиль рамки
	borderColor := InactiveBorderColor
	if ps.focusedPanel == FileTreePanel {
		borderColor = ActiveBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(ps.treeWidth - 1).
		Height(ps.Height() - 2).
		Padding(0, 1)

	// Заголовок панели
	title := "📁 Files"
	if ps.focusedPanel == FileTreePanel {
		title += " (focused)"
	}

	// Подзаголовок с фильтрами
	subtitle := ps.getFilterInfo()

	// Содержимое
	content := ps.renderFileTree()

	return style.Render(fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, content))
}

// renderStatusPanel отрисовывает панель статуса
func (ps *ProjectScreenReal) renderStatusPanel() string {
	// Стиль рамки
	borderColor := InactiveBorderColor
	if ps.focusedPanel == StatusPanel {
		borderColor = ActiveBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(ps.statusWidth - 1).
		Height(ps.Height() - 2).
		Padding(0, 1)

	// Заголовок панели
	title := "📊 Project Status"
	if ps.focusedPanel == StatusPanel {
		title += " (focused)"
	}

	// Содержимое
	content := ps.renderProjectInfo()

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

// getFilterInfo возвращает информацию о текущих фильтрах
func (ps *ProjectScreenReal) getFilterInfo() string {
	if ps.fileTree == nil {
		return ""
	}

	var filters []string
	if ps.fileTree.ShowHidden {
		filters = append(filters, "Hidden")
	}
	if ps.fileTree.FilterSurge {
		filters = append(filters, ".sg only")
	}

	if len(filters) > 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(DimTextColor)).
			Render("[" + strings.Join(filters, ", ") + "]")
	}

	return ""
}

// renderFileTree отрисовывает дерево файлов
func (ps *ProjectScreenReal) renderFileTree() string {
	if ps.fileTree == nil || len(ps.fileTree.FlatList) == 0 {
		return "No files found"
	}

	var lines []string
	maxLines := ps.Height() - MaxDisplayLines // Оставляем место для заголовка и рамки

	start := ps.fileTree.Selected
	if maxLines > ScrollOffset && start > maxLines/ScrollOffset {
		start = ps.fileTree.Selected - maxLines/ScrollOffset
	}
	if start < 0 {
		start = 0
	}

	end := start + maxLines
	if end > len(ps.fileTree.FlatList) {
		end = len(ps.fileTree.FlatList)
		start = end - maxLines
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		node := ps.fileTree.FlatList[i]
		line := node.GetDisplayName()

		// Обрезаем слишком длинные строки
		maxWidth := ps.treeWidth - 6
		if len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
		}

		// Подсвечиваем выбранную строку
		if i == ps.fileTree.Selected {
			if ps.focusedPanel == FileTreePanel {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#7C3AED")).
					Foreground(lipgloss.Color("#FFFFFF")).
					Render(line)
			} else {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#334155")).
					Render(line)
			}
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderProjectInfo отрисовывает информацию о проекте
func (ps *ProjectScreenReal) renderProjectInfo() string {
	var lines []string

	// Путь к проекту
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Path:"))
	projectPath := ps.projectPath
	if len(projectPath) > ps.statusWidth-10 {
		projectPath = "..." + projectPath[len(projectPath)-(ps.statusWidth-13):]
	}
	lines = append(lines, projectPath)
	lines = append(lines, "")

	// Статистика файлов
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Statistics:"))
	lines = append(lines, fmt.Sprintf("📁 Directories: %d", ps.statusInfo.DirCount))
	lines = append(lines, fmt.Sprintf("📄 Files: %d", ps.statusInfo.FileCount))
	lines = append(lines, "")

	// Информация о выбранном файле
	if selected := ps.fileTree.GetSelected(); selected != nil {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Selected:"))
		lines = append(lines, selected.Name)
		if !selected.IsDir {
			lines = append(lines, fmt.Sprintf("Size: %d bytes", selected.Size))
		}
		lines = append(lines, "")
	}

	// Горячие клавиши
	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Controls:"))
	lines = append(lines, "↑↓ / jk - Navigate")
	lines = append(lines, "Enter - Open file/folder")
	lines = append(lines, "Space - Expand/collapse")
	lines = append(lines, "h - Toggle hidden files")
	lines = append(lines, "s - Toggle .sg filter")
	lines = append(lines, "r - Refresh")
	lines = append(lines, "←→ - Switch panels")

	return strings.Join(lines, "\n")
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
	return "↑↓: Navigate • Enter: Open • Space: Expand • h: Hidden • s: .sg filter • r: Refresh"
}

// FullHelp возвращает полную справку
func (ps *ProjectScreenReal) FullHelp() []string {
	help := ps.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Project Screen:",
		"  ↑/↓ or j/k - Navigate files",
		"  ←/→ - Switch between tree and status panels",
		"  Enter - Open selected file or expand directory",
		"  Space - Expand/collapse directory",
		"  h - Toggle hidden files display",
		"  s - Toggle .sg files only filter",
		"  r or Ctrl+R - Refresh file tree",
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