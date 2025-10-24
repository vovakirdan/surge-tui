package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"surge-tui/internal/platform"
)

// PlaceholderScreen простой экран-заглушка для начальной разработки
type PlaceholderScreen struct {
	BaseScreen
	screenType string
}

// NewPlaceholderScreen создает новый экран-заглушку
func NewPlaceholderScreen(screenType string) *PlaceholderScreen {
	return &PlaceholderScreen{
		BaseScreen: NewBaseScreen(screenType),
		screenType: screenType,
	}
}

// Init инициализирует экран (Bubble Tea)
func (ps *PlaceholderScreen) Init() tea.Cmd {
	return nil
}

// Update обрабатывает сообщения (Bubble Tea)
func (ps *PlaceholderScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ps.SetSize(msg.Width, msg.Height-1)
		return ps, nil
	}
	return ps, nil
}

// View отрисовывает экран (Bubble Tea)
func (ps *PlaceholderScreen) View() string {
	content := "🚧 " + ps.screenType + " Screen\n\n"
	content += "This screen is under development.\n\n"
	content += "Available actions:\n"
	content += "• Tab - Switch to next screen\n"
	content += platform.ReplacePrimaryModifier("• Ctrl+Q - Quit application\n")
	content += "• F1 - Help\n\n"
	content += "Screen size: " + ps.dimensionsInfo()

	return content
}

// dimensionsInfo возвращает информацию о размерах экрана
func (ps *PlaceholderScreen) dimensionsInfo() string {
	if ps.Width() == 0 || ps.Height() == 0 {
		return "not set"
	}
	return fmt.Sprintf("%dx%d", ps.Width(), ps.Height())
}

// ShortHelp возвращает краткую справку
func (ps *PlaceholderScreen) ShortHelp() string {
	return platform.ReplacePrimaryModifier("Tab: Next screen • Ctrl+Q: Quit • F1: Help")
}
