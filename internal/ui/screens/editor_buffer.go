package screens

import (
	"strings"
	"unicode/utf8"
)

type textPos struct {
	line int
	col  int
}

type textBuffer struct {
	lines  [][]rune
	cursor textPos
	anchor *textPos
}

func newTextBuffer() *textBuffer {
	return &textBuffer{lines: [][]rune{{}}}
}

func newTextBufferFromString(content string) *textBuffer {
	buf := newTextBuffer()
	if content == "" {
		return buf
	}
	parts := strings.Split(content, "\n")
	buf.lines = make([][]rune, len(parts))
	for i, p := range parts {
		buf.lines[i] = []rune(p)
	}
	if strings.HasSuffix(content, "\n") {
		buf.lines = append(buf.lines, []rune{})
	}
	return buf
}

func (b *textBuffer) lineCount() int {
	return len(b.lines)
}

func (b *textBuffer) lineLength(line int) int {
	if line < 0 || line >= len(b.lines) {
		return 0
	}
	return len(b.lines[line])
}

func (b *textBuffer) normalizeCursor() {
	if b.cursor.line < 0 {
		b.cursor.line = 0
	}
	if b.cursor.line >= len(b.lines) {
		b.cursor.line = len(b.lines) - 1
	}
	lineLen := b.lineLength(b.cursor.line)
	if b.cursor.col < 0 {
		b.cursor.col = 0
	}
	if b.cursor.col > lineLen {
		b.cursor.col = lineLen
	}
}

func (b *textBuffer) moveCursorTo(line, col int) {
	b.cursor.line = line
	b.cursor.col = col
	b.normalizeCursor()
}

func (b *textBuffer) moveCursorLeft() {
	if b.cursor.col > 0 {
		b.cursor.col--
		return
	}
	if b.cursor.line > 0 {
		b.cursor.line--
		b.cursor.col = len(b.lines[b.cursor.line])
	}
}

func (b *textBuffer) moveCursorRight() {
	if b.cursor.col < len(b.lines[b.cursor.line]) {
		b.cursor.col++
		return
	}
	if b.cursor.line < len(b.lines)-1 {
		b.cursor.line++
		b.cursor.col = 0
	}
}

func (b *textBuffer) moveCursorUp() {
	if b.cursor.line == 0 {
		b.cursor.col = min(b.cursor.col, len(b.lines[0]))
		return
	}
	b.cursor.line--
	b.cursor.col = min(b.cursor.col, len(b.lines[b.cursor.line]))
}

func (b *textBuffer) moveCursorDown() {
	if b.cursor.line >= len(b.lines)-1 {
		b.cursor.col = min(b.cursor.col, len(b.lines[len(b.lines)-1]))
		return
	}
	b.cursor.line++
	b.cursor.col = min(b.cursor.col, len(b.lines[b.cursor.line]))
}

func (b *textBuffer) setAnchor() {
	pos := b.cursor
	b.anchor = &pos
}

func (b *textBuffer) clearAnchor() {
	b.anchor = nil
}

func (b *textBuffer) hasSelection() bool {
	if b.anchor == nil {
		return false
	}
	return b.anchor.line != b.cursor.line || b.anchor.col != b.cursor.col
}

func (b *textBuffer) selectionRange() (textPos, textPos) {
	if !b.hasSelection() {
		return b.cursor, b.cursor
	}
	start := *b.anchor
	end := b.cursor
	if start.line > end.line || (start.line == end.line && start.col > end.col) {
		start, end = end, start
	}
	return start, end
}

func (b *textBuffer) selectedText() string {
	if !b.hasSelection() {
		return ""
	}
	start, end := b.selectionRange()
	if start.line == end.line {
		return string(b.lines[start.line][start.col:end.col])
	}
	var builder strings.Builder
	builder.WriteString(string(b.lines[start.line][start.col:]))
	builder.WriteRune('\n')
	for line := start.line + 1; line < end.line; line++ {
		builder.WriteString(string(b.lines[line]))
		builder.WriteRune('\n')
	}
	builder.WriteString(string(b.lines[end.line][:end.col]))
	return builder.String()
}

