package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	projectPath  string
	unsavedFiles map[string]bool
	lastError    error

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
	case screens.CommandExecuteMsg:
		var cmds []tea.Cmd
		if back := a.router.GoBack(); back != nil {
			cmds = append(cmds, back)
		}
		if run := a.commands.Run(msg.ID, a); run != nil {
			cmds = append(cmds, run)
		}
		return a, tea.Batch(cmds...)
	case screens.CommandPaletteClosedMsg:
		return a, a.router.GoBack()
	case screens.ConfigChangedMsg:
		if msg.Config != nil {
			*a.config = *msg.Config
			a.theme = styles.NewTheme(a.config.Theme)
			a.rebuildCommandBindings()
		}
		return a, nil
	case screens.OpenFileMsg:
		return a, a.router.SwitchTo(EditorScreen)
	case ProjectInitializedMsg:
		if msg.Err != nil {
			a.lastError = msg.Err
		} else {
			var cmds []tea.Cmd
			newScreen := a.createScreen(ProjectScreen)
			// передаем последнюю известную геометрию
			if a.theme.Width() > 0 && a.theme.Height() > 0 {
				if updated, cmd := newScreen.Update(tea.WindowSizeMsg{Width: a.theme.Width(), Height: a.theme.Height()}); updated != nil {
					newScreen = updated
					if cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
			}
			if initCmd := newScreen.Init(); initCmd != nil {
				cmds = append(cmds, initCmd)
			}
			a.screens[ProjectScreen] = newScreen
			if a.currentScreen == ProjectScreen {
				if enter := newScreen.OnEnter(); enter != nil {
					cmds = append(cmds, enter)
				}
			}
			if len(cmds) > 0 {
				return a, tea.Batch(cmds...)
			}
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

// createScreen создает экран по типу
func (a *App) createScreen(screenType ScreenType) screens.Screen {
	switch screenType {
	case ProjectScreen:
		return screens.NewProjectScreenReal(a.projectPath)
	case EditorScreen:
		return screens.NewPlaceholderScreen("Editor")
	case BuildScreen:
		return screens.NewPlaceholderScreen("Build & Diagnostics")
	case FixModeScreen:
		return screens.NewPlaceholderScreen("Fix Mode")
	case CommandPaletteScreen:
		return screens.NewCommandPaletteScreen(a.commandFetcher())
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
	screensInfo := a.screenShortcutsSummary()
	help := "Ctrl+Q Quit • Ctrl+P Commands • F1 Help"
	return a.theme.StatusBar(fmt.Sprintf("%s | %s | %s | %s", proj, surge, screensInfo, help))
}

// registerBaseCommands wires global commands from config keybindings.
func (a *App) registerBaseCommands() {
	kb := a.config.Keybindings
	// Helper to register a global command
	reg := func(id, title, key string, run func(*App) tea.Cmd, enabled func(*App) bool) {
		cmd := &Command{
			ID:      id,
			Title:   title,
			Key:     key,
			Screen:  nil,
			Enabled: enabled,
			Run:     run,
		}
		a.commands.Register(cmd)
	}

	reg("quit", "Quit", kb["quit"], func(a *App) tea.Cmd { return tea.Quit }, nil)
	reg("open_settings", "Open Settings", kb["settings"], func(a *App) tea.Cmd { return a.router.SwitchTo(SettingsScreen) }, nil)
	reg("command_palette", "Command Palette", kb["command_palette"], func(a *App) tea.Cmd { return a.router.SwitchTo(CommandPaletteScreen) }, nil)
	reg("switch_screen", "Next Screen", kb["switch_screen"], func(a *App) tea.Cmd { return a.router.SwitchToNext() }, nil)
	reg("switch_screen_back", "Prev Screen", kb["switch_screen_back"], func(a *App) tea.Cmd { return a.router.SwitchToPrevious() }, nil)
	reg("init_project", "Init Project", kb["init_project"], func(a *App) tea.Cmd { return a.initProject() }, func(a *App) bool {
		return a.surgeAvailable && !a.isSurgeProject() && a.projectPath != ""
	})
	reg("goto_project", "Go to Project", kb["goto_project"], func(a *App) tea.Cmd { return a.router.SwitchTo(ProjectScreen) }, nil)
	reg("goto_editor", "Go to Editor", kb["goto_editor"], func(a *App) tea.Cmd { return a.router.SwitchTo(EditorScreen) }, nil)
	reg("goto_build", "Go to Build", kb["goto_build"], func(a *App) tea.Cmd { return a.router.SwitchTo(BuildScreen) }, nil)

	if cmd := a.commands.Get("goto_project"); cmd != nil {
		s := ProjectScreen
		cmd.Screen = &s
	}
	if cmd := a.commands.Get("goto_editor"); cmd != nil {
		s := EditorScreen
		cmd.Screen = &s
	}
	if cmd := a.commands.Get("goto_build"); cmd != nil {
		s := BuildScreen
		cmd.Screen = &s
	}
}

func (a *App) rebuildCommandBindings() {
	a.commands = NewCommandRegistry()
	a.registerBaseCommands()
}

func (a *App) commandFetcher() screens.CommandFetcher {
	return func() []screens.CommandEntry {
		slice := a.commands.All()
		entries := make([]screens.CommandEntry, 0, len(slice))
		for _, cmd := range slice {
			context := "Global"
			if cmd.Screen != nil {
				context = a.screenTitle(*cmd.Screen)
			}
			displayKey := prettifyKey(cmd.Key)
			if displayKey == "" {
				displayKey = "—"
			}
			entries = append(entries, screens.CommandEntry{
				ID:      cmd.ID,
				Title:   cmd.Title,
				Key:     displayKey,
				Context: context,
				Enabled: cmd.Enabled == nil || cmd.Enabled(a),
				RawKey:  cmd.Key,
			})
		}
		return entries
	}
}

func (a *App) screenTitle(screen ScreenType) string {
	switch screen {
	case ProjectScreen:
		return "Project"
	case EditorScreen:
		return "Editor"
	case BuildScreen:
		return "Build"
	case FixModeScreen:
		return "Fix Mode"
	case CommandPaletteScreen:
		return "Command Palette"
	case SettingsScreen:
		return "Settings"
	case HelpScreen:
		return "Help"
	case LogsScreen:
		return "Logs"
	default:
		return "Unknown"
	}
}

func (a *App) screenShortcutsSummary() string {
	kb := a.config.Keybindings
	type pair struct {
		label string
		key   string
	}
	items := []pair{
		{"Proj", kb["goto_project"]},
		{"Edit", kb["goto_editor"]},
		{"Build", kb["goto_build"]},
	}
	var parts []string
	for _, item := range items {
		if item.key == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%s", item.label, prettifyKey(item.key)))
	}
	if len(parts) == 0 {
		return "Screens: —"
	}
	return "Screens " + strings.Join(parts, " • ")
}

func prettifyKey(key string) string {
	if key == "" {
		return ""
	}
	parts := strings.Split(key, "+")
	for i, part := range parts {
		p := strings.TrimSpace(part)
		if len(p) == 0 {
			continue
		}
		if len(p) == 1 {
			parts[i] = strings.ToUpper(p)
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, "+")
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
