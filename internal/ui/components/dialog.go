package components

import (
	"fmt"
	"sync"

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
