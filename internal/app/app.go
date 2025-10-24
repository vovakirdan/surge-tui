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
	"surge-tui/internal/platform"
	"surge-tui/internal/ui/components"
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
	projectPath    string
	lastOpenedFile string
	unsavedFiles   map[string]bool
	lastError      error

	// Surge CLI
	surgeClient    *core.Client
	surgeAvailable bool
	surgeVersion   string

	quitDialog *components.ConfirmDialog
}

type projectInitCommander interface {
	CanInitProject() bool
	InitProjectInSelectedDir() tea.Cmd
}

type escHandler interface {
	HandleGlobalEsc() (bool, tea.Cmd)
}

// New создает новое приложение
func New(cfg *config.Config, projectPath string) *App {
	app := &App{
		config:         cfg,
		projectPath:    projectPath,
		lastOpenedFile: "",
		screens:        make(map[ScreenType]screens.Screen),
		eventBus:       NewEventBus(),
		theme:          styles.NewTheme(cfg.Theme),
		unsavedFiles:   make(map[string]bool),
		commands:       NewCommandRegistry(),
		quitDialog:     components.NewConfirmDialog("Quit surge-tui", "Exit the application? Unsaved changes may be lost."),
	}

	if app.quitDialog != nil {
		app.quitDialog.ConfirmText = "Quit"
		app.quitDialog.CancelText = "Cancel"
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
	case screens.OpenLocationMsg:
		return a, a.handleOpenLocation(msg)
	case screens.OpenFixModeMsg:
		return a, a.handleOpenFixMode(msg)
	case ProjectInitializedMsg:
		if msg.Err != nil {
			a.lastError = msg.Err
		} else {
			if msg.Path != "" {
				a.projectPath = msg.Path
			}
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
			if fixScreen, ok := a.screens[FixModeScreen].(*screens.FixModeScreen); ok && fixScreen != nil {
				fixScreen.SetProjectPath(a.projectPath)
			}
		}
		return a, nil
	case quitConfirmedMsg:
		if msg.confirmed {
			return a, tea.Quit
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

	content := fmt.Sprintf("%s\n%s", view, statusBar)
	if a.quitDialog != nil && a.quitDialog.Visible {
		content = fmt.Sprintf("%s\n%s", content, a.quitDialog.View())
	}

	return content
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
		return screens.NewProjectScreenReal(a.projectPath, a.config)
	case EditorScreen:
		return screens.NewEditorScreen(a.config)
	case BuildScreen:
		return screens.NewDiagnosticsScreen(a.projectPath, a.surgeClient)
	case FixModeScreen:
		return screens.NewFixModeScreen(a.projectPath, a.surgeClient)
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
	keyLabel := func(id, fallback string) string {
		if a.config != nil && a.config.Keybindings != nil {
			if key := strings.TrimSpace(a.config.Keybindings[id]); key != "" {
				return prettifyKey(key)
			}
		}
		return prettifyKey(fallback)
	}
	help := fmt.Sprintf("%s Quit • %s Commands • %s Switch Screens",
		keyLabel("quit", "ctrl+q"),
		keyLabel("command_palette", "ctrl+p"),
		keyLabel("switch_screen", "tab"),
	)
	return a.theme.StatusBar(fmt.Sprintf("%s | %s | %s", proj, surge, help))
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

	regScreen := func(screenType ScreenType, id, title, key string, run func(*App) tea.Cmd, enabled func(*App) bool) {
		st := screenType
		cmd := &Command{
			ID:      id,
			Title:   title,
			Key:     key,
			Screen:  &st,
			Enabled: enabled,
			Run:     run,
		}
		a.commands.Register(cmd)
	}

	projectAvailable := func(a *App) bool {
		ps := a.projectScreen()
		return a.currentScreen == ProjectScreen && ps != nil
	}
	projectHasTab := func(a *App) bool {
		ps := a.projectScreen()
		return a.currentScreen == ProjectScreen && ps != nil && ps.HasOpenTab()
	}
	diagnosticsAvailable := func(a *App) bool {
		ds := a.diagnosticsScreen()
		return a.currentScreen == BuildScreen && ds != nil
	}
	diagnosticsHaveEntries := func(a *App) bool {
		ds := a.diagnosticsScreen()
		return a.currentScreen == BuildScreen && ds != nil && ds.HasEntries()
	}
	fixAvailable := func(a *App) bool {
		fs := a.fixModeScreen()
		return a.currentScreen == FixModeScreen && fs != nil
	}
	fixHasEntries := func(a *App) bool {
		fs := a.fixModeScreen()
		return a.currentScreen == FixModeScreen && fs != nil && fs.HasEntries()
	}

	reg("quit", "Quit", kb["quit"], func(a *App) tea.Cmd { return a.requestQuit() }, nil)
	reg("open_settings", "Open Settings", kb["settings"], func(a *App) tea.Cmd { return a.router.SwitchTo(SettingsScreen) }, nil)
	reg("open_fix_mode", "Fix Mode", kb["fix_mode"], func(a *App) tea.Cmd { return a.router.SwitchTo(FixModeScreen) }, nil)
	reg("open_workspace", "Workspace", kb["workspace"], func(a *App) tea.Cmd { return a.router.SwitchTo(ProjectScreen) }, nil)
	reg("open_diagnostics", "Diagnostics", kb["build"], func(a *App) tea.Cmd { return a.router.SwitchTo(BuildScreen) }, nil)
	reg("command_palette", "Command Palette", kb["command_palette"], func(a *App) tea.Cmd { return a.router.SwitchTo(CommandPaletteScreen) }, nil)
	reg("switch_screen", "Next Screen", kb["switch_screen"], func(a *App) tea.Cmd { return a.router.SwitchToNext() }, nil)
	reg("switch_screen_back", "Prev Screen", kb["switch_screen_back"], func(a *App) tea.Cmd { return a.router.SwitchToPrevious() }, nil)
	reg("init_project", "Init Project", kb["init_project"], func(a *App) tea.Cmd { return a.initProject() }, func(a *App) bool {
		if !a.surgeAvailable || a.surgeClient == nil || a.currentScreen != ProjectScreen {
			return false
		}
		screen := a.getCurrentScreen()
		if commander, ok := screen.(projectInitCommander); ok {
			return commander.CanInitProject()
		}
		return false
	})
	reg("help", "Help", kb["help"], func(a *App) tea.Cmd { return a.router.SwitchTo(HelpScreen) }, nil)

	// Project screen commands
	regScreen(ProjectScreen, "project_open_selected", "Open Selected", "enter", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.OpenSelectedEntryCmd()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_toggle_directory", "Toggle Directory", "space", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ToggleSelectedDirectory()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_new_file", "New File", "n", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.ShowNewFileDialog()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_new_directory", "New Directory", "shift+n", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.ShowNewDirectoryDialog()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_rename", "Rename", "r", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.ShowRenameDialog()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_delete", "Delete", "delete", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.RequestDeleteSelected()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_toggle_hidden", "Toggle Hidden Files", "h", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ToggleHiddenEntries()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_toggle_surge_filter", "Toggle .sg Filter", "s", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ToggleSurgeFilter()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_refresh", "Refresh Tree", "ctrl+r", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.RefreshFileTree()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_focus_editor", "Focus Editor", "ctrl+right", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.FocusEditorPanel()
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_focus_tree", "Focus Tree", "ctrl+left", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.FocusFileTree()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_open_in_editor", "Open In Editor", "alt+enter", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.OpenSelectedInEditor()
		}
		return nil
	}, projectAvailable)
	regScreen(ProjectScreen, "project_tab_next", "Next Tab", "alt+right", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ActivateNextTab()
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_tab_prev", "Previous Tab", "alt+left", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ActivatePreviousTab()
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_tab_reorder_left", "Reorder Tab Left", "alt+shift+left", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ReorderTabLeft()
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_tab_reorder_right", "Reorder Tab Right", "alt+shift+right", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			ps.ReorderTabRight()
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_tab_close", "Close Tab", "ctrl+w", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.CloseActiveTabCmd(false)
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_save", "Save File", "ctrl+s", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.SaveActiveTabCmd()
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_cmd_write", ":w", ":w", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.RunEditorCommand("w")
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_cmd_quit", ":q", ":q", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.RunEditorCommand("q")
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_cmd_quit_force", ":q!", ":q!", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.RunEditorCommand("q!")
		}
		return nil
	}, projectHasTab)
	regScreen(ProjectScreen, "project_cmd_write_quit", ":wq", ":wq", func(a *App) tea.Cmd {
		if ps := a.projectScreen(); ps != nil {
			return ps.RunEditorCommand("wq")
		}
		return nil
	}, projectHasTab)

	// Diagnostics screen commands
	regScreen(BuildScreen, "diagnostics_run", "Run Diagnostics", "ctrl+r", func(a *App) tea.Cmd {
		if ds := a.diagnosticsScreen(); ds != nil {
			return ds.TriggerDiagnostics()
		}
		return nil
	}, diagnosticsAvailable)
	regScreen(BuildScreen, "diagnostics_open", "Open In Workspace", "enter", func(a *App) tea.Cmd {
		if ds := a.diagnosticsScreen(); ds != nil {
			return ds.OpenSelectedCmd()
		}
		return nil
	}, diagnosticsHaveEntries)
	regScreen(BuildScreen, "diagnostics_toggle_notes", "Toggle Notes", "n", func(a *App) tea.Cmd {
		if ds := a.diagnosticsScreen(); ds != nil {
			return ds.ToggleNotesCmd()
		}
		return nil
	}, diagnosticsAvailable)
	regScreen(BuildScreen, "diagnostics_open_fix_mode", "Open Fix Mode", "f", func(a *App) tea.Cmd {
		if ds := a.diagnosticsScreen(); ds != nil {
			return ds.OpenFixModeCmd()
		}
		return nil
	}, diagnosticsHaveEntries)

	// Fix mode commands
	regScreen(FixModeScreen, "fix_refresh", "Refresh Fixes", "ctrl+r", func(a *App) tea.Cmd {
		if fs := a.fixModeScreen(); fs != nil {
			return fs.RefreshCmd()
		}
		return nil
	}, fixAvailable)
	regScreen(FixModeScreen, "fix_apply_selected", "Apply Fix", "a", func(a *App) tea.Cmd {
		if fs := a.fixModeScreen(); fs != nil {
			return fs.ApplySelectedCmd()
		}
		return nil
	}, fixHasEntries)
	regScreen(FixModeScreen, "fix_apply_all", "Apply All Fixes", "shift+a", func(a *App) tea.Cmd {
		if fs := a.fixModeScreen(); fs != nil {
			return fs.ApplyAllCmd()
		}
		return nil
	}, fixHasEntries)
	regScreen(FixModeScreen, "fix_toggle_suggested", "Toggle Suggested Fixes", "tab", func(a *App) tea.Cmd {
		if fs := a.fixModeScreen(); fs != nil {
			return fs.ToggleSuggestedCmd()
		}
		return nil
	}, fixAvailable)

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

