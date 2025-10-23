package screens

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	diagHeaderColor    = "#38BDF8"
	diagErrorColor     = "#F87171"
	diagWarningColor   = "#FBBF24"
	diagInfoColor      = "#A5B4FC"
	diagSelectedBg     = "#312E81"
	diagSelectedFg     = "#F8FAFC"
	diagSecondaryColor = "#94A3B8"
)

func (ds *DiagnosticsScreen) render() string {
	var sections []string
	sections = append(sections, ds.renderHeaderSection())
	sections = append(sections, ds.renderTableSection())
	sections = append(sections, ds.renderDetailSection())
	return strings.Join(sections, "\n\n")
}

func (ds *DiagnosticsScreen) renderHeaderSection() string {
	width := ds.Width()
	if width <= 0 {
		width = 80
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(diagHeaderColor)).
		Render("Diagnostics")

	projectPath := ds.projectPath
	if projectPath == "" {
		projectPath = "(project not set)"
	} else {
		if w := width - 20; w > 0 {
			projectPath = truncatePath(projectPath, w)
		}
	}
	project := lipgloss.NewStyle().Foreground(lipgloss.Color(diagSecondaryColor)).
		Render("Project: " + projectPath)

	status := ds.status
	if ds.running {
		status = "Running diagnostics…"
	}
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(diagSecondaryColor))
	if ds.err != nil {
		statusStyle = statusStyle.Foreground(lipgloss.Color(diagErrorColor))
	}
	statusLine := statusStyle.Render(status)

	counts := ds.summaryCounts()
	countLine := lipgloss.NewStyle().Foreground(lipgloss.Color(diagSecondaryColor)).
		Render(counts)

	var meta string
	if !ds.lastRun.IsZero() {
		duration := ds.runDuration
		if duration == 0 {
			duration = time.Since(ds.lastRun)
		}
		meta = fmt.Sprintf("Last run: %s • Duration: %s • Exit code: %d",
			ds.lastRun.Format("15:04:05"), duration.Round(10*time.Millisecond), ds.exitCode)
		meta = lipgloss.NewStyle().Foreground(lipgloss.Color(diagSecondaryColor)).Render(meta)
	}

	lines := []string{title, project, statusLine, countLine}
	if meta != "" {
		lines = append(lines, meta)
	}

	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (ds *DiagnosticsScreen) renderTableSection() string {
	width := ds.Width()
	if width <= 0 {
		width = 80
	}
	height := ds.listHeight()
	if height <= 0 {
		height = 5
	}

	if len(ds.diagnostics) == 0 {
		msg := "No diagnostics to display."
		if ds.err != nil {
			msg = fmt.Sprintf("Diagnostics failed: %v", ds.err)
		} else if ds.running {
			msg = "Collecting diagnostics…"
		}
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Foreground(lipgloss.Color(diagSecondaryColor)).
			Render(msg)
	}

	severityWidth := 8
	codeWidth := 12
	locationMinWidth := 24
	available := width - severityWidth - codeWidth - locationMinWidth - 6
	if available < 16 {
		available = 16
	}
	messageWidth := available
	locationWidth := width - severityWidth - codeWidth - messageWidth - 6
	if locationWidth < locationMinWidth {
		locationWidth = locationMinWidth
		messageWidth = width - severityWidth - codeWidth - locationWidth - 6
		if messageWidth < 16 {
			messageWidth = 16
		}
	}

	start := ds.scroll
	end := min(ds.scroll+height, len(ds.diagnostics))

	var rows []string
	for idx := start; idx < end; idx++ {
		entry := ds.diagnostics[idx]
		severity := ds.renderSeverity(entry.Severity)
		code := entry.Code
		if code == "" {
			code = "—"
		}
		message := truncateString(entry.Message, messageWidth)
		location := fmt.Sprintf("%s:%d:%d", entry.File, entry.Line, entry.Column)
		location = truncateString(location, locationWidth)

		row := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
			severityWidth, severity,
			codeWidth, code,
			messageWidth, message,
			location,
		)

		rowStyle := lipgloss.NewStyle().Width(width)
		if idx == ds.selected {
			rowStyle = rowStyle.Background(lipgloss.Color(diagSelectedBg)).Foreground(lipgloss.Color(diagSelectedFg))
		}
		rows = append(rows, rowStyle.Render(row))
	}

	// Ensure height by padding empty rows
	for len(rows) < height {
		rows = append(rows, "")
	}

	return strings.Join(rows, "\n")
}

func (ds *DiagnosticsScreen) renderDetailSection() string {
	width := ds.Width()
	if width <= 0 {
		width = 80
	}

	style := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(diagSecondaryColor)).
		Padding(0, 1)

	if len(ds.diagnostics) == 0 || ds.selected < 0 || ds.selected >= len(ds.diagnostics) {
		return style.Render("Select a diagnostic to see details.")
	}

	entry := ds.diagnostics[ds.selected]

	header := fmt.Sprintf("%s — %s:%d:%d",
		strings.ToUpper(entry.Severity),
		entry.File,
		entry.Line,
		entry.Column,
	)
	if entry.Code != "" {
		header = fmt.Sprintf("%s [%s]", header, entry.Code)
	}

	var builder []string
	builder = append(builder, lipgloss.NewStyle().Bold(true).Render(header))
	builder = append(builder, entry.Message)

	if entry.HasFixes {
		builder = append(builder, lipgloss.NewStyle().
			Foreground(lipgloss.Color(diagInfoColor)).
			Render("🔧 Fixes available (open Fix Mode to apply)."))
	}

	if ds.includeNotes && len(entry.Notes) > 0 {
		builder = append(builder, lipgloss.NewStyle().Bold(true).Render("Notes:"))
		for _, note := range entry.Notes {
			builder = append(builder, "  • "+note)
		}
	}

	builder = append(builder, lipgloss.NewStyle().
		Foreground(lipgloss.Color(diagSecondaryColor)).
		Render("Enter: open in editor • F5: rerun diagnostics • n: toggle notes"))

	return style.Render(strings.Join(builder, "\n"))
}

func (ds *DiagnosticsScreen) renderSeverity(severity string) string {
	switch strings.ToLower(severity) {
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(diagErrorColor)).Render("ERROR")
	case "warning":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(diagWarningColor)).Render("WARN")
	case "note":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(diagInfoColor)).Render("NOTE")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(diagInfoColor)).Render("INFO")
	}
}

func truncateString(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width <= 1 {
		return text[:width]
	}
	runes := []rune(text)
	if len(runes) <= width {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func truncatePath(path string, width int) string {
	if width <= 0 {
		return path
	}
	if lipgloss.Width(path) <= width {
		return path
	}
	if width <= 3 {
		return path[:width]
	}
	base := filepath.Base(path)
	if lipgloss.Width(base)+3 >= width {
		return truncateString(base, width)
	}
	return "…" + truncateString(path[len(path)-width+1:], width-1)
}
