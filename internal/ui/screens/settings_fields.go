package screens

import (
	"strconv"
	"strings"
)

func allSettingsFields() []SettingsField {
	return []SettingsField{
		ThemeField,
		SurgeBinaryField,
		DefaultProjectField,
		TabSizeField,
		UseSpacesField,
		AutoSaveField,
		AutoSaveDelayField,
		ExternalEditorField,
		SyntaxHighlightField,
		MaxFileSizeField,
		RefreshRateField,
		LogLevelField,
	}
}

func (ss *SettingsScreen) fieldName(field SettingsField) string {
	switch field {
	case ThemeField:
		return "Theme"
	case SurgeBinaryField:
		return "Surge Binary Path"
	case DefaultProjectField:
		return "Default Project Directory"
	case TabSizeField:
		return "Tab Size"
	case UseSpacesField:
		return "Use Spaces Instead of Tabs"
	case AutoSaveField:
		return "Auto Save Files"
	case AutoSaveDelayField:
		return "Auto Save Delay (seconds)"
	case ExternalEditorField:
		return "External Editor Command"
	case SyntaxHighlightField:
		return "Syntax Highlighting"
	case MaxFileSizeField:
		return "Maximum File Size"
	case RefreshRateField:
		return "UI Refresh Rate"
	case LogLevelField:
		return "Log Level"
	default:
		return "Unknown"
	}
}

func (ss *SettingsScreen) fieldDescription() string {
	switch ss.state.selectedField {
	case ThemeField:
		return "Choose between 'dark' and 'light' theme. Press 'T' for quick toggle."
	case SurgeBinaryField:
		return "Path to surge executable. Can be 'surge' (in PATH) or full path like '/path/to/surge'."
	case DefaultProjectField:
		return "Default directory to open when starting surge-tui without arguments."
	case TabSizeField:
		return "Number of spaces for tab indentation (1-16)."
	case UseSpacesField:
		return "Use spaces instead of tab characters for indentation."
	case AutoSaveField:
		return "Automatically save files after editing."
	case AutoSaveDelayField:
		return "Delay in seconds before auto-saving files."
	case ExternalEditorField:
		return "Command to launch external editor (e.g., 'code', 'vim')."
	case SyntaxHighlightField:
		return "Enable syntax highlighting for source files."
	case MaxFileSizeField:
		return "Maximum file size to open in editor (in megabytes)."
	case RefreshRateField:
		return "UI refresh rate in milliseconds (10-1000)."
	case LogLevelField:
		return "Logging level: debug, info, warn, error."
	default:
		return ""
	}
}

func (ss *SettingsScreen) getCurrentValue() string {
	return ss.valueFor(ss.state.selectedField)
}

func (ss *SettingsScreen) valueFor(field SettingsField) string {
	switch field {
	case ThemeField:
		return ss.config.Theme
	case SurgeBinaryField:
		return ss.config.SurgeBinary
	case DefaultProjectField:
		return ss.config.DefaultProject
	case TabSizeField:
		return strconv.Itoa(ss.config.Editor.TabSize)
	case UseSpacesField:
		if ss.config.Editor.UseSpaces {
			return "true"
		}
		return "false"
	case AutoSaveField:
		if ss.config.Editor.AutoSave {
			return "true"
		}
		return "false"
	case AutoSaveDelayField:
		return strconv.Itoa(ss.config.Editor.AutoSaveDelay)
	case ExternalEditorField:
		return ss.config.Editor.ExternalEditor
	case SyntaxHighlightField:
		if ss.config.Editor.SyntaxHighlight {
			return "true"
		}
		return "false"
	case MaxFileSizeField:
		return strconv.FormatInt(ss.config.Performance.MaxFileSize/(1024*1024), 10) + "MB"
	case RefreshRateField:
		return strconv.Itoa(ss.config.Performance.RefreshRate) + "ms"
	case LogLevelField:
		return ss.config.Logging.Level
	default:
		return ""
	}
}

func (ss *SettingsScreen) setCurrentValue(value string) {
	switch ss.state.selectedField {
	case ThemeField:
		if value == "dark" || value == "light" {
			ss.config.Theme = value
		}
	case SurgeBinaryField:
		ss.config.SurgeBinary = strings.TrimSpace(value)
	case DefaultProjectField:
		ss.config.DefaultProject = strings.TrimSpace(value)
	case TabSizeField:
		if n, err := strconv.Atoi(value); err == nil && n >= 1 && n <= 16 {
			ss.config.Editor.TabSize = n
		}
	case UseSpacesField:
		ss.config.Editor.UseSpaces = parseBool(value)
	case AutoSaveField:
		ss.config.Editor.AutoSave = parseBool(value)
	case AutoSaveDelayField:
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			ss.config.Editor.AutoSaveDelay = n
		}
	case ExternalEditorField:
		ss.config.Editor.ExternalEditor = strings.TrimSpace(value)
	case SyntaxHighlightField:
		ss.config.Editor.SyntaxHighlight = parseBool(value)
	case MaxFileSizeField:
		v := strings.TrimSuffix(strings.ToLower(value), "mb")
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && n > 0 {
			ss.config.Performance.MaxFileSize = n * 1024 * 1024
		}
	case RefreshRateField:
		v := strings.TrimSuffix(strings.ToLower(value), "ms")
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 10 && n <= 1000 {
			ss.config.Performance.RefreshRate = n
		}
	case LogLevelField:
		switch value {
		case "debug", "info", "warn", "error":
			ss.config.Logging.Level = value
		}
	}
}

func (ss *SettingsScreen) originalValue(field SettingsField) string {
	switch field {
	case ThemeField:
		return ss.original.Theme
	case SurgeBinaryField:
		return ss.original.SurgeBinary
	case DefaultProjectField:
		return ss.original.DefaultProject
	case TabSizeField:
		return strconv.Itoa(ss.original.Editor.TabSize)
	case UseSpacesField:
		if ss.original.Editor.UseSpaces {
			return "true"
		}
		return "false"
	case AutoSaveField:
		if ss.original.Editor.AutoSave {
			return "true"
		}
		return "false"
	case AutoSaveDelayField:
		return strconv.Itoa(ss.original.Editor.AutoSaveDelay)
	case ExternalEditorField:
		return ss.original.Editor.ExternalEditor
	case SyntaxHighlightField:
		if ss.original.Editor.SyntaxHighlight {
			return "true"
		}
		return "false"
	case MaxFileSizeField:
		return strconv.FormatInt(ss.original.Performance.MaxFileSize/(1024*1024), 10) + "MB"
	case RefreshRateField:
		return strconv.Itoa(ss.original.Performance.RefreshRate) + "ms"
	case LogLevelField:
		return ss.original.Logging.Level
	default:
		return ""
	}
}

func (ss *SettingsScreen) isFieldChanged(field SettingsField) bool {
	current := ss.valueFor(field)
	original := ss.originalValue(field)
	return current != original
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1", "on":
		return true
	default:
		return false
	}
}

// ensure relative path for display when default project empty