func (a *App) handleOpenLocation(msg screens.OpenLocationMsg) tea.Cmd {
	var cmds []tea.Cmd

	if msg.FilePath != "" {
		if screen, ok := a.screens[ProjectScreen].(*screens.ProjectScreenReal); ok && screen != nil {
			screen.OpenLocation(msg.FilePath, msg.Line, msg.Column)
		}
	}

	cmds = append(cmds, a.router.SwitchTo(ProjectScreen))
	return tea.Batch(cmds...)
}

func (a *App) handleOpenFixMode(msg screens.OpenFixModeMsg) tea.Cmd {
	var cmds []tea.Cmd
	screenIface := a.screens[FixModeScreen]
	if screenIface == nil {
		screenIface = a.createScreen(FixModeScreen)
		a.screens[FixModeScreen] = screenIface
		if init := screenIface.Init(); init != nil {
			cmds = append(cmds, init)
		}
	}
	if screen, ok := screenIface.(*screens.FixModeScreen); ok && screen != nil {
		if a.projectPath != "" {
			screen.SetProjectPath(a.projectPath)
		}
		screen.FocusFix(msg.FilePath, msg.FixID)
	}
	cmds = append(cmds, a.router.SwitchTo(FixModeScreen))
	return tea.Batch(cmds...)
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

func (a *App) projectScreen() *screens.ProjectScreenReal {
	screen, _ := a.screens[ProjectScreen].(*screens.ProjectScreenReal)
	return screen
}

func (a *App) diagnosticsScreen() *screens.DiagnosticsScreen {
	screen, _ := a.screens[BuildScreen].(*screens.DiagnosticsScreen)
	return screen
}

func (a *App) fixModeScreen() *screens.FixModeScreen {
	screen, _ := a.screens[FixModeScreen].(*screens.FixModeScreen)
	return screen
}

func prettifyKey(key string) string {
	return platform.DisplayKey(key)
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

type quitConfirmedMsg struct {
	confirmed bool
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

// initProject вызывает `surge init` для текущего выбраного пути на экране проекта
func (a *App) initProject() tea.Cmd {
	if a.surgeClient == nil || !a.surgeAvailable {
		return nil
	}
	if screen := a.getCurrentScreen(); screen != nil {
		if commander, ok := screen.(projectInitCommander); ok {
			return commander.InitProjectInSelectedDir()
		}
	}
	return nil
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
