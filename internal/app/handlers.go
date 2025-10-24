package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"surge-tui/internal/platform"
)

// handleGlobalKeys обрабатывает глобальные горячие клавиши
func (a *App) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rawKey := msg.String()
	canonicalKey := platform.CanonicalKeyForLookup(rawKey)

	if a.quitDialog != nil && a.quitDialog.Visible {
		if cmd := a.quitDialog.Update(msg); cmd != nil {
			return a, cmd
		}
		return a, nil
	}

	// Сначала пытаемся найти команду через реестр
	if cmd := a.commands.Resolve(rawKey, a.currentScreen); cmd != nil {
		if cmd.Enabled == nil || cmd.Enabled(a) {
			return a, cmd.Run(a)
		}
		return a, nil
	}

	switch {
	case platform.MatchesKey(rawKey, "ctrl+c"):
		return a, a.requestQuit()
	case canonicalKey == "esc":
		current := a.getCurrentScreen()
		if handler, ok := current.(escHandler); ok {
			if handled, cmd := handler.HandleGlobalEsc(); handled {
				return a, cmd
			}
		}
		return a, a.router.SwitchTo(ProjectScreen)
	}

	// Если глобальные клавиши не обработаны, передаем экрану
	currentScreen := a.getCurrentScreen()
	if currentScreen != nil {
		updatedScreen, cmd := currentScreen.Update(msg)
		a.screens[a.currentScreen] = updatedScreen
		return a, cmd
	}

	return a, nil
}

// handleWindowResize обрабатывает изменение размера окна
func (a *App) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	// Обновляем размеры в теме
	a.theme.SetDimensions(msg.Width, msg.Height)

	// Передаем всем экранам
	var cmds []tea.Cmd
	for screenType, screen := range a.screens {
		if screen != nil {
			updatedScreen, cmd := screen.Update(msg)
			a.screens[screenType] = updatedScreen
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return a, tea.Batch(cmds...)
}

// handleScreenSwitch обрабатывает переключение экранов
func (a *App) handleScreenSwitch(msg ScreenSwitchMsg) (tea.Model, tea.Cmd) {
	// Проверяем, можно ли покинуть текущий экран
	currentScreen := a.getCurrentScreen()
	if currentScreen != nil && !currentScreen.CanExit() {
		// Показываем диалог подтверждения
		return a, nil // TODO: реализовать диалог
	}

	// Выходим из текущего экрана
	var cmds []tea.Cmd
	if currentScreen != nil {
		if exit := currentScreen.OnExit(); exit != nil {
			cmds = append(cmds, exit)
		}
	}

	// Переключаемся на новый экран
	a.currentScreen = msg.ScreenType

	// Инициализируем новый экран если нужно
	newScreen := a.screens[a.currentScreen]
	created := false
	if newScreen == nil {
		newScreen = a.createScreen(a.currentScreen)
		a.screens[a.currentScreen] = newScreen
		created = true
	}

	// Прокидываем последнюю известную геометрию окна в новый экран,
	// иначе у него останутся нулевые размеры и он будет показывать "Loading..."
	if newScreen != nil && a.theme.Width() > 0 && a.theme.Height() > 0 {
		if updated, cmd := newScreen.Update(tea.WindowSizeMsg{Width: a.theme.Width(), Height: a.theme.Height()}); updated != nil {
			newScreen = updated
			a.screens[a.currentScreen] = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	if created && newScreen != nil {
		if init := newScreen.Init(); init != nil {
			cmds = append(cmds, init)
		}
	}

	// Входим в новый экран
	if newScreen != nil {
		if enter := newScreen.OnEnter(); enter != nil {
			cmds = append(cmds, enter)
		}
	}

	if len(cmds) == 0 {
		return a, nil
	}
	return a, tea.Batch(cmds...)
}

// handleError обрабатывает ошибки
func (a *App) handleError(msg ErrorMsg) (tea.Model, tea.Cmd) {
	a.lastError = msg.Error
	// TODO: показать уведомление об ошибке
	return a, nil
}

func (a *App) requestQuit() tea.Cmd {
	if a.quitDialog == nil {
		return tea.Quit
	}
	if a.quitDialog.Visible {
		return nil
	}

	ch := a.quitDialog.Show()
	return func() tea.Msg {
		confirmed := <-ch
		return quitConfirmedMsg{confirmed: confirmed}
	}
}
