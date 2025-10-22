package screens

import (
	"context"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (ss *SettingsScreen) handleKeyPress(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		ss.selectPreviousField()
		return ss, nil
	case "down", "j":
		ss.selectNextField()
		return ss, nil
	case "enter", " ":
		return ss.enterEditMode()
	case "s", "ctrl+s":
		return ss, ss.saveSettings()
	case "r", "ctrl+r":
		return ss, ss.resetSettings()
	case "t":
		ss.toggleTheme()
		return ss, nil
	case "escape":
		// later integrate confirm dialog; for now noop
		return ss, nil
	}
	return ss, nil
}

func (ss *SettingsScreen) handleEditMode(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		return ss.commitEdit()
	case tea.KeyEsc:
		ss.cancelEdit()
		return ss, nil
	}

	var cmd tea.Cmd
	ss.input, cmd = ss.input.Update(msg)
	return ss, cmd
}

func (ss *SettingsScreen) handleResize(msg tea.WindowSizeMsg) {
	ss.SetSize(msg.Width, msg.Height-1)
	ss.state.menuWidth = settingsMenuWidth
	if ss.Width() < settingsMinWidth {
		ss.state.menuWidth = ss.Width() / 3
	}
	ss.state.contentWidth = ss.Width() - ss.state.menuWidth
	if ss.state.contentWidth > 4 {
		ss.input.Width = ss.state.contentWidth - 4
	}
}

func (ss *SettingsScreen) selectPreviousField() {
	if ss.state.selectedField == ThemeField {
		ss.state.selectedField = LogLevelField
		return
	}
	ss.state.selectedField--
}

func (ss *SettingsScreen) selectNextField() {
	if ss.state.selectedField == LogLevelField {
		ss.state.selectedField = ThemeField
		return
	}
	ss.state.selectedField++
}

func (ss *SettingsScreen) enterEditMode() (Screen, tea.Cmd) {
	ss.state.editMode = true
	ss.input.SetValue(ss.getCurrentValue())
	if ss.state.contentWidth > 4 {
		ss.input.Width = ss.state.contentWidth - 4
	}
	ss.input.CursorEnd()
	ss.input.Focus()
	return ss, nil
}

func (ss *SettingsScreen) commitEdit() (Screen, tea.Cmd) {
	value := ss.input.Value()
	ss.setCurrentValue(value)
	ss.state.editMode = false
	ss.input.Blur()
	ss.recalcChangeState()
	return ss, ss.validateField(ss.state.selectedField)
}

func (ss *SettingsScreen) cancelEdit() {
	ss.state.editMode = false
	ss.input.Blur()
}

func (ss *SettingsScreen) toggleTheme() {
	if ss.config.Theme == "dark" {
		ss.config.Theme = "light"
	} else {
		ss.config.Theme = "dark"
	}
	ss.recalcChangeState()
}

func (ss *SettingsScreen) saveSettings() tea.Cmd {
	return func() tea.Msg {
		if err := ss.config.SaveDefault(); err != nil {
			return settingsErrorMsg{Error: err}
		}
		ss.original = *ss.config
		ss.recalcChangeState()
		clone := ss.original
		return ConfigChangedMsg{Config: &clone}
	}
}

func (ss *SettingsScreen) resetSettings() tea.Cmd {
	*ss.config = ss.original
	ss.recalcChangeState()
	return ss.validateAllFields()
}

func (ss *SettingsScreen) recalcChangeState() {
	ss.state.hasChanges = false
	for _, field := range allSettingsFields() {
		if ss.isFieldChanged(field) {
			ss.state.hasChanges = true
			break
		}
	}
}

func (ss *SettingsScreen) validateField(field SettingsField) tea.Cmd {
	switch field {
	case SurgeBinaryField:
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			result := ValidationResult{Valid: true, Message: "Surge binary accessible"}
			if err := ss.config.ValidateSurgeBinary(ctx); err != nil {
				result.Valid = false
				result.Message = "Surge binary not found or not executable"
			}
			return validationCompleteMsg{Field: field, Result: result, Timestamp: time.Now()}
		}
	case DefaultProjectField:
		path := ss.config.DefaultProject
		return func() tea.Msg {
			result := ValidationResult{Valid: true, Message: ""}
			if path != "" {
				if _, err := filepath.Abs(path); err != nil {
					result.Valid = false
					result.Message = "Invalid path"
				}
			}
			return validationCompleteMsg{Field: field, Result: result, Timestamp: time.Now()}
		}
	default:
		return func() tea.Msg {
			return validationCompleteMsg{Field: field, Result: ValidationResult{Valid: true}, Timestamp: time.Now()}
		}
	}
}

func (ss *SettingsScreen) validateAllFields() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(allSettingsFields()))
	for _, field := range allSettingsFields() {
		cmds = append(cmds, ss.validateField(field))
	}
	return tea.Batch(cmds...)
}