func (b *textBuffer) deleteSelection() string {
	if !b.hasSelection() {
		return ""
	}
	start, end := b.selectionRange()
	removed := b.selectedText()

	if start.line == end.line {
		line := append([]rune{}, b.lines[start.line]...)
		line = append(line[:start.col], line[end.col:]...)
		b.lines[start.line] = line
	} else {
		head := append([]rune{}, b.lines[start.line][:start.col]...)
		tail := append([]rune{}, b.lines[end.line][end.col:]...)
		merged := append(head, tail...)
		b.lines[start.line] = merged
		b.lines = append(b.lines[:start.line+1], b.lines[end.line+1:]...)
	}

	b.cursor = start
	b.clearAnchor()
	b.normalizeCursor()
	return removed
}

func (b *textBuffer) insertRune(r rune) {
	if b.hasSelection() {
		b.deleteSelection()
	}
	if r == '\n' {
		current := b.lines[b.cursor.line]
		before := append([]rune{}, current[:b.cursor.col]...)
		after := append([]rune{}, current[b.cursor.col:]...)
		b.lines[b.cursor.line] = before
		lineIndex := b.cursor.line + 1
		b.lines = append(b.lines, nil)
		copy(b.lines[lineIndex+1:], b.lines[lineIndex:])
		b.lines[lineIndex] = after
		b.cursor.line++
		b.cursor.col = 0
		return
	}
	line := append([]rune{}, b.lines[b.cursor.line]...)
	line = append(line[:b.cursor.col], append([]rune{r}, line[b.cursor.col:]...)...)
	b.lines[b.cursor.line] = line
	b.cursor.col++
}

func (b *textBuffer) insertString(str string) {
	if str == "" {
		return
	}
	for len(str) > 0 {
		r, size := utf8.DecodeRuneInString(str)
		if r == utf8.RuneError && size == 1 {
			// skip invalid rune
			str = str[size:]
			continue
		}
		b.insertRune(r)
		str = str[size:]
	}
}

func (b *textBuffer) deleteBackward() string {
	if b.hasSelection() {
		return b.deleteSelection()
	}
	if b.cursor.col > 0 {
		line := append([]rune{}, b.lines[b.cursor.line]...)
		removed := string(line[b.cursor.col-1 : b.cursor.col])
		line = append(line[:b.cursor.col-1], line[b.cursor.col:]...)
		b.lines[b.cursor.line] = line
		b.cursor.col--
		return removed
	}
	if b.cursor.line == 0 {
		return ""
	}
	prevLine := append([]rune{}, b.lines[b.cursor.line-1]...)
	current := b.lines[b.cursor.line]
	removed := "\n"
	b.cursor.col = len(prevLine)
	prevLine = append(prevLine, current...)
	b.lines[b.cursor.line-1] = prevLine
	b.lines = append(b.lines[:b.cursor.line], b.lines[b.cursor.line+1:]...)
	b.cursor.line--
	return removed
}

func (b *textBuffer) deleteForward() string {
	if b.hasSelection() {
		return b.deleteSelection()
	}
	line := b.lines[b.cursor.line]
	if b.cursor.col < len(line) {
		removed := string(line[b.cursor.col : b.cursor.col+1])
		line = append(line[:b.cursor.col], line[b.cursor.col+1:]...)
		b.lines[b.cursor.line] = line
		return removed
	}
	if b.cursor.line >= len(b.lines)-1 {
		return ""
	}
	next := b.lines[b.cursor.line+1]
	removed := "\n"
	merged := append([]rune{}, line...)
	merged = append(merged, next...)
	b.lines[b.cursor.line] = merged
	b.lines = append(b.lines[:b.cursor.line+1], b.lines[b.cursor.line+2:]...)
	return removed
}

func (b *textBuffer) cloneLines() [][]rune {
	copyLines := make([][]rune, len(b.lines))
	for i, line := range b.lines {
		copyLines[i] = append([]rune{}, line...)
	}
	return copyLines
}

func (b *textBuffer) fullText() string {
	var builder strings.Builder
	for i, line := range b.lines {
		builder.WriteString(string(line))
		if i < len(b.lines)-1 {
			builder.WriteRune('\n')
		}
	}
	return builder.String()
}

func (b *textBuffer) replaceAll(content string) {
	b.lines = [][]rune{{}}
	b.cursor = textPos{}
	b.anchor = nil
	if content == "" {
		return
	}
	parts := strings.Split(content, "\n")
	b.lines = make([][]rune, len(parts))
	for i, p := range parts {
		b.lines[i] = []rune(p)
	}
	if strings.HasSuffix(content, "\n") {
		b.lines = append(b.lines, []rune{})
	}
	b.normalizeCursor()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
