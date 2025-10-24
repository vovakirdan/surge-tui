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

	"surge-tui/internal/fs"
)

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

func (ps *ProjectScreenReal) ShowNewFileDialog() tea.Cmd {
	if ps.newFileDialog == nil {
		return nil
	}
	ch := ps.newFileDialog.Show()
	return func() tea.Msg {
		value := <-ch
		return newFileConfirmedMsg{value: value}
	}
}

func (ps *ProjectScreenReal) ShowNewDirectoryDialog() tea.Cmd {
	if ps.newDirDialog == nil {
		return nil
	}
	ch := ps.newDirDialog.Show()
	return func() tea.Msg {
		value := <-ch
		return newDirConfirmedMsg{value: value}
	}
}

func (ps *ProjectScreenReal) ShowRenameDialog() tea.Cmd {
	if ps.renameDialog == nil {
		return nil
	}
	name := ""
	if ps.fileTree != nil {
		if node := ps.fileTree.GetSelected(); node != nil {
			name = node.Name
		}
	}
	ch := ps.renameDialog.ShowWithValue(name)
	return func() tea.Msg {
		value := <-ch
		return renameConfirmedMsg{value: value}
	}
}

func (ps *ProjectScreenReal) RequestDeleteSelected() tea.Cmd {
	if ps.fileTree == nil || ps.confirm == nil {
		return nil
	}
	node := ps.fileTree.GetSelected()
	if node == nil {
		return nil
	}
	ps.confirm.Description = fmt.Sprintf("Delete %s?", node.Name)
	ch := ps.confirm.Show()
	path := node.Path
	return func() tea.Msg {
		confirmed := <-ch
		return deleteConfirmedMsg{confirmed: confirmed, path: path}
	}
}

func (ps *ProjectScreenReal) ToggleHiddenEntries() {
	if ps.fileTree == nil {
		return
	}
	ps.fileTree.SetShowHidden(!ps.fileTree.ShowHidden)
	ps.updateStats()
	if ps.fileTree.ShowHidden {
		ps.setStatus("Hidden entries visible")
	} else {
		ps.setStatus("Hidden entries hidden")
	}
}

func (ps *ProjectScreenReal) ToggleSurgeFilter() {
	if ps.fileTree == nil {
		return
	}
	ps.fileTree.SetFilterSurge(!ps.fileTree.FilterSurge)
	ps.updateStats()
	if ps.fileTree.FilterSurge {
		ps.setStatus("Filter: .sg only")
	} else {
		ps.setStatus("Filter: all files")
	}
}

func (ps *ProjectScreenReal) RefreshFileTree() tea.Cmd {
	return ps.loadFileTree()
}

func (ps *ProjectScreenReal) FocusEditorPanel() {
	if len(ps.tabs) == 0 {
		return
	}
	ps.focusedPanel = EditorPanel
	ps.recalculateLayout()
}

func (ps *ProjectScreenReal) FocusFileTree() {
	ps.focusedPanel = FileTreePanel
	ps.recalculateLayout()
}

func (ps *ProjectScreenReal) OpenSelectedEntryCmd() tea.Cmd {
	return ps.openSelectedEntry()
}

func (ps *ProjectScreenReal) ToggleSelectedDirectory() {
	if ps.fileTree == nil {
		return
	}
	ps.fileTree.ToggleExpanded(ps.fileTree.Selected)
	ps.updateStats()
}

func (ps *ProjectScreenReal) ActivateNextTab() {
	ps.activateAdjacentTab(1)
}

func (ps *ProjectScreenReal) ActivatePreviousTab() {
	ps.activateAdjacentTab(-1)
}

func (ps *ProjectScreenReal) ReorderTabLeft() {
	ps.reorderTabs(-1)
}

func (ps *ProjectScreenReal) ReorderTabRight() {
	ps.reorderTabs(1)
}

func (ps *ProjectScreenReal) CloseActiveTabCmd(force bool) tea.Cmd {
	return ps.requestCloseActiveTab(force)
}

func (ps *ProjectScreenReal) SaveActiveTabCmd() tea.Cmd {
	ps.saveActiveTab()
	return nil
}

func (ps *ProjectScreenReal) RunEditorCommand(command string) tea.Cmd {
	tab := ps.activeEditorTab()
	if tab == nil {
		return nil
	}
	ps.executeEditorCommand(tab, command)
	return nil
}

func (ps *ProjectScreenReal) HasOpenTab() bool {
	return len(ps.tabs) > 0
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

func joinOverlay(base, modal string) string {
	return lipgloss.JoinVertical(lipgloss.Left, base, modal)
}

type deleteConfirmedMsg struct {
	confirmed bool
	path      string
}

func (ps *ProjectScreenReal) isProjectDirectory(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(path, "surge.toml"))
	return err == nil && !info.IsDir()
}

func (ps *ProjectScreenReal) selectedDirectoryNode() *fs.FileNode {
	if ps.fileTree == nil {
		return nil
	}
	node := ps.fileTree.GetSelected()
	if node != nil && node.IsDir {
		return node
	}
	return nil
}

func (ps *ProjectScreenReal) CanInitProject() bool {
	node := ps.selectedDirectoryNode()
	if node == nil {
		return false
	}
	return !ps.isProjectDirectory(node.Path)
}

func (ps *ProjectScreenReal) InitProjectInSelectedDir() tea.Cmd {
	node := ps.selectedDirectoryNode()
	if node == nil {
		ps.setStatus("Select directory to init")
		return nil
	}
	if ps.isProjectDirectory(node.Path) {
		ps.setStatus("Already a Surge project")
		return nil
	}
	ps.setStatus("Init project action (TODO)")
	return nil
}

func (ps *ProjectScreenReal) HandleGlobalEsc() (bool, tea.Cmd) {
	if ps.confirm != nil && ps.confirm.Visible {
		ps.confirm.Hide()
		return true, nil
	}
	if ps.closeDialog != nil && ps.closeDialog.Visible {
		ps.closeDialog.Hide()
		return true, nil
	}
	if ps.newFileDialog != nil && ps.newFileDialog.Visible {
		ps.newFileDialog.Hide()
		return true, nil
	}
	if ps.newDirDialog != nil && ps.newDirDialog.Visible {
		ps.newDirDialog.Hide()
		return true, nil
	}
	if ps.renameDialog != nil && ps.renameDialog.Visible {
		ps.renameDialog.Hide()
		return true, nil
	}

	if ps.focusedPanel == EditorPanel {
		if ps.handleEditorEscape() {
			return true, nil
		}
		return true, nil
	}

	return true, nil
}
