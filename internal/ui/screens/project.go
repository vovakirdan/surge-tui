package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProjectScreen экран обзора проекта
type ProjectScreen struct {
	BaseScreen

	// Состояние экрана
	projectPath   string
	fileTree      *FileTree
	selectedFile  string
	buildStatus   BuildStatus
	lastBuildTime string

	// UI компоненты
	focusedPanel PanelType
}

// PanelType тип панели на экране
type PanelType int

const (
	FileTreePanel PanelType = iota
	StatusPanel
)

// BuildStatus статус последней сборки
type BuildStatus struct {
	InProgress   bool
	Success      bool
	ErrorCount   int
	WarningCount int
	Duration     string
}

// FileTree дерево файлов проекта
type FileTree struct {
	Root      *FileNode
	Selected  int
	Expanded  map[string]bool
	Files     []string // Плоский список для навигации
}

// FileNode узел дерева файлов
type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*FileNode
	Parent   *FileNode
	Level    int
}

// NewProjectScreen создает новый экран проекта
func NewProjectScreen(projectPath string) *ProjectScreen {
	screen := &ProjectScreen{
		BaseScreen:   NewBaseScreen("Project Explorer"),
		projectPath:  projectPath,
		focusedPanel: FileTreePanel,
		fileTree: &FileTree{
			Expanded: make(map[string]bool),
		},
	}

	screen.loadFileTree()
	return screen
}

// Init инициализирует экран (Bubble Tea)
func (ps *ProjectScreen) Init() tea.Cmd {
	return nil
}

// Update обрабатывает сообщения (Bubble Tea)
func (ps *ProjectScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ps.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ps.SetSize(msg.Width, msg.Height-1) // -1 для статус-бара
		return ps, nil
	case FileTreeUpdateMsg:
		ps.updateFileTree(msg)
		return ps, nil
	case BuildStatusMsg:
		ps.updateBuildStatus(msg)
		return ps, nil
	}

	return ps, nil
}

// View отрисовывает экран (Bubble Tea)
func (ps *ProjectScreen) View() string {
	if ps.Width() == 0 {
		return "Loading project..."
	}

	// Разделяем экран на две части
	leftWidth := ps.Width() / 2
	rightWidth := ps.Width() - leftWidth

	// Левая панель - дерево файлов
	leftPanel := ps.renderFileTree(leftWidth, ps.Height())

	// Правая панель - статус проекта
	rightPanel := ps.renderProjectStatus(rightWidth, ps.Height())

	// Объединяем панели
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

// OnEnter вызывается при входе на экран
func (ps *ProjectScreen) OnEnter() tea.Cmd {
	// Обновляем дерево файлов
	return ps.refreshFileTree()
}

// handleKeyPress обрабатывает нажатия клавиш
func (ps *ProjectScreen) handleKeyPress(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "left", "right":
		ps.switchPanel()
		return ps, nil
	case "up", "k":
		return ps, ps.navigateUp()
	case "down", "j":
		return ps, ps.navigateDown()
	case "enter":
		return ps, ps.selectItem()
	case "space":
		return ps, ps.toggleExpand()
	case "ctrl+r":
		return ps, ps.refreshFileTree()
	case "ctrl+b":
		return ps, ps.startBuild()
	}

	return ps, nil
}

// switchPanel переключает фокус между панелями
func (ps *ProjectScreen) switchPanel() {
	if ps.focusedPanel == FileTreePanel {
		ps.focusedPanel = StatusPanel
	} else {
		ps.focusedPanel = FileTreePanel
	}
}

// navigateUp перемещает курсор вверх в дереве файлов
func (ps *ProjectScreen) navigateUp() tea.Cmd {
	if ps.focusedPanel == FileTreePanel && ps.fileTree.Selected > 0 {
		ps.fileTree.Selected--
	}
	return nil
}

// navigateDown перемещает курсор вниз в дереве файлов
func (ps *ProjectScreen) navigateDown() tea.Cmd {
	if ps.focusedPanel == FileTreePanel && ps.fileTree.Selected < len(ps.fileTree.Files)-1 {
		ps.fileTree.Selected++
	}
	return nil
}

// selectItem выбирает текущий элемент
func (ps *ProjectScreen) selectItem() tea.Cmd {
	if ps.focusedPanel == FileTreePanel && len(ps.fileTree.Files) > 0 {
		selectedPath := ps.fileTree.Files[ps.fileTree.Selected]
		ps.selectedFile = selectedPath

		// Если это файл, открываем его в редакторе
		if !ps.isDirectory(selectedPath) {
			return func() tea.Msg {
				return OpenFileMsg{FilePath: selectedPath}
			}
		}
	}
	return nil
}

// toggleExpand разворачивает/сворачивает директорию
func (ps *ProjectScreen) toggleExpand() tea.Cmd {
	if ps.focusedPanel == FileTreePanel && len(ps.fileTree.Files) > 0 {
		selectedPath := ps.fileTree.Files[ps.fileTree.Selected]
		if ps.isDirectory(selectedPath) {
			ps.fileTree.Expanded[selectedPath] = !ps.fileTree.Expanded[selectedPath]
			ps.rebuildFileList()
		}
	}
	return nil
}

// refreshFileTree обновляет дерево файлов
func (ps *ProjectScreen) refreshFileTree() tea.Cmd {
	return func() tea.Msg {
		// TODO: сканировать файловую систему в фоне
		return FileTreeUpdateMsg{}
	}
}

