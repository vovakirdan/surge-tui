package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (ss *SettingsScreen) renderMenu() string {
	fields := allSettingsFields()
	width := ss.state.menuWidth
	if width <= 0 {
		width = settingsMenuWidth
	}

	var lines []string
	for _, field := range fields {
		name := ss.fieldName(field)
		style := lipgloss.NewStyle().Width(width).Padding(0, 1)
		if field == ss.state.selectedField {
			style = style.Foreground(lipgloss.Color(selectedColor)).Bold(true)
		} else {
			style = style.Foreground(lipgloss.Color(unselectedColor))
		}
		if ss.isFieldChanged(field) {
			name = fmt.Sprintf("* %s", name)
		}
		lines = append(lines, style.Render(name))
	}

	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(width + 2)
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(selectedColor)).Bold(true).
		Render(" Settings ")

	body := strings.Join(lines, "\n")
	return border.Render(fmt.Sprintf("%s\n%s", title, body))
}

func (ss *SettingsScreen) renderContent() string {
	width := ss.state.contentWidth
	if width <= 0 {
		width = ss.Width() - ss.state.menuWidth
	}

	currentValue := ss.getCurrentValue()
	description := ss.fieldDescription()

	content := &strings.Builder{}

	title := lipgloss.NewStyle().Foreground(lipgloss.Color(selectedColor)).Bold(true).
		Render(ss.fieldName(ss.state.selectedField))
	content.WriteString(title)
	content.WriteString("\n\n")

	valueColor := unselectedColor
	if !ss.state.editMode && ss.isFieldChanged(ss.state.selectedField) {
		valueColor = modifiedColor
	}

	if ss.state.editMode {
		content.WriteString(ss.input.View())
	} else {
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(valueColor))
		content.WriteString(valueStyle.Render(currentValue))
	}

	content.WriteString("\n\n")
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor))
	content.WriteString(descStyle.Render(description))

	if result, ok := ss.state.validation[ss.state.selectedField]; ok && result.Message != "" {
		content.WriteString("\n\n")
		indicator := lipgloss.NewStyle()
		if result.Valid {
			indicator = indicator.Foreground(lipgloss.Color(validColor))
		} else {
			indicator = indicator.Foreground(lipgloss.Color(invalidColor)).Bold(true)
		}
		content.WriteString(indicator.Render(result.Message))
	}

	if ss.state.selectedField == SurgeBinaryField && !ss.state.surgeCheck.IsZero() {
		content.WriteString("\n\n")
		stamp := lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor)).
			Render(fmt.Sprintf("Last checked: %s", ss.state.surgeCheck.Format("15:04:05")))
		content.WriteString(stamp)
	}

	content.WriteString("\n\n")
	hint := "Enter: Edit • Space: Edit"
	if ss.state.editMode {
		hint = "Enter: Save • Esc: Cancel"
	}
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor))
	content.WriteString(hintStyle.Render(hint))

	inner := lipgloss.NewStyle().Padding(1).Width(width).
		Render(content.String())
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(width + 2).Render(inner)
}
