package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"surge-tui/internal/config"
	"surge-tui/internal/ui/screens"
	"surge-tui/internal/ui/styles"
)

// ScreenType определяет тип экрана
type ScreenType int

const (
	ProjectScreen ScreenType = iota
	FileManagerScreen
	EditorScreen
	BuildScreen
	FixModeScreen
	CommandPaletteScreen
	SettingsScreen
	HelpScreen
	LogsScreen
)

// App представляет главное приложение
type App struct {
	config        *config.Config
	currentScreen ScreenType
	screens       map[ScreenType]screens.Screen
	router        *ScreenRouter
	eventBus      *EventBus
	theme         *styles.Theme

	// Глобальное состояние
	projectPath   string
	unsavedFiles  map[string]bool
	lastError     error
}

// New создает новое приложение
func New(cfg *config.Config) *App {
	app := &App{
		config:       cfg,
		screens:      make(map[ScreenType]screens.Screen),
		eventBus:     NewEventBus(),
		theme:        styles.NewTheme(cfg.Theme),
		unsavedFiles: make(map[string]bool),
	}

	// Инициализируем роутер
	app.router = NewScreenRouter(app)

	// Создаем экраны (ленивая инициализация)
	app.initScreens()

	return app
}

// Init инициализирует приложение (Bubble Tea)
func (a *App) Init() tea.Cmd {
	// Создаем первый экран
	a.currentScreen = ProjectScreen
	a.screens[ProjectScreen] = a.createScreen(ProjectScreen)

	// Инициализируем экран
	if screen := a.getCurrentScreen(); screen != nil {
		return screen.Init()
	}

	return nil
}

// Update обрабатывает сообщения (Bubble Tea)
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleGlobalKeys(msg)
	case tea.WindowSizeMsg:
		return a.handleWindowResize(msg)
	case ScreenSwitchMsg:
		return a.handleScreenSwitch(msg)
	case ErrorMsg:
		return a.handleError(msg)
	}

	// Передаем сообщение текущему экрану
	currentScreen := a.getCurrentScreen()
	if currentScreen != nil {
		updatedScreen, cmd := currentScreen.Update(msg)
		a.screens[a.currentScreen] = updatedScreen
		return a, cmd
	}

	return a, nil
}

// View отрисовывает приложение (Bubble Tea)
func (a *App) View() string {
	currentScreen := a.getCurrentScreen()
	if currentScreen == nil {
		return "Loading..."
	}

	view := currentScreen.View()

	// Добавляем статус-бар
	statusBar := a.renderStatusBar()

	return fmt.Sprintf("%s\n%s", view, statusBar)
}

// getCurrentScreen возвращает текущий экран
func (a *App) getCurrentScreen() screens.Screen {
	return a.screens[a.currentScreen]
}

// initScreens инициализирует все экраны
func (a *App) initScreens() {
	// TODO: Создавать экраны по мере необходимости
	// Пока просто заглушки
}

// handleGlobalKeys обрабатывает глобальные горячие клавиши
func (a *App) handleGlobalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return a.handleQuit()
	case "ctrl+p":
		return a, a.router.SwitchTo(CommandPaletteScreen)
	case "f1":
		return a, a.router.SwitchTo(HelpScreen)
	case "ctrl+comma":
		return a, a.router.SwitchTo(SettingsScreen)
	case "tab":
		return a, a.router.SwitchToNext()
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
	var exitCmd tea.Cmd
	if currentScreen != nil {
		exitCmd = currentScreen.OnExit()
	}

	// Переключаемся на новый экран
	a.currentScreen = msg.ScreenType

	// Инициализируем новый экран если нужно
	newScreen := a.screens[a.currentScreen]
	if newScreen == nil {
		newScreen = a.createScreen(a.currentScreen)
		a.screens[a.currentScreen] = newScreen
	}

	// Входим в новый экран
	var enterCmd tea.Cmd
	if newScreen != nil {
		enterCmd = newScreen.OnEnter()
	}

	return a, tea.Batch(exitCmd, enterCmd)
}

// handleError обрабатывает ошибки
func (a *App) handleError(msg ErrorMsg) (tea.Model, tea.Cmd) {
	a.lastError = msg.Error
	// TODO: показать уведомление об ошибке
	return a, nil
}

// handleQuit обрабатывает выход из приложения
func (a *App) handleQuit() (tea.Model, tea.Cmd) {
	// Проверяем несохраненные файлы
	if len(a.unsavedFiles) > 0 {
		// TODO: показать диалог подтверждения
	}

	return a, tea.Quit
}

// createScreen создает экран по типу
func (a *App) createScreen(screenType ScreenType) screens.Screen {
	switch screenType {
	case ProjectScreen:
		return screens.NewPlaceholderScreen("Project")
	case FileManagerScreen:
		return screens.NewPlaceholderScreen("File Manager")
	case EditorScreen:
		return screens.NewPlaceholderScreen("Editor")
	case BuildScreen:
		return screens.NewPlaceholderScreen("Build & Diagnostics")
	case FixModeScreen:
		return screens.NewPlaceholderScreen("Fix Mode")
	case CommandPaletteScreen:
		return screens.NewPlaceholderScreen("Command Palette")
	case SettingsScreen:
		return screens.NewPlaceholderScreen("Settings")
	case HelpScreen:
		return screens.NewPlaceholderScreen("Help")
	case LogsScreen:
		return screens.NewPlaceholderScreen("Logs")
	default:
		return screens.NewPlaceholderScreen("Unknown")
	}
}

// renderStatusBar отрисовывает статус-бар
func (a *App) renderStatusBar() string {
	// TODO: реализовать статус-бар с информацией о проекте, горячих клавишах и т.д.
	return a.theme.StatusBar("Ready | Ctrl+Q: Quit | Ctrl+P: Commands | F1: Help")
}

// Сообщения для приложения

// ScreenSwitchMsg сообщение о переключении экрана
type ScreenSwitchMsg struct {
	ScreenType ScreenType
}

// ErrorMsg сообщение об ошибке
type ErrorMsg struct {
	Error error
}