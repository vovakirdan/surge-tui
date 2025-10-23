package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommandEntry описывает одну команду в палитре.
type CommandEntry struct {
	ID      string
	Title   string
	Key     string
	Context string
	Enabled bool
	RawKey  string
}

// CommandFetcher возвращает доступные команды.
type CommandFetcher func() []CommandEntry

// CommandExecuteMsg сообщает приложению, какую команду нужно выполнить.
type CommandExecuteMsg struct {
	ID string
}

// CommandPaletteClosedMsg сигнал закрытия палитры без выбора.
type CommandPaletteClosedMsg struct{}

// CommandPaletteScreen отображает список команд с фильтром.
type CommandPaletteScreen struct {
	BaseScreen

	fetch    CommandFetcher
	filter   textinput.Model
	entries  []CommandEntry
	filtered []CommandEntry
	selected int
}

func NewCommandPaletteScreen(fetch CommandFetcher) *CommandPaletteScreen {
	ti := textinput.New()
	ti.Placeholder = "Filter commands"
	ti.Focus()

	return &CommandPaletteScreen{
		BaseScreen: NewBaseScreen("Command Palette"),
		fetch:      fetch,
		filter:     ti,
	}
}

func (ps *CommandPaletteScreen) Init() tea.Cmd {
	ps.refresh()
	return nil
}

func (ps *CommandPaletteScreen) OnEnter() tea.Cmd {
	ps.refresh()
	ps.filter.SetValue("")
	ps.selected = 0
	ps.applyFilter()
	return nil
}

func (ps *CommandPaletteScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		ps.SetSize(m.Width, m.Height-1)
		ps.filter.Width = ps.Width() - 4
		return ps, nil
	case tea.KeyMsg:
		switch m.String() {
		case "up", "shift+tab", "k":
			if ps.selected > 0 {
				ps.selected--
			}
			return ps, nil
		case "down", "tab", "j":
			if ps.selected < len(ps.filtered)-1 {
				ps.selected++
			}
			return ps, nil
		case "ctrl+r":
			ps.refresh()
			return ps, nil
		case "enter":
			if ps.selected >= 0 && ps.selected < len(ps.filtered) {
				entry := ps.filtered[ps.selected]
				if entry.Enabled {
					return ps, func() tea.Msg { return CommandExecuteMsg{ID: entry.ID} }
				}
			}
			return ps, nil
		case "escape":
			return ps, func() tea.Msg { return CommandPaletteClosedMsg{} }
		}
		before := ps.filter.Value()
		var cmd tea.Cmd
		ps.filter, cmd = ps.filter.Update(m)
		if ps.filter.Value() != before {
			ps.applyFilter()
		}
		return ps, cmd
	}
	return ps, nil
}

func (ps *CommandPaletteScreen) View() string {
	width := ps.Width()
	if width <= 0 {
		width = 80
	}
	if width < 20 {
		width = 20
	}
	ps.filter.Width = width - 4

	builder := lipgloss.NewStyle().Padding(1).Width(width - 2)
	var lines []string
	if len(ps.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor)).Render("No commands match filter"))
	}
	for i, entry := range ps.filtered {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor))
		if !entry.Enabled {
			style = style.Faint(true)
		}
		if i == ps.selected {
			prefix = "→ "
			style = style.Foreground(lipgloss.Color(selectedColor)).Bold(true)
		}
		keyInfo := ""
		if entry.Key != "" {
			keyInfo = lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor)).Render(" [" + entry.Key + "]")
		}
		line := style.Render(prefix + entry.Title)
		if entry.Context != "" {
			context := lipgloss.NewStyle().Foreground(lipgloss.Color(unselectedColor)).Render(" — " + entry.Context)
			line += context
		}
		line += keyInfo
		lines = append(lines, line)
	}

	list := strings.Join(lines, "\n")
	content := builder.Render(ps.filter.View() + "\n\n" + list)
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(width).Render(content)
}

func (ps *CommandPaletteScreen) refresh() {
	if ps.fetch == nil {
		ps.entries = nil
		ps.filtered = nil
		return
	}
	ps.entries = ps.fetch()
	ps.applyFilter()
}

func (ps *CommandPaletteScreen) applyFilter() {
	filter := strings.ToLower(strings.TrimSpace(ps.filter.Value()))
	filtered := make([]CommandEntry, 0, len(ps.entries))
	for _, entry := range ps.entries {
		if filter == "" || strings.Contains(strings.ToLower(entry.Title), filter) || strings.Contains(strings.ToLower(entry.Key), filter) || strings.Contains(strings.ToLower(entry.Context), filter) || strings.Contains(strings.ToLower(entry.RawKey), filter) {
			filtered = append(filtered, entry)
		}
	}
	ps.filtered = filtered
	if len(ps.filtered) == 0 {
		ps.selected = -1
	} else if ps.selected >= len(ps.filtered) {
		ps.selected = len(ps.filtered) - 1
	} else if ps.selected < 0 {
		ps.selected = 0
	}
}
