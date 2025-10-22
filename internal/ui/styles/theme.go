package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme содержит все стили приложения
type Theme struct {
	// Размеры экрана
	width  int
	height int

	// Цветовая схема
	colors ColorScheme

	// Стили компонентов
	StatusBarStyle   lipgloss.Style
	TitleStyle       lipgloss.Style
	SubtitleStyle    lipgloss.Style
	TextStyle        lipgloss.Style
	HighlightStyle   lipgloss.Style
	ErrorStyle       lipgloss.Style
	SuccessStyle     lipgloss.Style
	WarningStyle     lipgloss.Style
	BorderStyle      lipgloss.Style
	ActiveTabStyle   lipgloss.Style
	InactiveTabStyle lipgloss.Style
	ButtonStyle      lipgloss.Style
	InputStyle       lipgloss.Style
}

// ColorScheme цветовая схема
type ColorScheme struct {
	Primary     string
	Secondary   string
	Accent      string
	Background  string
	Surface     string
	Text        string
	TextDim     string
	Error       string
	Success     string
	Warning     string
	Border      string
	BorderFocus string
}

// Предустановленные цветовые схемы
var (
	DarkScheme = ColorScheme{
		Primary:     "#7C3AED", // Фиолетовый
		Secondary:   "#10B981", // Зеленый
		Accent:      "#F59E0B", // Оранжевый
		Background:  "#0F172A", // Темно-синий
		Surface:     "#1E293B", // Темно-серый
		Text:        "#F1F5F9", // Светло-серый
		TextDim:     "#94A3B8", // Серый
		Error:       "#EF4444", // Красный
		Success:     "#10B981", // Зеленый
		Warning:     "#F59E0B", // Оранжевый
		Border:      "#334155", // Серый
		BorderFocus: "#7C3AED", // Фиолетовый
	}

	LightScheme = ColorScheme{
		Primary:     "#7C3AED", // Фиолетовый
		Secondary:   "#059669", // Зеленый
		Accent:      "#D97706", // Оранжевый
		Background:  "#FFFFFF", // Белый
		Surface:     "#F8FAFC", // Светло-серый
		Text:        "#0F172A", // Темно-синий
		TextDim:     "#64748B", // Серый
		Error:       "#DC2626", // Красный
		Success:     "#059669", // Зеленый
		Warning:     "#D97706", // Оранжевый
		Border:      "#E2E8F0", // Светло-серый
		BorderFocus: "#7C3AED", // Фиолетовый
	}
)

// NewTheme создает новую тему
func NewTheme(themeName string) *Theme {
	var colors ColorScheme
	switch themeName {
	case "light":
		colors = LightScheme
	default:
		colors = DarkScheme
	}

	theme := &Theme{
		colors: colors,
	}

	theme.initStyles()
	return theme
}

// initStyles инициализирует стили
func (t *Theme) initStyles() {
	t.StatusBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.colors.Surface)).
		Foreground(lipgloss.Color(t.colors.Text)).
		Padding(0, 1)

	t.TitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.Primary)).
		Bold(true).
		Padding(0, 1)

	t.SubtitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.TextDim)).
		Padding(0, 1)

	t.TextStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.Text))

	t.HighlightStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.Accent)).
		Bold(true)

	t.ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.Error)).
		Bold(true)

	t.SuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.Success)).
		Bold(true)

	t.WarningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.colors.Warning)).
		Bold(true)

	t.BorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.colors.Border))

	t.ActiveTabStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.colors.Primary)).
		Foreground(lipgloss.Color(t.colors.Background)).
		Padding(0, 2).
		Bold(true)

	t.InactiveTabStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.colors.Surface)).
		Foreground(lipgloss.Color(t.colors.TextDim)).
		Padding(0, 2)

	t.ButtonStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.colors.Primary)).
		Foreground(lipgloss.Color(t.colors.Background)).
		Padding(0, 2).
		Margin(0, 1).
		Bold(true)

	t.InputStyle = lipgloss.NewStyle().
		Background(lipgloss.Color(t.colors.Surface)).
		Foreground(lipgloss.Color(t.colors.Text)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(t.colors.Border)).
		Padding(0, 1)
}

// SetDimensions устанавливает размеры экрана
func (t *Theme) SetDimensions(width, height int) {
	t.width = width
	t.height = height
}

// Width возвращает ширину экрана
func (t *Theme) Width() int {
	return t.width
}

// Height возвращает высоту экрана
func (t *Theme) Height() int {
	return t.height
}

// Методы для быстрого применения стилей

// StatusBar рендерит статус-бар
func (t *Theme) StatusBar(text string) string {
	return t.StatusBarStyle.
		Width(t.width).
		Render(text)
}

// TitleBar рендерит заголовок
func (t *Theme) TitleBar(title string) string {
	return t.TitleStyle.Render(title)
}

// ErrorMessage рендерит сообщение об ошибке
func (t *Theme) ErrorMessage(text string) string {
	return t.ErrorStyle.Render("Error: " + text)
}

// SuccessMessage рендерит сообщение об успехе
func (t *Theme) SuccessMessage(text string) string {
	return t.SuccessStyle.Render("✓ " + text)
}

// WarningMessage рендерит предупреждение
func (t *Theme) WarningMessage(text string) string {
	return t.WarningStyle.Render("⚠ " + text)
}