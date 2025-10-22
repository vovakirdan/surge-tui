package screens

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Screen интерфейс для всех экранов приложения
type Screen interface {
	// Bubble Tea методы
	Init() tea.Cmd
	Update(tea.Msg) (Screen, tea.Cmd)
	View() string

	// Методы жизненного цикла экрана
	OnEnter() tea.Cmd // Вызывается при входе на экран
	OnExit() tea.Cmd  // Вызывается при выходе с экрана
	CanExit() bool    // Можно ли покинуть экран (например, есть несохраненные изменения)

	// Метаданные экрана
	Title() string      // Заголовок экрана для статус-бара
	ShortHelp() string  // Краткая справка по горячим клавишам
	FullHelp() []string // Полная справка
}

// BaseScreen базовая реализация экрана с общей функциональностью
type BaseScreen struct {
	width  int
	height int
	title  string
}

// NewBaseScreen создает базовый экран
func NewBaseScreen(title string) BaseScreen {
	return BaseScreen{
		title: title,
	}
}

// SetSize устанавливает размеры экрана
func (bs *BaseScreen) SetSize(width, height int) {
	bs.width = width
	bs.height = height
}

// Width возвращает ширину экрана
func (bs *BaseScreen) Width() int {
	return bs.width
}

// Height возвращает высоту экрана
func (bs *BaseScreen) Height() int {
	return bs.height
}

// Title возвращает заголовок экрана
func (bs *BaseScreen) Title() string {
	return bs.title
}

// CanExit базовая реализация - можно всегда выйти
func (bs *BaseScreen) CanExit() bool {
	return true
}

// OnEnter базовая реализация - ничего не делаем
func (bs *BaseScreen) OnEnter() tea.Cmd {
	return nil
}

// OnExit базовая реализация - ничего не делаем
func (bs *BaseScreen) OnExit() tea.Cmd {
	return nil
}

// ShortHelp базовая реализация справки
func (bs *BaseScreen) ShortHelp() string {
	return "Tab: Switch screens • Ctrl+Q: Quit"
}

// FullHelp базовая реализация полной справки
func (bs *BaseScreen) FullHelp() []string {
	return []string{
		"Navigation:",
		"  Tab/Shift+Tab - Switch between screens",
		"  Ctrl+P - Command palette",
		"  F1 - Help",
		"",
		"Global:",
		"  Ctrl+Q - Quit application",
		"  Ctrl+, - Settings",
	}
}