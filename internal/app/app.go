package app

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "surge-tui/internal/config"
    core "surge-tui/internal/core/surge"
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
    commands      *CommandRegistry

    // Глобальное состояние
    projectPath   string
    unsavedFiles  map[string]bool
    lastError     error

    // Surge CLI
    surgeClient    *core.Client
    surgeAvailable bool
    surgeVersion   string
}

// New создает новое приложение
func New(cfg *config.Config, projectPath string) *App {
    app := &App{
        config:       cfg,
        projectPath:  projectPath,
        screens:      make(map[ScreenType]screens.Screen),
        eventBus:     NewEventBus(),
        theme:        styles.NewTheme(cfg.Theme),
        unsavedFiles: make(map[string]bool),
        commands:     NewCommandRegistry(),
    }

    // Путь к проекту: CLI → конфиг → текущая директория
    if app.projectPath == "" {
        if cfg.DefaultProject != "" {
            app.projectPath = cfg.DefaultProject
        } else if wd, err := os.Getwd(); err == nil {
            app.projectPath = wd
        }
    }

    // Инициализируем клиента surge с путём из конфига
    app.surgeClient = core.NewClient(cfg.SurgeBinary)

    // Инициализируем роутер
    app.router = NewScreenRouter(app)

    // Регистрируем базовые глобальные команды из конфига
    app.registerBaseCommands()

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
        return tea.Batch(screen.Init(), a.checkSurgeAvailability())
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
    case SurgeAvailabilityMsg:
        a.surgeAvailable = msg.Available
        a.surgeVersion = msg.Version
        if msg.Err != nil {
            a.lastError = msg.Err
        }
        return a, nil
    case ProjectInitializedMsg:
        if msg.Err != nil {
            a.lastError = msg.Err
        } else {
            // Переинициализируем экран проекта чтобы перечитать дерево
            a.screens[ProjectScreen] = a.createScreen(ProjectScreen)
        }
        return a, nil
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
    key := msg.String()

    // Сначала пытаемся найти команду через реестр
    if cmd := a.commands.Resolve(key, a.currentScreen); cmd != nil {
        if cmd.Enabled == nil || cmd.Enabled(a) {
            return a, cmd.Run(a)
        }
        return a, nil
    }

    switch key {
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
    case "ctrl+i":
        return a, a.initProject()
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

    // Прокидываем последнюю известную геометрию окна в новый экран,
    // иначе у него останутся нулевые размеры и он будет показывать "Loading..."
    if newScreen != nil && a.theme.Width() > 0 && a.theme.Height() > 0 {
        if updated, _ := newScreen.Update(tea.WindowSizeMsg{Width: a.theme.Width(), Height: a.theme.Height()}); updated != nil {
            a.screens[a.currentScreen] = updated
            newScreen = updated
        }
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
        return screens.NewProjectScreenReal(a.projectPath)
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
		return screens.NewSettingsScreen(a.config)
	case HelpScreen:
		return screens.NewPlaceholderScreen("Help")
	case LogsScreen:
		return screens.NewPlaceholderScreen("Logs")
	default:
		return screens.NewPlaceholderScreen("Unknown")
	}
}

// ConfigChangedMsg сообщение об изменении конфигурации
type ConfigChangedMsg struct {
	Config *config.Config
}

// renderStatusBar отрисовывает статус-бар
func (a *App) renderStatusBar() string {
    proj := a.projectLabel()
    surge := "Surge: unknown"
    if a.surgeAvailable {
        if a.surgeVersion != "" {
            surge = "Surge: " + a.surgeVersion
        } else {
            surge = "Surge: available"
        }
    } else {
        surge = "Surge: not found"
    }
    help := "Ctrl+Q Quit • Ctrl+P Commands • F1 Help"
    return a.theme.StatusBar(fmt.Sprintf("%s | %s | %s", proj, surge, help))
}

// registerBaseCommands wires global commands from config keybindings.
func (a *App) registerBaseCommands() {
    kb := a.config.Keybindings
    // Helper to register a global command
    reg := func(id, title, key string, run func(*App) tea.Cmd, enabled func(*App) bool) {
        if key == "" {
            return
        }
        a.commands.Register(&Command{
            ID:      id,
            Title:   title,
            Key:     key,
            Screen:  nil,
            Enabled: enabled,
            Run:     run,
        })
    }

    reg("quit", "Quit", kb["quit"], func(a *App) tea.Cmd { return tea.Quit }, nil)
    reg("open_settings", "Open Settings", kb["settings"], func(a *App) tea.Cmd { return a.router.SwitchTo(SettingsScreen) }, nil)
    reg("command_palette", "Command Palette", kb["command_palette"], func(a *App) tea.Cmd { return a.router.SwitchTo(CommandPaletteScreen) }, nil)
    reg("switch_screen", "Next Screen", kb["switch_screen"], func(a *App) tea.Cmd { return a.router.SwitchToNext() }, nil)
    reg("switch_screen_back", "Prev Screen", kb["switch_screen_back"], func(a *App) tea.Cmd { return a.router.SwitchToPrevious() }, nil)
    reg("init_project", "Init Project", kb["init_project"], func(a *App) tea.Cmd { return a.initProject() }, func(a *App) bool {
        return a.surgeAvailable && !a.isSurgeProject() && a.projectPath != ""
    })
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

// Вспомогательные сообщения
type SurgeAvailabilityMsg struct {
    Available bool
    Version   string
    Err       error
}

type ProjectInitializedMsg struct {
    Path string
    Err  error
}

// checkSurgeAvailability проверяет наличие surge и версию
func (a *App) checkSurgeAvailability() tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        if a.surgeClient == nil {
            return SurgeAvailabilityMsg{Available: false}
        }
        err := a.surgeClient.CheckAvailable(ctx)
        if err != nil {
            return SurgeAvailabilityMsg{Available: false, Err: err}
        }
        ver, _ := a.surgeClient.GetVersion(ctx)
        return SurgeAvailabilityMsg{Available: true, Version: ver}
    }
}

// initProject вызывает `surge init` для текущего пути проекта
func (a *App) initProject() tea.Cmd {
    if a.surgeClient == nil || !a.surgeAvailable || a.projectPath == "" || a.isSurgeProject() {
        return nil
    }
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        err := a.surgeClient.InitProject(ctx, a.projectPath)
        return ProjectInitializedMsg{Path: a.projectPath, Err: err}
    }
}

// projectLabel формирует подпись проекта для статус-бара
func (a *App) projectLabel() string {
    if a.projectPath == "" {
        return "Project: (none)"
    }
    return "Project: " + filepath.Base(a.projectPath)
}

// isSurgeProject проверяет наличие surge.toml в корне
func (a *App) isSurgeProject() bool {
    if a.projectPath == "" {
        return false
    }
    if st, err := os.Stat(filepath.Join(a.projectPath, "surge.toml")); err == nil && !st.IsDir() {
        return true
    }
    return false
}
