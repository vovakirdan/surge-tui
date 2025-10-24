package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"surge-tui/internal/platform"
)

func (ps *ProjectScreenReal) activeEditorTab() *editorTab {
	if ps.activeTab < 0 || ps.activeTab >= len(ps.tabs) {
		return nil
	}
	return ps.tabs[ps.activeTab]
}

func (ps *ProjectScreenReal) findTabIndex(path string) int {
	if path == "" {
		return -1
	}
	abs := path
	if p, err := filepath.Abs(path); err == nil {
		abs = p
	}
	for i, tab := range ps.tabs {
		if tab.path == abs {
			return i
		}
	}
	return -1
}

func (ps *ProjectScreenReal) setActiveTab(index int) {
	if index < 0 || index >= len(ps.tabs) {
		return
	}
	ps.activeTab = index
	tab := ps.activeEditorTab()
	if tab != nil {
		tab.clampCursor()
		ps.ensureCursorVisible(tab)
	}
	ps.recalculateLayout()
}

func (ps *ProjectScreenReal) openFileTab(path string) *editorTab {
	if path == "" {
		return nil
	}

	if idx := ps.findTabIndex(path); idx >= 0 {
		ps.setActiveTab(idx)
		ps.focusedPanel = EditorPanel
		ps.recalculateLayout()
		return ps.activeEditorTab()
	}

	tab, err := newEditorTab(path)
	if err != nil {
		ps.setStatus(fmt.Sprintf("Failed to open file: %v", err))
		return nil
	}

	ps.tabs = append(ps.tabs, tab)
	ps.activeTab = len(ps.tabs) - 1
	ps.focusedPanel = EditorPanel
	ps.ensureCursorVisible(tab)
	ps.recalculateLayout()
	ps.setStatus("Opened " + tab.name)
	return tab
}

// OpenLocation открывает файл и позиционирует курсор на указанной строке и колонке.
func (ps *ProjectScreenReal) OpenLocation(path string, line, column int) {
	if path == "" {
		return
	}
	abs := path
	if !filepath.IsAbs(abs) && ps.projectPath != "" {
		abs = filepath.Join(ps.projectPath, path)
	}
	tab := ps.openFileTab(abs)
	if tab == nil {
		return
	}
	if line <= 0 {
		line = 1
	}
	if column <= 0 {
		column = 1
	}
	tab.setCursorPosition(line, column)
	tab.mode = editorModeNormal
	tab.clearPending()
	ps.ensureCursorVisible(tab)
	ps.focusedPanel = EditorPanel
	ps.recalculateLayout()
	ps.setStatus(fmt.Sprintf("Jumped to %s:%d:%d", filepath.Base(tab.path), line, column))
}

func (ps *ProjectScreenReal) activateAdjacentTab(offset int) {
	if len(ps.tabs) == 0 {
		return
	}
	index := ps.activeTab + offset
	if index < 0 {
		index = len(ps.tabs) - 1
	} else if index >= len(ps.tabs) {
		index = 0
	}
	ps.setActiveTab(index)
}

func (ps *ProjectScreenReal) reorderTabs(offset int) {
	if len(ps.tabs) < 2 || ps.activeTab < 0 {
		return
	}
	target := ps.activeTab + offset
	if target < 0 || target >= len(ps.tabs) {
		return
	}
	ps.tabs[ps.activeTab], ps.tabs[target] = ps.tabs[target], ps.tabs[ps.activeTab]
	ps.activeTab = target
}

func (ps *ProjectScreenReal) requestCloseActiveTab(force bool) tea.Cmd {
	tab := ps.activeEditorTab()
	if tab == nil {
		return nil
	}

	if !tab.dirty || force {
		index := ps.activeTab
		ps.forceCloseTab(index)
		return nil
	}

	if ps.closeDialog == nil {
		return nil
	}

	ps.closeDialog.Title = "Close Tab"
	ps.closeDialog.Description = fmt.Sprintf("Save changes before closing %s?", tab.name)
	ps.closeDialog.ConfirmText = "Close"
	ps.closeDialog.CancelText = "Cancel"

	index := ps.activeTab
	ch := ps.closeDialog.Show()

	return func() tea.Msg {
		confirmed := <-ch
		return closeTabConfirmedMsg{index: index, confirmed: confirmed}
	}
}

