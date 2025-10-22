package screens

import (
	"time"

	"surge-tui/internal/config"
)

// SettingsField represents configurable sections.
type SettingsField int

const (
	ThemeField SettingsField = iota
	SurgeBinaryField
	DefaultProjectField
	TabSizeField
	UseSpacesField
	AutoSaveField
	AutoSaveDelayField
	ExternalEditorField
	SyntaxHighlightField
	MaxFileSizeField
	RefreshRateField
	LogLevelField
)

// ValidationResult stores validation status for a field.
type ValidationResult struct {
	Valid   bool
	Message string
}

// validationCompleteMsg emitted when async validation done.
type validationCompleteMsg struct {
	Field     SettingsField
	Result    ValidationResult
	Timestamp time.Time
}

// settingsErrorMsg signals save failure.
type settingsErrorMsg struct {
	Error error
}

// ConfigChangedMsg notifies app that settings updated.
type ConfigChangedMsg struct {
	Config *config.Config
}

// SettingsScreenState groups state required across files.
type SettingsScreenState struct {
	selectedField SettingsField
	editMode      bool
	hasChanges    bool
	validation    map[SettingsField]ValidationResult
	surgeCheck    time.Time
	menuWidth     int
	contentWidth  int
}
