package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ScreenRouter управляет переключением между экранами
type ScreenRouter struct {
	app     *App
	history []ScreenType // История переходов для навигации назад
}

// NewScreenRouter создает новый роутер
func NewScreenRouter(app *App) *ScreenRouter {
	return &ScreenRouter{
		app:     app,
		history: make([]ScreenType, 0),
	}
}

// SwitchTo переключается на указанный экран
func (r *ScreenRouter) SwitchTo(screenType ScreenType) tea.Cmd {
	// Добавляем текущий экран в историю
	if len(r.history) == 0 || r.history[len(r.history)-1] != r.app.currentScreen {
		r.history = append(r.history, r.app.currentScreen)
	}

	return func() tea.Msg {
		return ScreenSwitchMsg{ScreenType: screenType}
	}
}

// SwitchToNext переключается на следующий экран по порядку
func (r *ScreenRouter) SwitchToNext() tea.Cmd {
	nextScreen := r.getNextScreen(r.app.currentScreen)
	return r.SwitchTo(nextScreen)
}

// SwitchToPrevious переключается на предыдущий экран по порядку
func (r *ScreenRouter) SwitchToPrevious() tea.Cmd {
	prevScreen := r.getPreviousScreen(r.app.currentScreen)
	return r.SwitchTo(prevScreen)
}

// GoBack возвращается к предыдущему экрану из истории
func (r *ScreenRouter) GoBack() tea.Cmd {
	if len(r.history) == 0 {
		return nil
	}

	// Берем последний экран из истории
	lastScreen := r.history[len(r.history)-1]
	r.history = r.history[:len(r.history)-1]

	return func() tea.Msg {
		return ScreenSwitchMsg{ScreenType: lastScreen}
	}
}

// getNextScreen возвращает следующий экран в циклическом порядке
func (r *ScreenRouter) getNextScreen(current ScreenType) ScreenType {
	screens := []ScreenType{
		ProjectScreen,
		BuildScreen,
		FixModeScreen,
		SettingsScreen,
		HelpScreen,
	}

	for i, screen := range screens {
		if screen == current {
			return screens[(i+1)%len(screens)]
		}
	}

	return ProjectScreen
}

// getPreviousScreen возвращает предыдущий экран в циклическом порядке
func (r *ScreenRouter) getPreviousScreen(current ScreenType) ScreenType {
	screens := []ScreenType{
		ProjectScreen,
		BuildScreen,
		FixModeScreen,
		SettingsScreen,
		HelpScreen,
	}

	for i, screen := range screens {
		if screen == current {
			if i == 0 {
				return screens[len(screens)-1]
			}
			return screens[i-1]
		}
	}

	return ProjectScreen
}

// CanNavigateBack проверяет, можно ли вернуться назад
func (r *ScreenRouter) CanNavigateBack() bool {
	return len(r.history) > 0
}

// ClearHistory очищает историю навигации
func (r *ScreenRouter) ClearHistory() {
	r.history = r.history[:0]
}
