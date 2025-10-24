package screens

import (
	"fmt"
	"path/filepath"
	"strings"
	"os"
	"surge-tui/internal/core/surge"

	"github.com/charmbracelet/lipgloss"
)

func (fs *FixModeScreen) renderLoading() string {
	return lipgloss.NewStyle().
		Width(fs.Width()).
		Height(fs.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Render("🔄 Loading fixes...")
}

func (fs *FixModeScreen) renderError() string {
	return lipgloss.NewStyle().
		Width(fs.Width()).
		Height(fs.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color(diagErrorColor)).
		Render(fmt.Sprintf("❌ Failed to load fixes\n\n%v", fs.err))
}

func (fs *FixModeScreen) renderEmpty() string {
	message := "No fixes available. Run diagnostics with suggestions to populate this list."
	return lipgloss.NewStyle().
		Width(fs.Width()).
		Height(fs.Height()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(message)
}

func (fs *FixModeScreen) renderContent() string {
	listWidth := fs.Width() / 2
	if listWidth < 32 {
		listWidth = 32
	}
	detailWidth := fs.Width() - listWidth - 1
	if detailWidth < 32 {
		detailWidth = 32
	}

	list := fs.renderList(listWidth)
	detail := fs.renderDetail(detailWidth)

	base := lipgloss.JoinHorizontal(lipgloss.Top, list, detail)

	if status := fs.statusLine(); status != "" {
		statusBar := lipgloss.NewStyle().
			Width(fs.Width()).
			Foreground(lipgloss.Color(diagSecondaryColor)).
			Render(status)
		base = lipgloss.JoinVertical(lipgloss.Left, base, statusBar)
	}

	return base
}

func (fs *FixModeScreen) renderList(width int) string {
	height := fs.listHeight()
	if height <= 0 {
		height = 3
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(diagSecondaryColor)).
		Width(width).
		Height(height+2).
		Padding(0, 1)

	var rows []string
	start := fs.scroll
	end := fs.scroll + height
	if end > len(fs.entries) {
		end = len(fs.entries)
	}

	for i := start; i < end; i++ {
		entry := fs.entries[i]
		title := entry.Fix.Title
		if title == "" {
			title = "(unnamed fix)"
		}
		line := fmt.Sprintf("%s — %s", truncateText(entry.FilePath, width-4), title)
		if i == fs.selected {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color(diagSelectedBg)).
				Foreground(lipgloss.Color(diagSelectedFg)).
				Render(line)
		}
		rows = append(rows, line)
	}

	return style.Render(strings.Join(rows, "\n"))
}

func (fs *FixModeScreen) renderDetail(width int) string {
	if len(fs.entries) == 0 {
		return ""
	}
	entry := fs.entries[fs.selected]

	header := fmt.Sprintf("%s\n%s", entry.Fix.Title, entry.Diagnostic.Message)
	meta := fmt.Sprintf("File: %s\nSeverity: %s\nCode: %s", entry.FilePath, strings.ToUpper(entry.Diagnostic.Severity), entry.Diagnostic.Code)
	preview := fs.getPreview(entry)
	diffBlock := fs.renderDiff(preview)

	body := strings.Join([]string{header, "", meta, "", "Diff:", diffBlock}, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(diagSecondaryColor)).
		Width(width).
		Height(fs.listHeight()+2).
		Padding(0, 1)

	return style.Render(body)
}

// Utility -----------------------------------------------------------------

func truncateText(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width <= 3 {
		return text[:width]
	}
	runes := []rune(text)
	if len(runes) <= width {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func (fs *FixModeScreen) previewKey(entry fixEntry) string {
	id := entry.Fix.ID
	if id == "" {
		id = fmt.Sprintf("%s:%d", entry.Diagnostic.Code, entry.Diagnostic.Location.StartLine)
	}
	return filepath.Clean(entry.FilePath) + "::" + id
}

func (fs *FixModeScreen) getPreview(entry fixEntry) *diffPreview {
	if fs.previewCache == nil {
		fs.previewCache = make(map[string]*diffPreview)
	}
	key := fs.previewKey(entry)
	if cached, ok := fs.previewCache[key]; ok {
		return cached
	}
	preview := fs.buildPreview(entry)
	fs.previewCache[key] = preview
	return preview
}

func (fs *FixModeScreen) buildPreview(entry fixEntry) *diffPreview {
	if len(entry.Fix.Edits) == 0 {
		return &diffPreview{Diff: "(no edits provided)"}
	}

	var sb strings.Builder
	var fileLines []string
	var fileErr error
	fileLoaded := false

	loadFile := func() {
		if fileLoaded {
			return
		}
		data, err := os.ReadFile(entry.FilePath)
		if err != nil {
			fileErr = err
		} else {
			fileLines = strings.Split(string(data), "\n")
		}
		fileLoaded = true
	}

	for idx, edit := range entry.Fix.Edits {
		loc := edit.Location
		sb.WriteString(fmt.Sprintf("@@ %d:%d-%d:%d @@\n", loc.StartLine, loc.StartCol, loc.EndLine, loc.EndCol))

		oldText := edit.OldText
		if oldText == "" {
			loadFile()
			if fileErr == nil {
				oldText = extractSegment(fileLines, loc)
			}
		}
		if oldText == "" {
			oldText = "(no original text)"
		}
		for _, line := range strings.Split(oldText, "\n") {
			sb.WriteString("- " + line + "\n")
		}

		newText := edit.NewText
		if newText == "" {
			newText = "(delete)"
		}
		for _, line := range strings.Split(newText, "\n") {
			sb.WriteString("+ " + line + "\n")
		}

		if idx < len(entry.Fix.Edits)-1 {
			sb.WriteString("\n")
		}
	}

	diff := strings.TrimSpace(sb.String())
	if diff == "" {
		diff = "(no diff)"
	}
	return &diffPreview{Diff: diff, Err: fileErr}
}

func extractSegment(lines []string, loc surge.LocationJSON) string {
	if len(lines) == 0 {
		return ""
	}

	startLine := int(loc.StartLine)
	endLine := int(loc.EndLine)
	if startLine == 0 {
		startLine = endLine
	}
	if startLine == 0 {
		startLine = 1
	}
	if endLine == 0 {
		endLine = startLine
	}
	if startLine > len(lines) {
		return ""
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	startIdx := startLine - 1
	endIdx := endLine - 1
	if endIdx < startIdx {
		endIdx = startIdx
	}

	startCol := int(loc.StartCol)
	if startCol <= 0 {
		startCol = 1
	}
	endCol := int(loc.EndCol)
	if endCol <= 0 {
		endCol = len([]rune(lines[endIdx])) + 1
	}

	var parts []string
	for i := startIdx; i <= endIdx; i++ {
		lineRunes := []rune(lines[i])
		lineLen := len(lineRunes)
		var segment string

		switch {
		case startIdx == endIdx:
			sc := clamp(startCol-1, 0, lineLen)
			ec := clamp(endCol-1, sc, lineLen)
			segment = string(lineRunes[sc:ec])
		case i == startIdx:
			sc := clamp(startCol-1, 0, lineLen)
			segment = string(lineRunes[sc:])
		case i == endIdx:
			ec := clamp(endCol-1, 0, lineLen)
			segment = string(lineRunes[:ec])
		default:
			segment = string(lineRunes)
		}
		parts = append(parts, segment)
	}

	return strings.Join(parts, "\n")
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (fs *FixModeScreen) renderDiff(preview *diffPreview) string {
	if preview == nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(diagSecondaryColor)).Render("(no diff)")
	}
	diff := preview.Diff
	if diff == "" {
		diff = "(no diff)"
	}
	lines := strings.Split(diff, "\n")
	styled := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		var style lipgloss.Style
		switch {
		case strings.HasPrefix(line, "+"):
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(fixDiffAddColor))
		case strings.HasPrefix(line, "-"):
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(fixDiffDelColor))
		case strings.HasPrefix(line, "@@"):
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(fixDiffMetaColor)).Bold(true)
		default:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color(diagSecondaryColor))
		}
		styled = append(styled, style.Render(line))
	}
	if preview.Err != nil {
		warning := lipgloss.NewStyle().Foreground(lipgloss.Color(fixDiffWarnColor)).Render("⚠ " + preview.Err.Error())
		styled = append(styled, warning)
	}
	return strings.Join(styled, "\n")
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	aClean := filepath.Clean(a)
	bClean := filepath.Clean(b)
	return strings.EqualFold(aClean, bClean)
}
