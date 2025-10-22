package screens

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/config"
)

const (
	// Settings screen layout
	SettingsMenuWidth = 25
	SettingsMinWidth  = 80

	// Colors for settings
	SelectedColor     = "#7C3AED"
	UnselectedColor   = "#94A3B8"
	ValidColor        = "#10B981"
	InvalidColor      = "#EF4444"
	ModifiedColor     = "#F59E0B"
)

// SettingsField represents a configurable field
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

// SettingsScreen экран настроек приложения
type SettingsScreen struct {
	BaseScreen

	// Состояние
	config        *config.Config
	originalConfig *config.Config
	selectedField SettingsField
	editMode      bool
	editValue     string
	hasChanges    bool

	// Валидация
	validationStatus map[SettingsField]ValidationResult
	surgeCheckTime   time.Time

	// Размеры
	menuWidth    int
	contentWidth int
}

// ValidationResult результат валидации поля
type ValidationResult struct {
	Valid   bool
	Message string
}

// NewSettingsScreen создает новый экран настроек
func NewSettingsScreen(cfg *config.Config) *SettingsScreen {
	// Создаем копию конфигурации для редактирования
	original := *cfg
	editable := *cfg

	return &SettingsScreen{
		BaseScreen:       NewBaseScreen("Settings"),
		config:           &editable,
		originalConfig:   &original,
		selectedField:    ThemeField,
		editMode:         false,
		validationStatus: make(map[SettingsField]ValidationResult),
		menuWidth:        SettingsMenuWidth,
	}
}

// Init инициализирует экран
func (ss *SettingsScreen) Init() tea.Cmd {
	return ss.validateAllFields()
}

// Update обрабатывает сообщения
func (ss *SettingsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ss.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		ss.handleResize(msg)
		return ss, nil
	case validationCompleteMsg:
		ss.validationStatus[msg.Field] = msg.Result
		if msg.Field == SurgeBinaryField {
			ss.surgeCheckTime = time.Now()
		}
		return ss, nil
	}

	return ss, nil
}

// View отрисовывает экран
func (ss *SettingsScreen) View() string {
	if ss.Width() == 0 {
		return "Loading settings..."
	}

	leftPanel := ss.renderMenu()
	rightPanel := ss.renderContent()

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

// handleKeyPress обрабатывает нажатия клавиш
func (ss *SettingsScreen) handleKeyPress(msg tea.KeyMsg) (Screen, tea.Cmd) {
	if ss.editMode {
		return ss.handleEditMode(msg)
	}

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
		// Быстрое переключение темы
		if ss.config.Theme == "dark" {
			ss.config.Theme = "light"
		} else {
			ss.config.Theme = "dark"
		}
		ss.markChanged()
		return ss, nil
	case "escape":
		if ss.hasChanges {
			// TODO: показать диалог подтверждения
			return ss, nil
		}
		return ss, nil
	}

	return ss, nil
}

// handleEditMode обрабатывает ввод в режиме редактирования
func (ss *SettingsScreen) handleEditMode(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return ss.commitEdit()
	case "escape":
		ss.cancelEdit()
		return ss, nil
	case "backspace":
		if len(ss.editValue) > 0 {
			ss.editValue = ss.editValue[:len(ss.editValue)-1]
		}
		return ss, nil
	case "ctrl+a":
		// Выделить всё (пока просто очистим)
		ss.editValue = ""
		return ss, nil
	default:
		// Добавляем обычные символы
		if len(msg.String()) == 1 {
			ss.editValue += msg.String()
		}
		return ss, nil
	}
}

// handleResize обрабатывает изменение размера
func (ss *SettingsScreen) handleResize(msg tea.WindowSizeMsg) {
	ss.SetSize(msg.Width, msg.Height-1) // -1 для статус-бара

	ss.menuWidth = SettingsMenuWidth
	if ss.Width() < SettingsMinWidth {
		ss.menuWidth = ss.Width() / 3
	}
	ss.contentWidth = ss.Width() - ss.menuWidth
}

// selectPreviousField переходит к предыдущему полю
func (ss *SettingsScreen) selectPreviousField() {
	if ss.selectedField == ThemeField {
		ss.selectedField = LogLevelField
	} else {
		ss.selectedField--
	}
}

// selectNextField переходит к следующему полю
func (ss *SettingsScreen) selectNextField() {
	if ss.selectedField == LogLevelField {
		ss.selectedField = ThemeField
	} else {
		ss.selectedField++
	}
}