func (ps *ProjectScreenReal) forceCloseTab(index int) {
	if index < 0 || index >= len(ps.tabs) {
		return
	}

	tab := ps.tabs[index]
	ps.tabs = append(ps.tabs[:index], ps.tabs[index+1:]...)

	if len(ps.tabs) == 0 {
		ps.activeTab = -1
		ps.focusedPanel = FileTreePanel
		ps.recalculateLayout()
		ps.setStatus("Closed " + tab.name)
		return
	}

	if index >= len(ps.tabs) {
		index = len(ps.tabs) - 1
	}
	ps.activeTab = index
	ps.ensureCursorVisible(ps.activeEditorTab())
	ps.recalculateLayout()
	ps.setStatus("Closed " + tab.name)
}

func (ps *ProjectScreenReal) saveActiveTab() {
	tab := ps.activeEditorTab()
	if tab == nil {
		return
	}
	if err := tab.save(); err != nil {
		ps.setStatus(fmt.Sprintf("Save failed: %v", err))
		return
	}
	ps.setStatus("Saved " + tab.name)
}

func (ps *ProjectScreenReal) ensureCursorVisible(tab *editorTab) {
	if tab == nil {
		return
	}
	height := ps.editorContentHeight()
	if height < 1 {
		height = 1
	}
	if tab.cursor.Line < tab.scroll {
		tab.scroll = tab.cursor.Line
	}
	if tab.cursor.Line >= tab.scroll+height {
		tab.scroll = tab.cursor.Line - height + 1
	}
	if tab.scroll < 0 {
		tab.scroll = 0
	}
}

func (ps *ProjectScreenReal) handleEditorEscape() bool {
	tab := ps.activeEditorTab()
	if tab == nil {
		return false
	}

	switch tab.mode {
	case editorModeInsert:
		if tab.cursor.Col > 0 {
			tab.cursor.Col--
		}
		tab.mode = editorModeNormal
		tab.clearPending()
		ps.editorCommand.Blur()
		ps.setStatus("-- NORMAL --")
		return true
	case editorModeCommand:
		tab.mode = editorModeNormal
		ps.editorCommand.Blur()
		ps.setStatus("-- NORMAL --")
		return true
	default:
		if tab.pending != "" {
			tab.clearPending()
			return true
		}
	}

	return false
}

func (ps *ProjectScreenReal) editorContentHeight() int {
	height := ps.Height()
	if height < 1 {
		return 1
	}

	// Учитываем рамку панели
	height -= 2

	// Учитываем строку табов, если есть открытые вкладки
	if len(ps.tabs) > 0 {
		height--
	}

	// Учитываем статусную строку редактора
	height--

	// Дополнительно учитываем командную строку в командном режиме
	if tab := ps.activeEditorTab(); tab != nil && tab.mode == editorModeCommand {
		height--
	}

	if height < 1 {
		height = 1
	}
	return height
}

func (ps *ProjectScreenReal) editorContentWidth() int {
	width := ps.mainWidth - 8 // учёт бордера, паддинга и ширины номера строки
	if width < 8 {
		width = 8
	}
	return width
}

