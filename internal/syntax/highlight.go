package syntax

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// TokenKind represents a syntax classification used for highlighting.
type TokenKind int

const (
	TokenPlain TokenKind = iota
	TokenKeyword
	TokenBuiltinType
	TokenTypeName
	TokenString
	TokenNumber
	TokenComment
	TokenDirective
	TokenAttribute
	TokenOperator
)

// Segment describes a classified piece of text.
type Segment struct {
	Kind TokenKind
	Text string
}

// Line stores the highlight result for a single line.
type Line struct {
	Segments []Segment
	length   int
}

func (l *Line) append(kind TokenKind, text string) {
	if len(text) == 0 {
		return
	}
	if n := len(l.Segments); n > 0 && l.Segments[n-1].Kind == kind {
		l.Segments[n-1].Text += text
	} else {
		l.Segments = append(l.Segments, Segment{Kind: kind, Text: text})
	}
	l.length += utf8.RuneCountInString(text)
}

// Length returns the visible rune length of the line.
func (l Line) Length() int {
	return l.length
}

// HighlightDocument highlights the provided lines, preserving cross-line state
// (block comments, multi-line strings).
func HighlightDocument(lines []string) []Line {
	result := make([]Line, len(lines))
	var state lexState
	for i, line := range lines {
		result[i], state = highlightLine(line, state)
	}
	return result
}

// PlainLine creates a Line consisting entirely of plain text.
func PlainLine(text string) Line {
	var line Line
	line.append(TokenPlain, text)
	return line
}

// WindowLine returns a slice of the provided line starting at the given column
// and constrained to the specified visual width. Ellipses are inserted when
// content is truncated on either side.
func WindowLine(line Line, start, width int) Line {
	var out Line
	if width <= 0 {
		return out
	}

	total := line.Length()
	if total == 0 {
		return out
	}

	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}

	remaining := width

	leftIndicator := start > 0 && remaining > 0
	if leftIndicator {
		out.append(TokenPlain, "…")
		remaining--
	}

	contentWidth := remaining
	if contentWidth < 0 {
		contentWidth = 0
	}

	rightIndicator := start+contentWidth < total
	reservedRight := false
	if rightIndicator && remaining > 0 {
		reservedRight = true
		remaining--
		if contentWidth > 0 {
			contentWidth--
		}
	}

	skip := start
	copyLeft := contentWidth

	for _, seg := range line.Segments {
		if copyLeft <= 0 {
			break
		}
		runes := []rune(seg.Text)
		segLen := len(runes)
		if segLen == 0 {
			continue
		}
		idx := 0
		if skip > 0 {
			if skip >= segLen {
				skip -= segLen
				continue
			}
			idx = skip
			skip = 0
		}
		if idx >= segLen {
			continue
		}
		take := segLen - idx
		if take > copyLeft {
			take = copyLeft
		}
		if take > 0 {
			out.append(seg.Kind, string(runes[idx:idx+take]))
			copyLeft -= take
		}
	}

	if rightIndicator {
		if reservedRight {
			out.append(TokenPlain, "…")
		} else if removeLastRune(&out) {
			out.append(TokenPlain, "…")
		} else if !leftIndicator {
			out.append(TokenPlain, "…")
		} else {
			out.append(TokenPlain, "…")
		}
	}

	return out
}

func removeLastRune(line *Line) bool {
	if line == nil {
		return false
	}
	for len(line.Segments) > 0 {
		idx := len(line.Segments) - 1
		seg := &line.Segments[idx]
		runes := []rune(seg.Text)
		if len(runes) == 0 {
			line.Segments = line.Segments[:idx]
			continue
		}
		runes = runes[:len(runes)-1]
		seg.Text = string(runes)
		line.length--
		if len(runes) == 0 {
			line.Segments = line.Segments[:idx]
		}
		return true
	}
	return false
}

type lexState struct {
	blockDepth int
	inString   bool
}

