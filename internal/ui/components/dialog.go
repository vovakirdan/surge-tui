package components

import (
	"fmt"

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
}

// NewConfirmDialog создает диалог с дефолтными кнопками.
func NewConfirmDialog(title, description string) *ConfirmDialog {
	return &ConfirmDialog{
		Title:       title,
		Description: description,
		ConfirmText: "Yes",
		CancelText:  "No",
		result:      make(chan bool, 1),
	}
}

// Show делает диалог видимым и возвращает канал результата.
func (d *ConfirmDialog) Show() <-chan bool {
	d.Visible = true
	d.result = make(chan bool, 1)
	return d.result
}

// Hide скрывает диалог без результата.
func (d *ConfirmDialog) Hide() {
	d.Visible = false
	select {
	case d.result <- false:
	default:
	}
}

// Update обрабатывает нажатия.
func (d *ConfirmDialog) Update(msg tea.Msg) tea.Cmd {
	if !d.Visible {
		return nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "enter":
			d.Visible = false
			select {
			case d.result <- true:
			default:
			}
		case "n", "escape":
			d.Visible = false
			select {
			case d.result <- false:
			default:
			}
		}
	}
	return nil
}

// View отрисовывает диалог поверх остальных компонентов.
func (d *ConfirmDialog) View() string {
	if !d.Visible {
		return ""
	}
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
	title := lipgloss.NewStyle().Bold(true).Render(d.Title)
	desc := lipgloss.NewStyle().Render(d.Description)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).
		Render(fmt.Sprintf("%s: %s  %s: %s", d.ConfirmText, "Enter", d.CancelText, "Esc"))
	return border.Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, desc, hint))
}