// startBuild запускает сборку проекта
func (ps *ProjectScreen) startBuild() tea.Cmd {
	return func() tea.Msg {
		// TODO: запустить сборку в фоне
		return BuildStartMsg{ProjectPath: ps.projectPath}
	}
}

// renderFileTree отрисовывает дерево файлов
func (ps *ProjectScreen) renderFileTree(width, height int) string {
	title := "📁 Files"
	if ps.focusedPanel == FileTreePanel {
		title = "📁 Files (focused)"
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(width-2).
		Height(height-2).
		Padding(1)

	if ps.focusedPanel == FileTreePanel {
		style = style.BorderForeground(lipgloss.Color("#7C3AED"))
	}

	content := ps.buildFileTreeContent(width-4, height-4)

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

// renderProjectStatus отрисовывает статус проекта
func (ps *ProjectScreen) renderProjectStatus(width, height int) string {
	title := "📊 Project Status"
	if ps.focusedPanel == StatusPanel {
		title = "📊 Project Status (focused)"
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(width-2).
		Height(height-2).
		Padding(1)

	if ps.focusedPanel == StatusPanel {
		style = style.BorderForeground(lipgloss.Color("#7C3AED"))
	}

	content := ps.buildStatusContent()

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

// buildFileTreeContent строит содержимое дерева файлов
func (ps *ProjectScreen) buildFileTreeContent(width, height int) string {
	if len(ps.fileTree.Files) == 0 {
		return "No files found"
	}

	var lines []string
	for i, filePath := range ps.fileTree.Files {
		prefix := "  "
		if i == ps.fileTree.Selected && ps.focusedPanel == FileTreePanel {
			prefix = "▶ "
		}

		fileName := filepath.Base(filePath)
		if ps.isDirectory(filePath) {
			fileName = "📁 " + fileName
		} else {
			fileName = "📄 " + fileName
		}

		line := prefix + fileName
		if len(line) > width {
			line = line[:width-3] + "..."
		}

		lines = append(lines, line)

		if len(lines) >= height {
			break
		}
	}

	return strings.Join(lines, "\n")
}

// buildStatusContent строит содержимое статуса проекта
func (ps *ProjectScreen) buildStatusContent() string {
	var lines []string

	// Информация о проекте
	lines = append(lines, fmt.Sprintf("Path: %s", ps.projectPath))
	lines = append(lines, "")

	// Статус сборки
	lines = append(lines, "Last Build:")
	if ps.buildStatus.InProgress {
		lines = append(lines, "  🔄 Building...")
	} else if ps.buildStatus.Success {
		lines = append(lines, "  ✅ Success")
	} else {
		lines = append(lines, "  ❌ Failed")
	}

	if ps.buildStatus.ErrorCount > 0 {
		lines = append(lines, fmt.Sprintf("  🔴 %d errors", ps.buildStatus.ErrorCount))
	}

	if ps.buildStatus.WarningCount > 0 {
		lines = append(lines, fmt.Sprintf("  🟡 %d warnings", ps.buildStatus.WarningCount))
	}

	if ps.buildStatus.Duration != "" {
		lines = append(lines, fmt.Sprintf("  ⏱️  %s", ps.buildStatus.Duration))
	}

	lines = append(lines, "")

	// Быстрые действия
	lines = append(lines, "Quick Actions:")
	lines = append(lines, "  Enter - Open file")
	lines = append(lines, "  Space - Expand/collapse")
	lines = append(lines, "  Ctrl+B - Build")
	lines = append(lines, "  Ctrl+R - Refresh")

	return strings.Join(lines, "\n")
}

// Вспомогательные методы

func (ps *ProjectScreen) loadFileTree() {
	// TODO: загрузить дерево файлов из файловой системы
}

func (ps *ProjectScreen) updateFileTree(msg FileTreeUpdateMsg) {
	// TODO: обновить дерево файлов
}

func (ps *ProjectScreen) updateBuildStatus(msg BuildStatusMsg) {
	ps.buildStatus = BuildStatus{
		InProgress:   msg.InProgress,
		Success:      msg.Success,
		ErrorCount:   msg.ErrorCount,
		WarningCount: msg.WarningCount,
		Duration:     msg.Duration,
	}
}

func (ps *ProjectScreen) rebuildFileList() {
	// TODO: пересобрать плоский список файлов из дерева
}

func (ps *ProjectScreen) isDirectory(path string) bool {
	// TODO: проверить, является ли путь директорией
	return false
}

// ShortHelp возвращает краткую справку
func (ps *ProjectScreen) ShortHelp() string {
	return "↑↓: Navigate • Enter: Open • Space: Expand • Ctrl+B: Build"
}

// FullHelp возвращает полную справку
func (ps *ProjectScreen) FullHelp() []string {
	help := ps.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Project Screen:",
		"  ↑/↓ or j/k - Navigate files",
		"  ←/→ - Switch panels",
		"  Enter - Open selected file",
		"  Space - Expand/collapse directory",
		"  Ctrl+B - Build project",
		"  Ctrl+R - Refresh file tree",
	}...)
	return help
}

// Сообщения для экрана проекта

// FileTreeUpdateMsg обновление дерева файлов
type FileTreeUpdateMsg struct{}

// BuildStatusMsg статус сборки
type BuildStatusMsg struct {
	InProgress   bool
	Success      bool
	ErrorCount   int
	WarningCount int
	Duration     string
}

// BuildStartMsg запуск сборки
type BuildStartMsg struct {
	ProjectPath string
}

// OpenFileMsg открытие файла
type OpenFileMsg struct {
	FilePath string
}