func (ps *ProjectScreenReal) handleEditorKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	if len(ps.tabs) == 0 {
		return ps, nil
	}

	key := platform.CanonicalKeyForLookup(msg.String())
	switch key {
	case "ctrl+left":
		ps.focusedPanel = FileTreePanel
		ps.recalculateLayout()
		return ps, nil
	case "ctrl+right":
		ps.focusedPanel = EditorPanel
		ps.recalculateLayout()
		return ps, nil
	case "alt+left", "alt+h", "ctrl+shift+tab":
		ps.activateAdjacentTab(-1)
		return ps, nil
	case "alt+right", "alt+l", "ctrl+tab":
		ps.activateAdjacentTab(1)
		return ps, nil
	case "alt+shift+left":
		ps.reorderTabs(-1)
		return ps, nil
	case "alt+shift+right":
		ps.reorderTabs(1)
		return ps, nil
	}

	tab := ps.activeEditorTab()
	if tab == nil {
		return ps, nil
	}

	switch tab.mode {
	case editorModeInsert:
		return ps.handleInsertModeKey(tab, msg)
	case editorModeCommand:
		return ps.handleCommandModeKey(tab, msg)
	default:
		return ps.handleNormalModeKey(tab, msg)
	}
}

func (ps *ProjectScreenReal) handleInsertModeKey(tab *editorTab, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		ps.handleEditorEscape()
		return ps, nil
	case tea.KeyEnter:
		tab.insertNewLine()
		ps.ensureCursorVisible(tab)
		return ps, nil
	case tea.KeyBackspace:
		tab.deleteBackward()
		ps.ensureCursorVisible(tab)
		return ps, nil
	case tea.KeyDelete:
		tab.deleteForward()
		ps.ensureCursorVisible(tab)
		return ps, nil
	case tea.KeySpace:
		tab.insertString(" ")
		ps.ensureCursorVisible(tab)
		return ps, nil
	case tea.KeyRunes:
		if msg.Alt {
			// Alt-modified runes are ignored in insert mode for now.
			return ps, nil
		}
		tab.insertRunes(msg.Runes)
		ps.ensureCursorVisible(tab)
		return ps, nil
	}

	key := platform.CanonicalKeyForLookup(msg.String())
	switch key {
	case "ctrl+s":
		ps.saveActiveTab()
	case "tab":
		tab.insertString("\t")
		ps.ensureCursorVisible(tab)
	case "space":
		tab.insertString(" ")
		ps.ensureCursorVisible(tab)
	case "backspace", "ctrl+h":
		tab.deleteBackward()
		ps.ensureCursorVisible(tab)
	case "left":
		tab.moveCursor(0, -1)
		ps.ensureCursorVisible(tab)
	case "right":
		tab.moveCursor(0, 1)
		ps.ensureCursorVisible(tab)
	case "up":
		tab.moveCursor(-1, 0)
		ps.ensureCursorVisible(tab)
	case "down":
		tab.moveCursor(1, 0)
		ps.ensureCursorVisible(tab)
	case "ctrl+w":
		return ps, ps.requestCloseActiveTab(false)
	case "home":
		tab.moveToStartOfLine()
	case "end":
		tab.moveToEndOfLine()
	default:
		return ps, nil
	}

	return ps, nil
}

func (ps *ProjectScreenReal) handleCommandModeKey(tab *editorTab, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		ps.handleEditorEscape()
		return ps, nil
	case tea.KeyEnter:
		command := strings.TrimSpace(ps.editorCommand.Value())
		ps.editorCommand.SetValue("")
		ps.executeEditorCommand(tab, command)
		return ps, nil
	}

	var cmd tea.Cmd
	ps.editorCommand, cmd = ps.editorCommand.Update(msg)
	return ps, cmd
}

