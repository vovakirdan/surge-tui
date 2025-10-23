package components

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmDialog предоставляет переиспользуемое окно подтверждения.
type ConfirmDialog struct {
	Title       string
	Description string
	ConfirmText string
	CancelText  string

	Visible  bool
	selected int // 0 = cancel, 1 = confirm
	result   chan bool
	mu       sync.Mutex
}

// NewConfirmDialog создает диалог с дефолтными кнопками.
func NewConfirmDialog(title, description string) *ConfirmDialog {
	return &ConfirmDialog{
		Title:       title,
		Description: description,
		ConfirmText: "Yes",
		CancelText:  "No",
	}
}

// Show делает диалог видимым и возвращает канал результата.
func (d *ConfirmDialog) Show() <-chan bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.Visible && d.result != nil {
		return d.result
	}

	d.result = make(chan bool, 1)
	d.Visible = true
	return d.result
}

// Hide скрывает диалог без результата.
func (d *ConfirmDialog) Hide() {
	d.respond(false)
}

// Update обрабатывает нажатия.
func (d *ConfirmDialog) Update(msg tea.Msg) tea.Cmd {
	d.mu.Lock()
	visible := d.Visible
	d.mu.Unlock()

	if !visible {
		return nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "left", "h":
			d.selected = 0 // Cancel
		case "right", "l":
			d.selected = 1 // Confirm
		case "y":
			d.respond(true)
		case "n", "escape":
			d.respond(false)
		case "enter":
			d.respond(d.selected == 1)
		}
	}
	return nil
}

// View отрисовывает диалог поверх остальных компонентов.
func (d *ConfirmDialog) View() string {
	d.mu.Lock()
	visible := d.Visible
	title := d.Title
	desc := d.Description
	confirm := d.ConfirmText
	cancel := d.CancelText
	selected := d.selected
	d.mu.Unlock()

	if !visible {
		return ""
	}

	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	titleView := lipgloss.NewStyle().Bold(true).Render(title)
	descView := lipgloss.NewStyle().Render(desc)

	// Стили для кнопок
	activeStyle := lipgloss.NewStyle().Background(lipgloss.Color("#7C3AED")).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Padding(0, 1)

	// Кнопки
	var cancelBtn, confirmBtn string
	if selected == 0 {
		cancelBtn = activeStyle.Render(cancel)
		confirmBtn = inactiveStyle.Render(confirm)
	} else {
		cancelBtn = inactiveStyle.Render(cancel)
		confirmBtn = activeStyle.Render(confirm)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, cancelBtn, "  ", confirmBtn)

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).
		Render("←→: Select • Y/N: Quick • Enter: Confirm • Esc: Cancel")

	return border.Render(fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", titleView, descView, buttons, hint))
}

func (d *ConfirmDialog) respond(value bool) {
	d.mu.Lock()
	if !d.Visible && d.result == nil {
		d.mu.Unlock()
		return
	}
	ch := d.result
	d.Visible = false
	d.result = nil
	d.mu.Unlock()

	if ch != nil {
		select {
		case ch <- value:
		default:
		}
	}
}

// InputDialog предоставляет переиспользуемое окно ввода текста.
type InputDialog struct {
	Title       string
	Placeholder string
	ConfirmText string
	CancelText  string

	Visible  bool
	selected int // 0 = cancel, 1 = confirm
	input    textinput.Model
	result   chan *string // nil означает отмену
	mu       sync.Mutex
}

// NewInputDialog создает диалог ввода с дефолтными кнопками.
func NewInputDialog(title, placeholder string) *InputDialog {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 40

	return &InputDialog{
		Title:       title,
		Placeholder: placeholder,
		ConfirmText: "OK",
		CancelText:  "Cancel",
		input:       ti,
	}
}

// Show делает диалог видимым и возвращает канал результата.
func (d *InputDialog) Show() <-chan *string {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.Visible && d.result != nil {
		return d.result
	}

	d.result = make(chan *string, 1)
	d.Visible = true
	d.input.Focus()
	d.input.SetValue("")
	d.input.SetCursor(0)
	return d.result
}

// ShowWithValue делает диалог видимым с предзаполненным значением.
func (d *InputDialog) ShowWithValue(value string) <-chan *string {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.Visible && d.result != nil {
		return d.result
	}

	d.result = make(chan *string, 1)
	d.Visible = true
	d.input.Focus()
	d.input.SetValue(value)
	d.input.SetCursor(len(value))
	return d.result
}

// Hide скрывает диалог без результата.
func (d *InputDialog) Hide() {
	d.respond(nil)
}

// Update обрабатывает нажатия.
func (d *InputDialog) Update(msg tea.Msg) tea.Cmd {
	d.mu.Lock()
	visible := d.Visible
	d.mu.Unlock()

	if !visible {
		return nil
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab":
			d.selected = (d.selected + 1) % 2
			return nil
		case "shift+tab":
			d.selected = (d.selected + 1) % 2
			return nil
		case "left", "h":
			d.selected = 0 // Cancel
			return nil
		case "right", "l":
			d.selected = 1 // Confirm
			return nil
		case "escape":
			d.respond(nil)
			return nil
		case "enter":
			if d.selected == 1 {
				value := strings.TrimSpace(d.input.Value())
				d.respond(&value)
			} else {
				d.respond(nil)
			}
			return nil
		}
	}

	// Обновляем input только если диалог видим
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return cmd
}

// View отрисовывает диалог поверх остальных компонентов.
func (d *InputDialog) View() string {
	d.mu.Lock()
	visible := d.Visible
	title := d.Title
	confirm := d.ConfirmText
	cancel := d.CancelText
	selected := d.selected
	d.mu.Unlock()

	if !visible {
		return ""
	}

	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	titleView := lipgloss.NewStyle().Bold(true).Render(title)
	inputView := d.input.View()

	// Стили для кнопок
	activeStyle := lipgloss.NewStyle().Background(lipgloss.Color("#7C3AED")).Foreground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Padding(0, 1)

	// Кнопки
	var cancelBtn, confirmBtn string
	if selected == 0 {
		cancelBtn = activeStyle.Render(cancel)
		confirmBtn = inactiveStyle.Render(confirm)
	} else {
		cancelBtn = inactiveStyle.Render(cancel)
		confirmBtn = activeStyle.Render(confirm)
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, cancelBtn, "  ", confirmBtn)

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).
		Render("Tab/←→: Select • Enter: Confirm • Esc: Cancel")

	return border.Render(fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", titleView, inputView, buttons, hint))
}

func (d *InputDialog) respond(value *string) {
	d.mu.Lock()
	if !d.Visible && d.result == nil {
		d.mu.Unlock()
		return
	}
	ch := d.result
	d.Visible = false
	d.result = nil
	d.input.Blur()
	d.mu.Unlock()

	if ch != nil {
		select {
		case ch <- value:
		default:
		}
	}
}
