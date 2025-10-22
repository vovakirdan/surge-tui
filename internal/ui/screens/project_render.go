package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	return style.Render(fmt.Sprintf("❌ Error loading project\n\n%s\n\n%v\n\nPress 'Ctrl+R' to retry", ps.projectPath, ps.err))
}

func (ps *ProjectScreenReal) renderFileTreePanel() string {
	borderColor := InactiveBorderColor
	if ps.focusedPanel == FileTreePanel {
		borderColor = ActiveBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(ps.treeWidth-1).
		Height(ps.Height()-2).
		Padding(0, 1)

	title := "📁 Files"
	if ps.focusedPanel == FileTreePanel {
		title += " (focused)"
	}

	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color(DimTextColor)).Render(ps.getFilterInfo())
	content := ps.renderFileTree()
	if status := ps.statusLine(); status != "" {
		content = fmt.Sprintf("%s\n\n%s", content, lipgloss.NewStyle().Foreground(lipgloss.Color(DimTextColor)).Render(status))
	}

	return style.Render(fmt.Sprintf("%s\n%s\n\n%s", title, subtitle, content))
}

func (ps *ProjectScreenReal) renderStatusPanel() string {
	borderColor := InactiveBorderColor
	if ps.focusedPanel == StatusPanel {
		borderColor = ActiveBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Width(ps.statusWidth-1).
		Height(ps.Height()-2).
		Padding(0, 1)

	title := "📊 Project Status"
	if ps.focusedPanel == StatusPanel {
		title += " (focused)"
	}

	content := ps.renderProjectInfo()
	return style.Render(fmt.Sprintf("%s\n\n%s", title, content))
}

func (ps *ProjectScreenReal) renderFileTree() string {
	if ps.fileTree == nil || len(ps.fileTree.FlatList) == 0 {
		return "No files found"
	}

	var lines []string
	maxLines := max(ps.Height() - MaxDisplayLines, 1)

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
		start = max(end - maxLines, 0)
	}

	for i := start; i < end; i++ {
		node := ps.fileTree.FlatList[i]
		line := node.GetDisplayName()

		maxWidth := max(ps.treeWidth - 6, 1)
		if len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
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

func (ps *ProjectScreenReal) renderProjectInfo() string {
	var lines []string

	lines = append(lines, lipgloss.NewStyle().Bold(true).Render("Path:"))
	projectPath := ps.projectPath
	if len(projectPath) > ps.statusWidth-10 {
		projectPath = "..." + projectPath[len(projectPath)-(ps.statusWidth-13):]
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
	lines = append(lines, "↑↓ / jk - Navigate")
	lines = append(lines, "Enter - Open file / expand directory")
	lines = append(lines, "Space - Expand/collapse")
	lines = append(lines, "n - New file")
	lines = append(lines, "Shift+N - New directory")
	lines = append(lines, "r - Rename")
	lines = append(lines, "Delete - Remove")
	lines = append(lines, "h - Toggle hidden files")
	lines = append(lines, "s - Toggle .sg filter")
	lines = append(lines, "Ctrl+R - Refresh")
	lines = append(lines, "←→ - Switch panels")

	return strings.Join(lines, "\n")
}