func highlightLine(line string, state lexState) (Line, lexState) {
	var out Line
	runes := []rune(line)
	i := 0
	for i < len(runes) {
		if state.blockDepth > 0 {
			start := i
			for i < len(runes) {
				if runes[i] == '/' && i+1 < len(runes) && runes[i+1] == '*' {
					state.blockDepth++
					i += 2
					continue
				}
				if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					state.blockDepth--
					i += 2
					if state.blockDepth == 0 {
						break
					}
					continue
				}
				i++
			}
			out.append(TokenComment, string(runes[start:i]))
			continue
		}

		if state.inString {
			start := i
			for i < len(runes) {
				if runes[i] == '\\' && i+1 < len(runes) {
					i += 2
					continue
				}
				if runes[i] == '"' {
					i++
					state.inString = false
					break
				}
				i++
			}
			if state.inString {
				i = len(runes)
			}
			out.append(TokenString, string(runes[start:i]))
			continue
		}

		if i+2 < len(runes) && runes[i] == '/' && runes[i+1] == '/' && runes[i+2] == '/' {
			out.append(TokenDirective, string(runes[i:]))
			break
		}

		if i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '/' {
			out.append(TokenComment, string(runes[i:]))
			break
		}

		if i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '*' {
			start := i
			i += 2
			state.blockDepth++
			for i < len(runes) && state.blockDepth > 0 {
				if runes[i] == '/' && i+1 < len(runes) && runes[i+1] == '*' {
					state.blockDepth++
					i += 2
					continue
				}
				if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '/' {
					state.blockDepth--
					i += 2
					continue
				}
				i++
			}
			if state.blockDepth == 0 && i < len(runes) {
				// ensure we include trailing segment outside comment
			}
			out.append(TokenComment, string(runes[start:i]))
			continue
		}

		r := runes[i]
		if unicode.IsSpace(r) {
			start := i
			for i < len(runes) && unicode.IsSpace(runes[i]) {
				i++
			}
			out.append(TokenPlain, string(runes[start:i]))
			continue
		}

		if r == '"' {
			start := i
			i++
			state.inString = true
			for i < len(runes) {
				if runes[i] == '\\' && i+1 < len(runes) {
					i += 2
					continue
				}
				if runes[i] == '"' {
					i++
					state.inString = false
					break
				}
				i++
			}
			if state.inString {
				i = len(runes)
			}
			out.append(TokenString, string(runes[start:i]))
			continue
		}

		if isNumberStart(runes, i) {
			start := i
			i = consumeNumber(runes, i)
			out.append(TokenNumber, string(runes[start:i]))
			continue
		}

		if r == '@' {
			start := i
			i++
			for i < len(runes) && isIdentifierPart(runes[i]) {
				i++
			}
			out.append(TokenAttribute, string(runes[start:i]))
			continue
		}

		if isIdentifierStart(r) {
			start := i
			i++
			for i < len(runes) && isIdentifierPart(runes[i]) {
				i++
			}
			text := string(runes[start:i])
			out.append(classifyIdentifier(text), text)
			continue
		}

		if isOperatorRune(r) {
			start := i
			i++
			for i < len(runes) && isOperatorRune(runes[i]) {
				// prevent swallowing start of comments
				if runes[i] == '/' && i+1 < len(runes) && (runes[i+1] == '/' || runes[i+1] == '*') {
					break
				}
				i++
			}
			out.append(TokenOperator, string(runes[start:i]))
			continue
		}

		out.append(TokenPlain, string(runes[i:i+1]))
		i++
	}

	return out, state
}

func isIdentifierStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentifierPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isOperatorRune(r rune) bool {
	switch r {
	case '+', '-', '*', '/', '%', '=', '!', '<', '>', '&', '|', '^', '~', '?', ':', '.', '#':
		return true
	}
	return false
}

func isNumberStart(runes []rune, i int) bool {
	if i >= len(runes) {
		return false
	}
	r := runes[i]
	if unicode.IsDigit(r) {
		return true
	}
	if r == '.' && i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
		return true
	}
	return false
}