func (ps *ProjectScreenReal) handleNormalModeKey(tab *editorTab, msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := platform.CanonicalKeyForLookup(msg.String())

	switch {
	case tab.hasPending("y"):
		tab.clearPending()
		if key == "y" {
			ps.copyLine()
			return ps, nil
		}
	case tab.hasPending("d"):
		tab.clearPending()
		if key == "d" {
			ps.cutLine()
			return ps, nil
		}
	case tab.hasPending("g"):
		tab.clearPending()
		if key == "g" {
			tab.cursor.Line = 0
			tab.cursor.Col = 0
			ps.ensureCursorVisible(tab)
			return ps, nil
		}
	}

	switch key {
	case "i":
		tab.mode = editorModeInsert
		ps.setStatus("-- INSERT --")
	case "a":
		tab.moveCursor(0, 1)
		tab.mode = editorModeInsert
		ps.ensureCursorVisible(tab)
		ps.setStatus("-- INSERT --")
	case "o":
		tab.moveToEndOfLine()
		tab.insertNewLine()
		tab.mode = editorModeInsert
		ps.ensureCursorVisible(tab)
		ps.setStatus("-- INSERT --")
	case "O":
		tab.cursor.Col = 0
		tab.insertNewLine()
		tab.cursor.Line--
		if tab.cursor.Line < 0 {
			tab.cursor.Line = 0
		}
		tab.mode = editorModeInsert
		ps.ensureCursorVisible(tab)
		ps.setStatus("-- INSERT --")
	case ":":
		tab.mode = editorModeCommand
		ps.editorCommand.SetValue("")
		ps.editorCommand.SetCursor(len(ps.editorCommand.Value()))
		ps.editorCommand.Focus()
		ps.setStatus("-- COMMAND --")
	case "h", "left":
		tab.moveCursor(0, -1)
		ps.ensureCursorVisible(tab)
	case "l", "right":
		tab.moveCursor(0, 1)
		ps.ensureCursorVisible(tab)
	case "j", "down":
		tab.moveCursor(1, 0)
		ps.ensureCursorVisible(tab)
	case "k", "up":
		tab.moveCursor(-1, 0)
		ps.ensureCursorVisible(tab)
	case "0", "home":
		tab.moveToStartOfLine()
	case "$", "end":
		tab.moveToEndOfLine()
	case "G":
		tab.cursor.Line = tab.lineCount() - 1
		tab.moveToEndOfLine()
		ps.ensureCursorVisible(tab)
	case "g":
		tab.setPending("g")
	case "y":
		tab.setPending("y")
	case "d":
		tab.setPending("d")
	case "p":
		ps.pasteLine()
	case "x":
		tab.deleteForward()
		ps.ensureCursorVisible(tab)
	case "ctrl+s":
		ps.saveActiveTab()
	case "ctrl+w":
		return ps, ps.requestCloseActiveTab(false)
	case "ctrl+q":
		return ps, ps.requestCloseActiveTab(false)
	default:
		tab.clearPending()
	}

	return ps, nil
}

func (ps *ProjectScreenReal) executeEditorCommand(tab *editorTab, input string) {
	tab.mode = editorModeNormal
	ps.editorCommand.Blur()

	if input == "" {
		ps.setStatus("-- NORMAL --")
		return
	}

	force := false
	if strings.HasSuffix(input, "!") {
		force = true
		input = strings.TrimSuffix(input, "!")
	}

	switch input {
	case "w", "write":
		ps.saveActiveTab()
	case "q", "quit":
		if tab.dirty && !force {
			ps.setStatus("Unsaved changes (use :q!)")
			return
		}
		ps.forceCloseTab(ps.activeTab)
	case "wq", "x", "xit":
		if err := tab.save(); err != nil {
			ps.setStatus(fmt.Sprintf("Save failed: %v", err))
			return
		}
		ps.forceCloseTab(ps.activeTab)
	default:
		ps.setStatus("Unknown command: " + input)
	}
}

func (ps *ProjectScreenReal) copyLine() {
	tab := ps.activeEditorTab()
	if tab == nil {
		return
	}
	ps.yankBuffer = tab.copyLine()
	ps.setStatus("Line yanked")
}

func (ps *ProjectScreenReal) cutLine() {
	tab := ps.activeEditorTab()
	if tab == nil {
		return
	}
	ps.yankBuffer = tab.deleteLine()
	ps.ensureCursorVisible(tab)
	ps.setStatus("Line cut")
}

func (ps *ProjectScreenReal) pasteLine() {
	tab := ps.activeEditorTab()
	if tab == nil || ps.yankBuffer == "" {
		return
	}
	tab.pasteLine(ps.yankBuffer)
	ps.ensureCursorVisible(tab)
	ps.setStatus("Line pasted")
}