// enterEditMode входит в режим редактирования
func (ss *SettingsScreen) enterEditMode() (Screen, tea.Cmd) {
	ss.editMode = true
	ss.editValue = ss.getCurrentValue()
	return ss, nil
}

// commitEdit применяет изменения
func (ss *SettingsScreen) commitEdit() (Screen, tea.Cmd) {
	if err := ss.setCurrentValue(ss.editValue); err != nil {
		// TODO: показать ошибку
	} else {
		ss.markChanged()
	}

	ss.editMode = false
	ss.editValue = ""

	// Валидируем измененное поле
	return ss, ss.validateField(ss.selectedField)
}

// cancelEdit отменяет редактирование
func (ss *SettingsScreen) cancelEdit() {
	ss.editMode = false
	ss.editValue = ""
}

// getCurrentValue возвращает текущее значение выбранного поля
func (ss *SettingsScreen) getCurrentValue() string {
	switch ss.selectedField {
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
	}
	return ""
}

// setCurrentValue устанавливает значение выбранного поля
func (ss *SettingsScreen) setCurrentValue(value string) error {
	switch ss.selectedField {
	case ThemeField:
		if value == "dark" || value == "light" {
			ss.config.Theme = value
		}
	case SurgeBinaryField:
		ss.config.SurgeBinary = value
	case DefaultProjectField:
		ss.config.DefaultProject = value
	case TabSizeField:
		if size, err := strconv.Atoi(value); err == nil && size > 0 && size <= 16 {
			ss.config.Editor.TabSize = size
		}
	case UseSpacesField:
		ss.config.Editor.UseSpaces = (value == "true" || value == "yes" || value == "1")
	case AutoSaveField:
		ss.config.Editor.AutoSave = (value == "true" || value == "yes" || value == "1")
	case AutoSaveDelayField:
		if delay, err := strconv.Atoi(value); err == nil && delay > 0 {
			ss.config.Editor.AutoSaveDelay = delay
		}
	case ExternalEditorField:
		ss.config.Editor.ExternalEditor = value
	case SyntaxHighlightField:
		ss.config.Editor.SyntaxHighlight = (value == "true" || value == "yes" || value == "1")
	case MaxFileSizeField:
		value = strings.TrimSuffix(strings.ToLower(value), "mb")
		if size, err := strconv.ParseInt(value, 10, 64); err == nil && size > 0 {
			ss.config.Performance.MaxFileSize = size * 1024 * 1024
		}
	case RefreshRateField:
		value = strings.TrimSuffix(strings.ToLower(value), "ms")
		if rate, err := strconv.Atoi(value); err == nil && rate >= 10 && rate <= 1000 {
			ss.config.Performance.RefreshRate = rate
		}
	case LogLevelField:
		if value == "debug" || value == "info" || value == "warn" || value == "error" {
			ss.config.Logging.Level = value
		}
	}
	return nil
}

// markChanged отмечает наличие изменений
func (ss *SettingsScreen) markChanged() {
	ss.hasChanges = true
}

// saveSettings сохраняет настройки
func (ss *SettingsScreen) saveSettings() tea.Cmd {
	return func() tea.Msg {
		if err := ss.config.SaveDefault(); err != nil {
			return settingsErrorMsg{Error: err}
		}
		// Обновляем оригинальную конфигурацию
		*ss.originalConfig = *ss.config
		return settingsSavedMsg{}
	}
}

// resetSettings сбрасывает настройки
func (ss *SettingsScreen) resetSettings() tea.Cmd {
	*ss.config = *ss.originalConfig
	ss.hasChanges = false
	return ss.validateAllFields()
}

// validateField валидирует одно поле
func (ss *SettingsScreen) validateField(field SettingsField) tea.Cmd {
	switch field {
	case SurgeBinaryField:
		return func() tea.Msg {
			result := ValidationResult{Valid: true, Message: ""}
			if err := ss.config.ValidateSurgeBinary(); err != nil {
				result.Valid = false
				result.Message = "Surge binary not found or not executable"
			} else {
				result.Message = "Surge binary accessible"
			}
			return validationCompleteMsg{Field: field, Result: result}
		}
	case DefaultProjectField:
		return func() tea.Msg {
			result := ValidationResult{Valid: true, Message: ""}
			if ss.config.DefaultProject != "" {
				if _, err := filepath.Abs(ss.config.DefaultProject); err != nil {
					result.Valid = false
					result.Message = "Invalid path"
				}
			}
			return validationCompleteMsg{Field: field, Result: result}
		}
	default:
		return func() tea.Msg {
			return validationCompleteMsg{
				Field:  field,
				Result: ValidationResult{Valid: true, Message: ""},
			}
		}
	}
}

