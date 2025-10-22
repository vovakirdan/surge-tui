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

	Visible bool
	result  chan bool
	mu      sync.Mutex
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
		case "y", "enter":
			d.respond(true)
		case "n", "escape":
			d.respond(false)
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
	d.mu.Unlock()

	if !visible {
		return ""
	}
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	titleView := lipgloss.NewStyle().Bold(true).Render(title)
	descView := lipgloss.NewStyle().Render(desc)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).
		Render(fmt.Sprintf("%s: %s  %s: %s", confirm, "Enter", cancel, "Esc"))
	return border.Render(fmt.Sprintf("%s\n\n%s\n\n%s", titleView, descView, hint))
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