func consumeNumber(runes []rune, i int) int {
	if i >= len(runes) {
		return i
	}
	// Hex / binary prefixes
	if runes[i] == '0' && i+1 < len(runes) {
		switch runes[i+1] {
		case 'x', 'X':
			i += 2
			for i < len(runes) && (isHexDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			return i
		case 'b', 'B':
			i += 2
			for i < len(runes) && (runes[i] == '0' || runes[i] == '1' || runes[i] == '_') {
				i++
			}
			return i
		}
	}
	// Decimal part
	for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '_') {
		i++
	}
	if i < len(runes) && runes[i] == '.' {
		if i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
			i++
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
		}
	}
	if i < len(runes) && (runes[i] == 'e' || runes[i] == 'E') {
		j := i + 1
		if j < len(runes) && (runes[j] == '+' || runes[j] == '-') {
			j++
		}
		if j < len(runes) && unicode.IsDigit(runes[j]) {
			j++
			for j < len(runes) && (unicode.IsDigit(runes[j]) || runes[j] == '_') {
				j++
			}
			i = j
		}
	}
	return i
}

func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

var keywords = func() map[string]struct{} {
	list := []string{
		"pub", "fn", "let", "mut", "if", "else", "while", "for", "in",
		"break", "continue", "import", "newtype", "type", "literal", "alias",
		"extern", "return", "signal", "compare", "spawn", "is", "finally",
		"async", "await", "macro", "pragma", "own",
		"true", "false", "nothing",
	}
	set := make(map[string]struct{}, len(list))
	for _, k := range list {
		set[k] = struct{}{}
	}
	return set
}()

var builtinTypes = func() map[string]struct{} {
	list := []string{
		"int", "uint", "float",
		"int8", "int16", "int32", "int64",
		"uint8", "uint16", "uint32", "uint64",
		"float16", "float32", "float64",
		"bool", "string",
	}
	set := make(map[string]struct{}, len(list))
	for _, k := range list {
		set[k] = struct{}{}
	}
	return set
}()

func classifyIdentifier(text string) TokenKind {
	lower := strings.ToLower(text)
	if _, ok := keywords[lower]; ok {
		return TokenKeyword
	}
	if _, ok := builtinTypes[lower]; ok {
		return TokenBuiltinType
	}
	if isGenericTypeName(text) {
		return TokenTypeName
	}
	return TokenPlain
}

func isGenericTypeName(text string) bool {
	if len(text) == 0 {
		return false
	}
	r, _ := utf8.DecodeRuneInString(text)
	if !unicode.IsUpper(r) {
		return false
	}
	for _, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// HighlightTheme contains styles for each token kind.
type HighlightTheme struct {
	styles map[TokenKind]lipgloss.Style
}

// Style returns the style for the given token kind.
func (t HighlightTheme) Style(kind TokenKind) lipgloss.Style {
	if t.styles == nil {
		return lipgloss.NewStyle()
	}
	if style, ok := t.styles[kind]; ok {
		return style
	}
	if style, ok := t.styles[TokenPlain]; ok {
		return style
	}
	return lipgloss.NewStyle()
}

// NewHighlightTheme constructs a highlight theme for the given mode ("light" or "dark").
func NewHighlightTheme(mode string) HighlightTheme {
	dark := map[TokenKind]lipgloss.Style{
		TokenPlain:       lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")),
		TokenKeyword:     lipgloss.NewStyle().Foreground(lipgloss.Color("#C084FC")).Bold(true),
		TokenBuiltinType: lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8")).Bold(true),
		TokenTypeName:    lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA")),
		TokenString:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FDE68A")),
		TokenNumber:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FB923C")),
		TokenComment:     lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")),
		TokenDirective:   lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")),
		TokenAttribute:   lipgloss.NewStyle().Foreground(lipgloss.Color("#F472B6")),
		TokenOperator:    lipgloss.NewStyle().Foreground(lipgloss.Color("#A5B4FC")),
	}

	light := map[TokenKind]lipgloss.Style{
		TokenPlain:       lipgloss.NewStyle().Foreground(lipgloss.Color("#1F2937")),
		TokenKeyword:     lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true),
		TokenBuiltinType: lipgloss.NewStyle().Foreground(lipgloss.Color("#2563EB")).Bold(true),
		TokenTypeName:    lipgloss.NewStyle().Foreground(lipgloss.Color("#1D4ED8")),
		TokenString:      lipgloss.NewStyle().Foreground(lipgloss.Color("#B45309")),
		TokenNumber:      lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")),
		TokenComment:     lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
		TokenDirective:   lipgloss.NewStyle().Foreground(lipgloss.Color("#0EA5E9")),
		TokenAttribute:   lipgloss.NewStyle().Foreground(lipgloss.Color("#DB2777")),
		TokenOperator:    lipgloss.NewStyle().Foreground(lipgloss.Color("#2563EB")),
	}

	if strings.ToLower(mode) == "light" {
		return HighlightTheme{styles: light}
	}
	return HighlightTheme{styles: dark}
}