// validateAllFields валидирует все поля
func (ss *SettingsScreen) validateAllFields() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, ss.validateField(SurgeBinaryField))
	cmds = append(cmds, ss.validateField(DefaultProjectField))
	return tea.Batch(cmds...)
}

// renderMenu отрисовывает левое меню
func (ss *SettingsScreen) renderMenu() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(InactiveBorderColor)).
		Width(ss.menuWidth - 1).
		Height(ss.Height() - 2).
		Padding(1)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(SelectedColor)).
		Render("⚙️  Settings")

	var items []string
	fields := []struct {
		field SettingsField
		name  string
	}{
		{ThemeField, "Theme"},
		{SurgeBinaryField, "Surge Binary"},
		{DefaultProjectField, "Default Project"},
		{TabSizeField, "Tab Size"},
		{UseSpacesField, "Use Spaces"},
		{AutoSaveField, "Auto Save"},
		{AutoSaveDelayField, "Auto Save Delay"},
		{ExternalEditorField, "External Editor"},
		{SyntaxHighlightField, "Syntax Highlight"},
		{MaxFileSizeField, "Max File Size"},
		{RefreshRateField, "Refresh Rate"},
		{LogLevelField, "Log Level"},
	}

	for _, item := range fields {
		indicator := "  "
		color := UnselectedColor

		if item.field == ss.selectedField {
			indicator = "▶ "
			color = SelectedColor
		}

		// Добавляем индикатор изменений
		changeIndicator := ""
		if ss.isFieldChanged(item.field) {
			changeIndicator = " •"
		}

		// Добавляем индикатор валидации
		validationIndicator := ""
		if result, exists := ss.validationStatus[item.field]; exists {
			if result.Valid {
				validationIndicator = " ✓"
			} else {
				validationIndicator = " ✗"
			}
		}

		itemText := lipgloss.NewStyle().
			Foreground(lipgloss.Color(color)).
			Render(indicator + item.name + changeIndicator + validationIndicator)

		items = append(items, itemText)
	}

	content := strings.Join(items, "\n")

	// Добавляем помощь внизу
	help := "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color(UnselectedColor)).
		Render("↑↓: Navigate\nEnter: Edit\nS: Save\nR: Reset\nT: Toggle theme")

	return style.Render(title + "\n\n" + content + help)
}

// renderContent отрисовывает правую панель с содержимым
func (ss *SettingsScreen) renderContent() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(InactiveBorderColor)).
		Width(ss.contentWidth - 1).
		Height(ss.Height() - 2).
		Padding(1)

	content := ss.renderFieldContent()

	return style.Render(content)
}

// renderFieldContent отрисовывает содержимое выбранного поля
func (ss *SettingsScreen) renderFieldContent() string {
	fieldName := ss.getFieldName()
	currentValue := ss.getCurrentValue()
	description := ss.getFieldDescription()

	var content strings.Builder

	// Заголовок
	content.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(SelectedColor)).
		Render(fieldName))
	content.WriteString("\n\n")

	// Текущее значение
	valueLabel := "Current value: "
	if ss.editMode {
		valueLabel = "Editing: "
		currentValue = ss.editValue + "█" // Курсор
	}

	valueColor := UnselectedColor
	if ss.isFieldChanged(ss.selectedField) {
		valueColor = ModifiedColor
	}

	content.WriteString(valueLabel)
	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(valueColor)).
		Render(currentValue))
	content.WriteString("\n\n")

	// Описание
	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(UnselectedColor)).
		Render(description))

	// Валидация
	if result, exists := ss.validationStatus[ss.selectedField]; exists && result.Message != "" {
		content.WriteString("\n\n")
		validationColor := ValidColor
		if !result.Valid {
			validationColor = InvalidColor
		}
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(validationColor)).
			Render(result.Message))
	}

	// Специальная информация для surge binary
	if ss.selectedField == SurgeBinaryField && !ss.surgeCheckTime.IsZero() {
		content.WriteString("\n\n")
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(UnselectedColor)).
			Render(fmt.Sprintf("Last checked: %s", ss.surgeCheckTime.Format("15:04:05"))))
	}

	// Подсказки
	if ss.editMode {
		content.WriteString("\n\n")
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(UnselectedColor)).
			Render("Enter: Save • Escape: Cancel"))
	} else {
		content.WriteString("\n\n")
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(UnselectedColor)).
			Render("Enter: Edit • Space: Edit"))
	}

	return content.String()
}

