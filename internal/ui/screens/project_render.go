package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/platform"
)

func (ps *ProjectScreenReal) renderLoading() string {
	style := lipgloss.NewStyle().
		Width(ps.Width()).
		Height(ps.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(LoadingColor))

	return style.Render("🔄 Loading project...\n\n" + ps.projectPath)
}

func (ps *ProjectScreenReal) renderError() string {
	style := lipgloss.NewStyle().
		Width(ps.Width()).
		Height(ps.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(ErrorColor))

	message := fmt.Sprintf("❌ Error loading project\n\n%s\n\n%v\n\nPress 'Ctrl+R' to retry", ps.projectPath, ps.err)
	return style.Render(platform.ReplacePrimaryModifier(message))
}

func (ps *ProjectScreenReal) renderFileTreePanel() string {
	width := ps.treeWidth
	if width <= 0 {
		width = ps.Width()
	}
	height := max(ps.Height()-2, 3)

	borderColor := InactiveBorderColor
	if ps.focusedPanel == FileTreePanel {
		borderColor = ActiveBorderColor
	}

	title := "📁 Files"
	if ps.focusedPanel == FileTreePanel {
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render(title + " (focused)")
	} else {
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E2E8F0")).Render(title)
	}

	filterInfo := lipgloss.NewStyle().Foreground(lipgloss.Color(DimTextColor)).Render(ps.getFilterInfo())
	treeContent := ps.renderFileTree(width)

	var builder []string
	builder = append(builder, title, filterInfo, "", treeContent)

	if status := ps.statusLine(); status != "" && (ps.focusedPanel == FileTreePanel || len(ps.tabs) == 0) {
		builder = append(builder, "", lipgloss.NewStyle().Foreground(lipgloss.Color(DimTextColor)).Render(status))
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(width).
		Height(height).
		Padding(0, 1)

	return style.Render(strings.Join(builder, "\n"))
}

func (ps *ProjectScreenReal) renderWorkspacePanel() string {
	if ps.mainWidth <= 0 {
		return ""
	}
	if len(ps.tabs) == 0 {
		return ps.renderWorkspaceEmpty()
	}
	return ps.renderWorkspaceEditor()
}

func (ps *ProjectScreenReal) renderWorkspaceEmpty() string {
	width := ps.mainWidth
	height := max(ps.Height()-2, 3)

	borderColor := InactiveBorderColor
	if ps.focusedPanel == EditorPanel {
		borderColor = ActiveBorderColor
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("🛠 Workspace")
	content := ps.renderProjectInfo(width)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(width).
		Height(height).
		Padding(0, 1)

	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

func (ps *ProjectScreenReal) renderWorkspaceEditor() string {
	width := max(ps.mainWidth, 20)
	height := max(ps.Height()-2, 3)

	borderColor := InactiveBorderColor
	if ps.focusedPanel == EditorPanel {
		borderColor = ActiveBorderColor
	}

	innerWidth := max(width-2, 10)
	tabBar := ps.renderTabBar(innerWidth)
	body := ps.renderEditorBody()
	status := ps.renderEditorStatus()

	var parts []string
	if tabBar != "" {
		parts = append(parts, tabBar)
	}
	parts = append(parts, body, status)

	if tab := ps.activeEditorTab(); tab != nil && tab.mode == editorModeCommand {
		parts = append(parts, ps.renderCommandLine())
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(width).
		Height(height).
		Padding(0, 1)

	return style.Render(strings.Join(parts, "\n"))
}

func (ps *ProjectScreenReal) renderTabBar(width int) string {
	if len(ps.tabs) == 0 {
		return ""
	}
	if width < 10 {
		width = 10
	}

	var rendered []string
	for i, tab := range ps.tabs {
		title := tab.name
		if tab.dirty {
			title = "*" + title
		}
		if lipgloss.Width(title) > width/2 {
			title = truncateMiddle(title, max(width/2, 8))
		}
		if i == ps.activeTab {
			rendered = append(rendered, ps.tabActiveStyle.Render(title))
		} else {
			rendered = append(rendered, ps.tabNormalStyle.Render(title))
		}
	}

	line := strings.Join(rendered, " ")
	return lipgloss.NewStyle().Width(width).Render(line)
}

func (ps *ProjectScreenReal) renderEditorBody() string {
	tab := ps.activeEditorTab()
	if tab == nil {
		return ""
	}

	contentWidth := ps.editorContentWidth()
	contentHeight := ps.editorContentHeight()

	lineNumberStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#7C3AED"))

	start := tab.scroll
	end := min(start+contentHeight, tab.lineCount())

	var rows []string
	for idx := start; idx < end; idx++ {
		line := tab.lines[idx]
		runes := []rune(line)
		col := min(tab.cursor.Col, len(runes))

		display := line
		if idx == tab.cursor.Line {
			before := string(runes[:col])
			cursor := " "
			after := ""
			if col < len(runes) {
				cursor = string(runes[col])
				after = string(runes[col+1:])
			}
			display = before + cursorStyle.Render(cursor) + after
		}

		contentStyle := lipgloss.NewStyle().Width(contentWidth)
		if idx == tab.cursor.Line {
			contentStyle = contentStyle.Background(lipgloss.Color("#1F2937"))
		}

		number := lineNumberStyle.Render(fmt.Sprintf("%5d ", idx+1))
		row := lipgloss.JoinHorizontal(lipgloss.Left, number, contentStyle.Render(display))
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left,
			lineNumberStyle.Render(fmt.Sprintf("%5d ", 1)),
			lipgloss.NewStyle().Width(contentWidth).Render(""),
		))
	}

	body := strings.Join(rows, "\n")
	bodyStyle := lipgloss.NewStyle().
		Width(contentWidth + 6).
		Height(contentHeight)

	return bodyStyle.Render(body)
}

func (ps *ProjectScreenReal) renderEditorStatus() string {
	tab := ps.activeEditorTab()
	if tab == nil {
		return ""
	}

	mode := ""
	switch tab.mode {
	case editorModeInsert:
		mode = "-- INSERT --"
	case editorModeCommand:
		mode = "-- COMMAND --"
	default:
		mode = "-- NORMAL --"
	}

	dirty := ""
	if tab.dirty {
		dirty = "*"
	}

	position := fmt.Sprintf("L%d C%d", tab.cursor.Line+1, tab.cursor.Col+1)
	info := fmt.Sprintf("%s %s %s | %s", mode, dirty, tab.name, position)

	if status := ps.statusLine(); status != "" && ps.focusedPanel == EditorPanel {
		info += "  —  " + status
	}

	return lipgloss.NewStyle().
		Width(max(ps.mainWidth-2, 20)).
		Background(lipgloss.Color("#1E293B")).
		Foreground(lipgloss.Color("#CBD5F5")).
		Padding(0, 1).
		Render(strings.TrimSpace(info))
}

func (ps *ProjectScreenReal) renderCommandLine() string {
	return lipgloss.NewStyle().
		Width(max(ps.mainWidth-2, 20)).
		Background(lipgloss.Color("#111827")).
		Foreground(lipgloss.Color("#F8FAFC")).
		Padding(0, 1).
		Render(ps.editorCommand.View())
}

func (ps *ProjectScreenReal) renderFileTree(panelWidth int) string {
	if ps.fileTree == nil || len(ps.fileTree.FlatList) == 0 {
		return "No files found"
	}

	if panelWidth <= 0 {
		panelWidth = max(ps.treeWidth, 20)
	}

	var lines []string
	maxLines := max(ps.Height()-MaxDisplayLines, 1)

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
		start = max(end-maxLines, 0)
	}

	for i := start; i < end; i++ {
		node := ps.fileTree.FlatList[i]
		line := node.GetDisplayName()

		maxWidth := max(panelWidth-6, 1)
		if len(line) > maxWidth {
			runes := []rune(line)
			if len(runes) > maxWidth {
				line = string(runes[:maxWidth-3]) + "..."
			} else {
				line = line[:maxWidth]
			}
		}

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

func (ps *ProjectScreenReal) getFilterInfo() string {
	if ps.fileTree == nil {
		return "Filters: none"
	}
	var filters []string
	if ps.fileTree.ShowHidden {
		filters = append(filters, "Hidden")
	}
	if ps.fileTree.FilterSurge {
		filters = append(filters, ".sg only")
	}
	if len(filters) == 0 {
		return "Filters: none"
	}
	return "Filters: " + strings.Join(filters, ", ")
}

func (ps *ProjectScreenReal) renderProjectInfo(width int) string {
	var lines []string

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Path:"))
	projectPath := ps.projectPath
	if lipgloss.Width(projectPath) > width-10 {
		projectPath = truncateMiddle(projectPath, max(width-10, 16))
	}
	lines = append(lines, projectPath)
	lines = append(lines, "")

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Statistics:"))
	lines = append(lines, fmt.Sprintf("📁 Directories: %d", ps.statusInfo.DirCount))
	lines = append(lines, fmt.Sprintf("📄 Files: %d", ps.statusInfo.FileCount))
	lines = append(lines, "")

	if selected := ps.fileTree.GetSelected(); selected != nil {
		lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Selected:"))
		lines = append(lines, selected.Name)
		if !selected.IsDir {
			lines = append(lines, fmt.Sprintf("Size: %d bytes", selected.Size))
		}
		lines = append(lines, "")
	}

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Controls:"))
	lines = append(lines, "↑↓ / j k - Navigate tree")
	lines = append(lines, "Enter - Open file tab / expand dir")
	lines = append(lines, "Space - Expand/collapse directory")
	lines = append(lines, "n / Shift+N - New file / directory")
	lines = append(lines, "r - Rename • Delete - Remove")
	lines = append(lines, "h - Toggle hidden • s - Toggle .sg")
	lines = append(lines, platform.ReplacePrimaryModifier("Ctrl+R - Refresh tree listing"))
	lines = append(lines, platform.ReplacePrimaryModifier("Ctrl+→ focus editor • Ctrl+← focus tree"))
	lines = append(lines, "Alt+←/→ switch tab • Alt+Shift+←/→ reorder")
	lines = append(lines, ":w save • :q quit tab • yy/dd/p line copy/cut/paste")

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Actions:"))
	lines = append(lines, ps.renderFileActions())

	return strings.Join(lines, "\n")
}

func (ps *ProjectScreenReal) renderFileActions() string {
	if ps.fileTree == nil {
		return "–"
	}
	node := ps.fileTree.GetSelected()
	if node == nil {
		return "–"
	}

	var entries []string
	button := func(label, hint string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Render(fmt.Sprintf("[ %s ]", strings.ToUpper(label))) + " " + hint
	}
	buttonDisabled := func(label, hint string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#475569")).Render(fmt.Sprintf("[ %s ]", strings.ToUpper(label))) + " " + hint
	}

	if node.IsDir {
		if ps.isProjectDirectory(node.Path) {
			entries = append(entries, button("format", "Format project (TODO)"))
			entries = append(entries, button("build", "Build project (TODO)"))
			entries = append(entries, button("diagnostic", "Run diagnostics (TODO)"))
		} else {
			entries = append(entries, button("init", "Initialize project"))
			entries = append(entries, buttonDisabled("format", "Project not initialized"))
			entries = append(entries, buttonDisabled("build", "Project not initialized"))
			entries = append(entries, buttonDisabled("diagnostic", "Project not initialized"))
		}
	} else {
		entries = append(entries, button("open", "Open in editor (Enter)"))
	}

	return strings.Join(entries, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncateMiddle(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= limit {
		return s
	}
	if limit <= 3 {
		return s[:limit]
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return string(runes[:limit])
	}
	left := (limit - 3) / 2
	right := limit - 3 - left
	return string(runes[:left]) + "..." + string(runes[len(runes)-right:])
}
