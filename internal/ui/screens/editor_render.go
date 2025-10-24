package screens

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/platform"
)

func (es *EditorScreen) renderLoading() string {
	return lipgloss.NewStyle().
		Width(es.Width()).
		Height(es.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(LoadingColor)).
		Render("🔄 Loading file...\n\n" + es.filePath)
}

func (es *EditorScreen) renderError() string {
	return lipgloss.NewStyle().
		Width(es.Width()).
		Height(es.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(ErrorColor)).
		Render(platform.ReplacePrimaryModifier(fmt.Sprintf("❌ Failed to load file\n\n%s\n\n%v\n\nCtrl+R to retry", es.filePath, es.err)))
}

func (es *EditorScreen) renderEmpty() string {
	message := "No file selected\n\nOpen a file from the project tree or via the command palette."
	return lipgloss.NewStyle().
		Width(es.Width()).
		Height(es.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(message)
}

func (es *EditorScreen) renderContent() string {
	header := es.renderHeader()
	body := es.renderBody()
	footer := es.renderFooter()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (es *EditorScreen) renderHeader() string {
	if es.filePath == "" {
		return ""
	}
	name := filepath.Base(es.filePath)
	info := fmt.Sprintf("%s — %d lines • %.1f KB • %s", name, es.stats.lineCount, float64(es.stats.size)/1024.0, es.stats.modTime.Format(time.RFC822))
	return lipgloss.NewStyle().
		Width(es.Width()).
		Bold(true).
		Background(lipgloss.Color("#1E293B")).
		Foreground(lipgloss.Color("#F8FAFC")).
		Padding(0, 1).
		Render(info)
}

func (es *EditorScreen) renderBody() string {
	height := max(es.contentHeight(), 1)
	var lines []string
	start := es.scroll
	end := min(start+height, len(es.lines))
	for idx := start; idx < end; idx++ {
		lineNumber := fmt.Sprintf("%6d ", idx+1)
		content := es.lines[idx]
		if !es.softWrap {
			maxWidth := es.Width() - 8
			if maxWidth > 0 && len([]rune(content)) > maxWidth {
				content = string([]rune(content)[:maxWidth]) + "…"
			}
		}
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(lineNumber)+content)
	}
	return lipgloss.NewStyle().
		Width(es.Width()).
		Height(height).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

func (es *EditorScreen) renderFooter() string {
	status := es.statusLine()
	if status == "" {
		status = fmt.Sprintf("%s | %d/%d lines", es.filePath, es.scroll+1, es.stats.lineCount)
	}
	return lipgloss.NewStyle().
		Width(es.Width()).
		Background(lipgloss.Color("#1E293B")).
		Foreground(lipgloss.Color("#CBD5F5")).
		Padding(0, 1).
		Render(status)
}
