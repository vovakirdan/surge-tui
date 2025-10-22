package screens

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (ps *ProjectScreenReal) beginInput(mode projectInputMode, placeholder string) {
	ps.inputMode = mode
	ps.input.Placeholder = placeholder
	if mode == projectInputRename {
		ps.input.SetValue(placeholder)
		ps.input.SetCursor(len(placeholder))
	} else {
		ps.input.SetValue("")
		ps.input.SetCursor(0)
	}
	ps.input.Focus()
}

func (ps *ProjectScreenReal) handleInputKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		value := strings.TrimSpace(ps.input.Value())
		if value == "" {
			ps.setStatus("Name cannot be empty")
			ps.cancelInput()
			return ps, nil
		}
		if err := ps.performInput(value); err != nil {
			ps.setStatus(err.Error())
		} else {
			ps.setStatus("Done")
		}
		ps.cancelInput()
		return ps, ps.loadFileTree()
	case tea.KeyEsc:
		ps.cancelInput()
		return ps, nil
	}

	var cmd tea.Cmd
	ps.input, cmd = ps.input.Update(msg)
	return ps, cmd
}

func (ps *ProjectScreenReal) cancelInput() {
	ps.inputMode = projectInputNone
	ps.input.Blur()
	ps.input.SetValue("")
}

func (ps *ProjectScreenReal) performInput(name string) error {
	switch ps.inputMode {
	case projectInputNewFile:
		return ps.createEntry(name, false)
	case projectInputNewDir:
		return ps.createEntry(name, true)
	case projectInputRename:
		return ps.renameEntry(name)
	default:
		return nil
	}
}

func (ps *ProjectScreenReal) selectedDirPath() string {
	if ps.fileTree == nil {
		return ps.projectPath
	}
	node := ps.fileTree.GetSelected()
	if node == nil {
		return ps.projectPath
	}
	if node.IsDir {
		return node.Path
	}
	if node.Parent != nil {
		return node.Parent.Path
	}
	return ps.projectPath
}

func (ps *ProjectScreenReal) createEntry(name string, dir bool) error {
	if strings.ContainsAny(name, "/\\") {
		return errors.New("name cannot contain separators")
	}
	targetDir := ps.selectedDirPath()
	path := filepath.Join(targetDir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", name)
	}
	if dir {
		return os.Mkdir(path, 0o755)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func (ps *ProjectScreenReal) renameEntry(name string) error {
	node := ps.fileTree.GetSelected()
	if node == nil {
		return errors.New("nothing selected")
	}
	if strings.ContainsAny(name, "/\\") {
		return errors.New("name cannot contain separators")
	}
	newPath := filepath.Join(filepath.Dir(node.Path), name)
	if _, err := os.Stat(newPath); err == nil && !strings.EqualFold(newPath, node.Path) {
		return fmt.Errorf("%s already exists", name)
	}
	return os.Rename(node.Path, newPath)
}

func (ps *ProjectScreenReal) performDelete(path string) error {
	return os.RemoveAll(path)
}

func (ps *ProjectScreenReal) setStatus(msg string) {
	ps.statusMsg = msg
	ps.statusAt = time.Now()
}

func (ps *ProjectScreenReal) statusLine() string {
	if ps.statusMsg == "" {
		return ""
	}
	if time.Since(ps.statusAt) > 3*time.Second {
		ps.statusMsg = ""
		return ""
	}
	return ps.statusMsg
}

func (ps *ProjectScreenReal) renderInputModal() string {
	prompt := ps.input.View()
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color(DimTextColor)).Render("Enter: confirm • Esc: cancel")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2).Render(fmt.Sprintf("%s\n%s", prompt, hint))
}

func joinOverlay(base, modal string) string {
	return lipgloss.JoinVertical(lipgloss.Left, base, modal)
}

type deleteConfirmedMsg struct {
	confirmed bool
	path      string
}