// RenderOptions configures how a highlighted line is rendered.
type RenderOptions struct {
	MaxWidth    int
	AddEllipsis bool
	CursorCol   int
	CursorStyle *lipgloss.Style
	PadToWidth  int
}

// Render formats a highlighted line using the provided theme and options.
// It returns the rendered string and a bool indicating whether the line was truncated.
func Render(line Line, theme HighlightTheme, opts RenderOptions) (string, bool) {
	var builder strings.Builder
	maxWidth := opts.MaxWidth
	useLimit := maxWidth > 0
	cursorCol := opts.CursorCol
	cursorActive := cursorCol >= 0 && opts.CursorStyle != nil
	cursorStyle := lipgloss.NewStyle()
	if opts.CursorStyle != nil {
		cursorStyle = *opts.CursorStyle
	}

	allowedWidth := maxWidth
	requiresEllipsis := false
	if useLimit && opts.AddEllipsis && line.Length() > maxWidth {
		requiresEllipsis = true
	}
	if requiresEllipsis && maxWidth > 1 {
		allowedWidth = maxWidth - 1
	}

	width := 0
	truncated := false
	absolutePos := 0

renderLoop:
	for _, seg := range line.Segments {
		runes := []rune(seg.Text)
		start := 0
		style := theme.Style(seg.Kind)
		for start < len(runes) {
			if useLimit && width >= allowedWidth {
				truncated = true
				break renderLoop
			}

			end := len(runes)
			if useLimit && width+(end-start) > allowedWidth {
				end = start + (allowedWidth - width)
			}

			if cursorActive && cursorCol >= absolutePos+start && cursorCol < absolutePos+end {
				cursorIndex := cursorCol - (absolutePos + start)
				if cursorIndex > 0 {
					text := string(runes[start : start+cursorIndex])
					builder.WriteString(style.Render(text))
					width += utf8.RuneCountInString(text)
					start += cursorIndex
				}
				if useLimit && width >= allowedWidth {
					truncated = true
					break renderLoop
				}
				cursorRune := string(runes[start])
				builder.WriteString(cursorStyle.Render(cursorRune))
				width++
				start++
				continue
			}

			text := string(runes[start:end])
			builder.WriteString(style.Render(text))
			width += utf8.RuneCountInString(text)
			start = end
		}
		absolutePos += len(runes)
		if useLimit && width >= allowedWidth {
			truncated = width >= allowedWidth && (absolutePos < line.Length())
			break
		}
	}

	if cursorActive && cursorCol >= line.Length() && (!useLimit || width < allowedWidth) {
		builder.WriteString(cursorStyle.Render(" "))
		width++
	}

	if truncated && opts.AddEllipsis && maxWidth > 0 {
		if width < maxWidth {
			builder.WriteString(theme.Style(TokenPlain).Render("…"))
			width++
		} else if width == maxWidth {
			// already filled exactly, replace last char with ellipsis.
			// This case is rare; skip to avoid complex manipulation.
		}
	}

	if opts.PadToWidth > 0 && width < opts.PadToWidth {
		padding := strings.Repeat(" ", opts.PadToWidth-width)
		builder.WriteString(padding)
	}

	return builder.String(), truncated
}