// getFieldName возвращает название поля
func (ss *SettingsScreen) getFieldName() string {
	switch ss.selectedField {
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
	}
	return "Unknown"
}

// getFieldDescription возвращает описание поля
func (ss *SettingsScreen) getFieldDescription() string {
	switch ss.selectedField {
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
		return "Command to launch external editor (e.g., 'code', 'vim', 'nano')."
	case SyntaxHighlightField:
		return "Enable syntax highlighting for source files."
	case MaxFileSizeField:
		return "Maximum file size to open in editor (in megabytes)."
	case RefreshRateField:
		return "UI refresh rate in milliseconds (10-1000)."
	case LogLevelField:
		return "Logging level: debug, info, warn, error."
	}
	return ""
}

// isFieldChanged проверяет, изменено ли поле
func (ss *SettingsScreen) isFieldChanged(field SettingsField) bool {
	current := ss.getCurrentValueForField(field)
	original := ss.getOriginalValueForField(field)
	return current != original
}

// getCurrentValueForField возвращает текущее значение поля
func (ss *SettingsScreen) getCurrentValueForField(field SettingsField) string {
	oldField := ss.selectedField
	ss.selectedField = field
	value := ss.getCurrentValue()
	ss.selectedField = oldField
	return value
}

// getOriginalValueForField возвращает оригинальное значение поля
func (ss *SettingsScreen) getOriginalValueForField(field SettingsField) string {
	switch field {
	case ThemeField:
		return ss.originalConfig.Theme
	case SurgeBinaryField:
		return ss.originalConfig.SurgeBinary
	case DefaultProjectField:
		return ss.originalConfig.DefaultProject
	case TabSizeField:
		return strconv.Itoa(ss.originalConfig.Editor.TabSize)
	case UseSpacesField:
		if ss.originalConfig.Editor.UseSpaces {
			return "true"
		}
		return "false"
	case AutoSaveField:
		if ss.originalConfig.Editor.AutoSave {
			return "true"
		}
		return "false"
	case AutoSaveDelayField:
		return strconv.Itoa(ss.originalConfig.Editor.AutoSaveDelay)
	case ExternalEditorField:
		return ss.originalConfig.Editor.ExternalEditor
	case SyntaxHighlightField:
		if ss.originalConfig.Editor.SyntaxHighlight {
			return "true"
		}
		return "false"
	case MaxFileSizeField:
		return strconv.FormatInt(ss.originalConfig.Performance.MaxFileSize/(1024*1024), 10) + "MB"
	case RefreshRateField:
		return strconv.Itoa(ss.originalConfig.Performance.RefreshRate) + "ms"
	case LogLevelField:
		return ss.originalConfig.Logging.Level
	}
	return ""
}

// Title возвращает заголовок экрана
func (ss *SettingsScreen) Title() string {
	title := "Settings"
	if ss.hasChanges {
		title += " (modified)"
	}
	return title
}

// ShortHelp возвращает краткую справку
func (ss *SettingsScreen) ShortHelp() string {
	return "↑↓: Navigate • Enter: Edit • S: Save • R: Reset • T: Toggle theme"
}

// FullHelp возвращает полную справку
func (ss *SettingsScreen) FullHelp() []string {
	help := ss.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Settings Screen:",
		"  ↑/↓ or j/k - Navigate between settings",
		"  Enter or Space - Edit selected setting",
		"  S or Ctrl+S - Save settings to file",
		"  R or Ctrl+R - Reset to original values",
		"  T - Quick toggle theme (dark/light)",
		"  Escape - Cancel edit or exit (with confirmation)",
		"",
		"Edit Mode:",
		"  Enter - Confirm changes",
		"  Escape - Cancel changes",
		"  Ctrl+A - Select all (clear)",
		"  Backspace - Delete character",
	}...)
	return help
}

// Сообщения для экрана настроек

type validationCompleteMsg struct {
	Field  SettingsField
	Result ValidationResult
}

type settingsSavedMsg struct{}

type settingsErrorMsg struct {
	Error error
}
