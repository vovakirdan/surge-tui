package screens

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type editorMode int

const (
	editorModeNormal editorMode = iota
	editorModeInsert
	editorModeCommand
)

type cursorPosition struct {
	Line int
	Col  int
}

type editorTab struct {
	path      string
	name      string
	lines     []string
	cursor    cursorPosition
	scroll    int
	mode      editorMode
	pending   string
	dirty     bool
	created   bool
	lastSaved int64
}

func newEditorTab(path string) (*editorTab, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path // fallback to original path
	}

	lines := []string{""}
	created := false

	if data, err := os.ReadFile(abs); err == nil {
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines = strings.Split(text, "\n")
		if len(lines) == 0 {
			lines = []string{""}
		}
	} else {
		if !os.IsNotExist(err) {
			return nil, err
		}
		created = true
	}

	tab := &editorTab{
		path:    abs,
		name:    filepath.Base(abs),
		lines:   lines,
		cursor:  cursorPosition{Line: 0, Col: 0},
		mode:    editorModeNormal,
		pending: "",
		dirty:   created,
		created: created,
	}
	tab.clampCursor()
	return tab, nil
}

func (t *editorTab) lineCount() int {
	return len(t.lines)
}

func (t *editorTab) clampCursor() {
	if t.cursor.Line < 0 {
		t.cursor.Line = 0
	}
	if t.cursor.Line >= len(t.lines) {
		t.cursor.Line = len(t.lines) - 1
		if t.cursor.Line < 0 {
			t.cursor.Line = 0
		}
	}
	maxCol := utf8.RuneCountInString(t.lines[t.cursor.Line])
	if t.cursor.Col < 0 {
		t.cursor.Col = 0
	} else if t.cursor.Col > maxCol {
		t.cursor.Col = maxCol
	}
}

func (t *editorTab) moveCursor(lineDelta, colDelta int) {
	t.cursor.Line += lineDelta
	if t.cursor.Line < 0 {
		t.cursor.Line = 0
	} else if t.cursor.Line >= len(t.lines) {
		t.cursor.Line = len(t.lines) - 1
	}

	t.cursor.Col += colDelta
	if t.cursor.Col < 0 {
		t.cursor.Col = 0
	}
	maxCol := utf8.RuneCountInString(t.lines[t.cursor.Line])
	if t.cursor.Col > maxCol {
		t.cursor.Col = maxCol
	}
}

func (t *editorTab) moveToStartOfLine() {
	t.cursor.Col = 0
}

func (t *editorTab) moveToEndOfLine() {
	t.cursor.Col = utf8.RuneCountInString(t.lines[t.cursor.Line])
}

func (t *editorTab) insertRunes(rs []rune) {
	if len(rs) == 0 {
		return
	}
	lineRunes := []rune(t.lines[t.cursor.Line])
	col := t.cursor.Col
	if col > len(lineRunes) {
		col = len(lineRunes)
	}
	newRunes := append(lineRunes[:col], append(rs, lineRunes[col:]...)...)
	t.lines[t.cursor.Line] = string(newRunes)
	t.cursor.Col += len(rs)
	t.dirty = true
}

func (t *editorTab) insertString(text string) {
	t.insertRunes([]rune(text))
}

func (t *editorTab) insertNewLine() {
	lineRunes := []rune(t.lines[t.cursor.Line])
	col := t.cursor.Col
	if col > len(lineRunes) {
		col = len(lineRunes)
	}

	left := string(lineRunes[:col])
	right := string(lineRunes[col:])

	t.lines[t.cursor.Line] = left

	if t.cursor.Line == len(t.lines)-1 {
		t.lines = append(t.lines, right)
	} else {
		t.lines = append(t.lines[:t.cursor.Line+1], append([]string{right}, t.lines[t.cursor.Line+1:]...)...)
	}

	t.cursor.Line++
	t.cursor.Col = 0
	t.dirty = true
}

func (t *editorTab) deleteBackward() {
	if t.cursor.Col > 0 {
		lineRunes := []rune(t.lines[t.cursor.Line])
		col := t.cursor.Col
		if col > len(lineRunes) {
			col = len(lineRunes)
		}
		newRunes := append(lineRunes[:col-1], lineRunes[col:]...)
		t.lines[t.cursor.Line] = string(newRunes)
		t.cursor.Col--
		t.dirty = true
		return
	}
	if t.cursor.Line == 0 {
		return
	}

	prevLine := []rune(t.lines[t.cursor.Line-1])
	current := t.lines[t.cursor.Line]

	t.cursor.Col = len(prevLine)
	t.lines[t.cursor.Line-1] = string(append(prevLine, []rune(current)...))
	t.lines = append(t.lines[:t.cursor.Line], t.lines[t.cursor.Line+1:]...)
	t.cursor.Line--
	if len(t.lines) == 0 {
		t.lines = []string{""}
		t.cursor.Line = 0
		t.cursor.Col = 0
	}
	t.dirty = true
}

func (t *editorTab) deleteForward() {
	lineRunes := []rune(t.lines[t.cursor.Line])
	col := t.cursor.Col
	if col < len(lineRunes) {
		newRunes := append(lineRunes[:col], lineRunes[col+1:]...)
		t.lines[t.cursor.Line] = string(newRunes)
		t.dirty = true
		return
	}

	if t.cursor.Line >= len(t.lines)-1 {
		return
	}

	next := t.lines[t.cursor.Line+1]
	t.lines[t.cursor.Line] = t.lines[t.cursor.Line] + next
	t.lines = append(t.lines[:t.cursor.Line+1], t.lines[t.cursor.Line+2:]...)
	if len(t.lines) == 0 {
		t.lines = []string{""}
		t.cursor.Line = 0
		t.cursor.Col = 0
	}
	t.dirty = true
}

func (t *editorTab) deleteLine() string {
	if len(t.lines) == 0 {
		t.lines = []string{""}
		t.cursor.Line = 0
		t.cursor.Col = 0
		return ""
	}

	line := t.lines[t.cursor.Line]
	if len(t.lines) == 1 {
		t.lines[0] = ""
		t.cursor.Col = 0
	} else {
		t.lines = append(t.lines[:t.cursor.Line], t.lines[t.cursor.Line+1:]...)
		if t.cursor.Line >= len(t.lines) {
			t.cursor.Line = len(t.lines) - 1
		}
	}
	t.clampCursor()
	t.dirty = true
	return line
}

func (t *editorTab) copyLine() string {
	if len(t.lines) == 0 {
		return ""
	}
	return t.lines[t.cursor.Line]
}

func (t *editorTab) pasteLine(content string) {
	insertIndex := t.cursor.Line + 1
	if insertIndex >= len(t.lines) {
		t.lines = append(t.lines, content)
		t.cursor.Line = len(t.lines) - 1
	} else {
		t.lines = append(t.lines[:insertIndex], append([]string{content}, t.lines[insertIndex:]...)...)
		t.cursor.Line = insertIndex
	}
	t.cursor.Col = 0
	t.dirty = true
}

func (t *editorTab) save() error {
	perm := os.FileMode(0o644)
	if info, err := os.Stat(t.path); err == nil {
		perm = info.Mode()
	}
	content := strings.Join(t.lines, "\n")
	if err := os.WriteFile(t.path, []byte(content), perm); err != nil {
		return err
	}
	t.dirty = false
	return nil
}

func (t *editorTab) setPending(cmd string) {
	t.pending = cmd
}

func (t *editorTab) clearPending() {
	t.pending = ""
}

func (t *editorTab) hasPending(cmd string) bool {
	return t.pending == cmd
}
