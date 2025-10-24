package screens

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/config"
	"surge-tui/internal/platform"
)

// SettingsScreen renders and edits application configuration.
type SettingsScreen struct {
	BaseScreen

	config   *config.Config
	original config.Config

	state SettingsScreenState
	input textinput.Model
}

// NewSettingsScreen constructs settings UI with editable copy of config.
func NewSettingsScreen(cfg *config.Config) *SettingsScreen {
	editable := *cfg

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 512

	screen := &SettingsScreen{
		BaseScreen: NewBaseScreen("Settings"),
		config:     &editable,
		original:   *cfg,
		input:      ti,
	}

	screen.state.validation = make(map[SettingsField]ValidationResult)
	screen.state.menuWidth = settingsMenuWidth
	screen.state.selectedField = ThemeField
	return screen
}

// Init kicks off initial validation.
func (ss *SettingsScreen) Init() tea.Cmd {
	return ss.validateAllFields()
}

// Update routes messages depending on edit mode.
func (ss *SettingsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		if ss.state.editMode {
			return ss.handleEditMode(m)
		}
		return ss.handleKeyPress(m)
	case tea.WindowSizeMsg:
		ss.handleResize(m)
		return ss, nil
	case validationCompleteMsg:
		if ss.state.validation == nil {
			ss.state.validation = make(map[SettingsField]ValidationResult)
		}
		ss.state.validation[m.Field] = m.Result
		if m.Field == SurgeBinaryField {
			ss.state.surgeCheck = m.Timestamp
		}
		return ss, nil
	case settingsErrorMsg:
		// TODO: surface error via notification system
		_ = m.Error
		return ss, nil
	case ConfigChangedMsg:
		if m.Config != nil {
			ss.original = *m.Config
			*ss.config = *m.Config
		}
		ss.recalcChangeState()
		return ss, ss.validateAllFields()
	}
	return ss, nil
}

// View renders the screen.
func (ss *SettingsScreen) View() string {
	if ss.Width() == 0 {
		return "Loading settings..."
	}
	left := ss.renderMenu()
	right := ss.renderContent()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// Title reflects pending changes.
func (ss *SettingsScreen) Title() string {
	if ss.state.hasChanges {
		return "Settings (modified)"
	}
	return "Settings"
}

// ShortHelp returns quick help line.
func (ss *SettingsScreen) ShortHelp() string {
	return "↑↓: Navigate • Enter: Edit • S: Save • R: Reset • T: Toggle theme"
}

// FullHelp details controls.
func (ss *SettingsScreen) FullHelp() []string {
	help := ss.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Settings Screen:",
		"  ↑/↓ or j/k - Navigate between settings",
		"  Enter or Space - Edit selected setting",
		platform.ReplacePrimaryModifier("  S or Ctrl+S - Save settings to file"),
		platform.ReplacePrimaryModifier("  R or Ctrl+R - Reset to original values"),
		"  T - Quick toggle theme (dark/light)",
		"  Escape - Cancel edit or exit",
		"",
		"Edit Mode:",
		"  Enter - Confirm changes",
		"  Escape - Cancel changes",
		"  Backspace/Delete - Remove character",
		"  Arrow keys - Move cursor",
	}...)
	return help
}
