package screens

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"surge-tui/internal/config"
	"surge-tui/internal/ui/styles"
)

// DiagnosticSeverity описывает уровень серьёзности диагностики.
type DiagnosticSeverity int

const (
	SeverityInfo DiagnosticSeverity = iota
	SeverityWarning
	SeverityError
)

// EditorDiagnostic описывает проблему в файле для отображения в гаттере.
type EditorDiagnostic struct {
	Line      int
	Column    int
	EndLine   int
	EndColumn int
	Message   string
	Severity  DiagnosticSeverity
}

type textPosRange struct {
	start textPos
	end   textPos
}

type editorSnapshot struct {
	lines  [][]rune
	cursor textPos
	anchor *textPos
	dirty  bool
}

type highlightToken struct {
	start int
	end   int
	style lipgloss.Style
}

type editorState struct {
	buffer                  *textBuffer
	filePath                string
	fileMode                fs.FileMode
	lastModified            time.Time
	lastSavedContent        string
	dirty                   bool
	loading                 bool
	statusMessage           string
	statusAt                time.Time
	clipboard               string
	diagnostics             map[int][]EditorDiagnostic
	diagnosticLines         []int
	undoStack               []editorSnapshot
	redoStack               []editorSnapshot
	maxHistory              int
	searchQuery             string
	searchMatches           []textPosRange
	searchIndex             int
	autoSaveToken           int
	pendingAutoSave         int
	autoSaveEnabled         bool
	autoSaveDelay           time.Duration
	externalRunning         bool
	launchExternalAfterSave bool
	viewportWidth           int
	viewportHeight          int
	scrollOffset            int
	horizontalOffset        int
}

// EditorScreen реализует экран редактирования кода.
type EditorScreen struct {
	BaseScreen

	cfg   *config.Config
	theme *styles.Theme

	state editorState

	searchInput textinput.Model

	keywordRegex *regexp.Regexp
	stringRegex  *regexp.Regexp
	numberRegex  *regexp.Regexp
	commentRegex *regexp.Regexp
	blockComment *regexp.Regexp

	// Styles
	lineNumberStyle        lipgloss.Style
	activeLineNumberStyle  lipgloss.Style
	gutterStyle            lipgloss.Style
	gutterProblemError     lipgloss.Style
	gutterProblemWarning   lipgloss.Style
	currentLineStyle       lipgloss.Style
	selectionStyle         lipgloss.Style
	cursorStyle            lipgloss.Style
	searchHighlightStyle   lipgloss.Style
	statusMessageStyle     lipgloss.Style
	statusErrorStyle       lipgloss.Style
	statusSuccessStyle     lipgloss.Style
	tabHintStyle           lipgloss.Style
	diagnosticMessageStyle lipgloss.Style
}

type editorLoadedMsg struct {
	path    string
	content string
	modTime time.Time
	mode    fs.FileMode
	err     error
}

type saveCompletedMsg struct {
	err error
}

type autoSaveTickMsg struct {
	token int
}

type externalEditorClosedMsg struct {
	err error
}

// NewEditorScreen создаёт экран редактора.
func NewEditorScreen(cfg *config.Config, theme *styles.Theme) *EditorScreen {
	es := &EditorScreen{
		BaseScreen: NewBaseScreen("Editor"),
		cfg:        cfg,
		theme:      theme,
	}
	es.state.buffer = newTextBuffer()
	es.state.maxHistory = 50
	es.state.diagnostics = make(map[int][]EditorDiagnostic)
	es.state.autoSaveEnabled = cfg.Editor.AutoSave
	if cfg.Editor.AutoSaveDelay <= 0 {
		es.state.autoSaveDelay = 5 * time.Second
	} else {
		es.state.autoSaveDelay = time.Duration(cfg.Editor.AutoSaveDelay) * time.Second
	}
	es.searchInput = textinput.New()
	es.searchInput.Placeholder = "Search"
	es.searchInput.CharLimit = 256
	es.searchInput.Prompt = "/ "
	es.keywordRegex = regexp.MustCompile(`\b(package|import|func|var|const|type|struct|if|else|for|switch|case|return|break|continue|go|defer|map|range|chan|select|interface|class|enum|while|do|try|catch|finally)\b`)
	es.stringRegex = regexp.MustCompile(`"([^"\\]|\\.)*"|'([^'\\]|\\.)*'`)
	es.numberRegex = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	es.commentRegex = regexp.MustCompile(`//.*$`)
	es.blockComment = regexp.MustCompile(`/\*.*?\*/`)
	es.applyTheme(theme)
	return es
}

// SetTheme применяет тему к экрану.
func (es *EditorScreen) SetTheme(theme *styles.Theme) {
	es.theme = theme
	es.applyTheme(theme)
}

func (es *EditorScreen) applyTheme(theme *styles.Theme) {
	base := theme.TextStyle
	es.lineNumberStyle = base.Copy().Foreground(lipgloss.Color("#64748B"))
	es.activeLineNumberStyle = base.Copy().Foreground(lipgloss.Color("#F97316")).Bold(true)
	es.gutterStyle = base.Copy().Foreground(lipgloss.Color("#475569"))
	es.gutterProblemError = base.Copy().Foreground(lipgloss.Color("#EF4444")).Bold(true)
	es.gutterProblemWarning = base.Copy().Foreground(lipgloss.Color("#FBBF24")).Bold(true)
	es.currentLineStyle = base.Copy().Background(lipgloss.Color("#1E293B"))
	es.selectionStyle = base.Copy().Background(lipgloss.Color("#334155"))
	es.cursorStyle = base.Copy().Background(lipgloss.Color("#F97316")).Foreground(lipgloss.Color("#0F172A"))
	es.searchHighlightStyle = base.Copy().Background(lipgloss.Color("#4C1D95"))
	es.statusMessageStyle = base.Copy().Foreground(lipgloss.Color("#E2E8F0"))
	es.statusErrorStyle = theme.ErrorStyle
	es.statusSuccessStyle = theme.SuccessStyle
	es.tabHintStyle = base.Copy().Foreground(lipgloss.Color("#94A3B8"))
	es.diagnosticMessageStyle = base.Copy().Foreground(lipgloss.Color("#F97316"))
}

// Init реализует интерфейс tea.Model.
func (es *EditorScreen) Init() tea.Cmd {
	return nil
}

// InterceptKey позволяет редактору перехватывать клавиши до глобальных команд.
func (es *EditorScreen) InterceptKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	handled, cmd := es.handleKey(msg)
	return handled, cmd
}

// Update обновляет состояние экрана.
func (es *EditorScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		handled, cmd := es.handleKey(m)
		if handled {
			return es, cmd
		}
	case tea.WindowSizeMsg:
		es.SetSize(m.Width, m.Height-1)
		es.recomputeViewport()
		return es, nil
	case editorLoadedMsg:
		es.handleLoaded(m)
		return es, nil
	case saveCompletedMsg:
		var cmd tea.Cmd
		if m.err != nil {
			es.setStatusError(fmt.Sprintf("Save failed: %v", m.err))
			es.state.launchExternalAfterSave = false
		} else {
			es.setStatusSuccess("Saved")
			es.state.dirty = false
			es.state.lastSavedContent = es.state.buffer.fullText()
			es.state.lastModified = time.Now()
			es.removeAutoSave()
			if es.state.launchExternalAfterSave {
				es.state.launchExternalAfterSave = false
				cmd = es.runExternalEditorCommand()
			}
		}
		return es, cmd
	case autoSaveTickMsg:
		if m.token == es.state.pendingAutoSave && es.state.autoSaveEnabled && es.state.dirty {
			if err := es.writeAutoSave(); err != nil {
				es.setStatusError(fmt.Sprintf("Autosave failed: %v", err))
			} else {
				es.setStatusMessage("Autosaved")
			}
			es.state.pendingAutoSave = 0
		}
		return es, nil
	case externalEditorClosedMsg:
		es.state.externalRunning = false
		if m.err != nil {
			es.setStatusError(fmt.Sprintf("External editor: %v", m.err))
		}
		if es.state.filePath != "" {
			if info, err := os.Stat(es.state.filePath); err == nil && info.ModTime().After(es.state.lastModified) {
				return es, es.OpenFile(es.state.filePath)
			}
		}
		return es, nil
	}
	return es, nil
}

// View отрисовывает экран редактора.
func (es *EditorScreen) View() string {
	if es.Width() == 0 {
		return "Loading editor..."
	}

	var parts []string
	header := es.renderHeader()
	parts = append(parts, header)

	if es.state.loading {
		loading := es.theme.SubtitleStyle.Render("Loading file...")
		parts = append(parts, loading)
	} else {
		parts = append(parts, es.renderBuffer())
	}

	parts = append(parts, es.renderFooter())
	return strings.Join(parts, "\n")
}

// Title возвращает заголовок.
func (es *EditorScreen) Title() string {
	if es.state.filePath == "" {
		return "Editor"
	}
	name := filepath.Base(es.state.filePath)
	if es.state.dirty {
		name += "*"
	}
	return "Editor: " + name
}

// ShortHelp возвращает краткую справку.
func (es *EditorScreen) ShortHelp() string {
	return "Ctrl+S Save • / Search • Ctrl+Z Undo • Ctrl+Y Redo"
}

// FullHelp возвращает полную справку.
func (es *EditorScreen) FullHelp() []string {
	help := es.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Editor:",
		"  Ctrl+S - Save file",
		"  Ctrl+Z/Ctrl+Y - Undo/Redo",
		"  Ctrl+C/Ctrl+X/Ctrl+V - Copy/Cut/Paste",
		"  Tab - Insert indentation",
		"  / - Search, Enter to confirm, n/N for next/previous match",
		"  Alt+Up/Alt+Down - Navigate diagnostics",
		"  Ctrl+E - Open in external editor",
	}...)
	return help
}

// OnEnter обновляет статус при входе.
func (es *EditorScreen) OnEnter() tea.Cmd {
	if es.state.filePath == "" {
		es.setStatusMessage("Select a file to edit from Project screen")
	}
	return nil
}

// OnExit очищает поисковый режим.
func (es *EditorScreen) OnExit() tea.Cmd {
	es.searchInput.Blur()
	es.state.searchMatches = nil
	es.state.searchQuery = ""
	return nil
}

// CanExit всегда разрешает выход (автосохранение защищает данные).
func (es *EditorScreen) CanExit() bool {
	return true
}

// OpenFile загружает файл в редактор.
func (es *EditorScreen) OpenFile(path string) tea.Cmd {
	es.state.loading = true
	es.state.filePath = path
	es.state.scrollOffset = 0
	es.state.horizontalOffset = 0
	es.state.searchMatches = nil
	es.state.searchIndex = 0
	es.state.redoStack = nil
	es.state.undoStack = nil
	es.state.diagnostics = make(map[int][]EditorDiagnostic)
	es.state.diagnosticLines = nil
	es.setStatusMessage(fmt.Sprintf("Opening %s", filepath.Base(path)))
	return es.loadFile(path)
}

func (es *EditorScreen) loadFile(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return editorLoadedMsg{path: path, err: err}
		}
		info, err := os.Stat(path)
		if err != nil {
			return editorLoadedMsg{path: path, content: string(data), err: err}
		}
		return editorLoadedMsg{path: path, content: string(data), modTime: info.ModTime(), mode: info.Mode()}
	}
}

func (es *EditorScreen) handleLoaded(msg editorLoadedMsg) {
	es.state.loading = false
	if msg.err != nil {
		es.setStatusError(fmt.Sprintf("Failed to open %s: %v", filepath.Base(msg.path), msg.err))
		return
	}
	if es.state.buffer == nil {
		es.state.buffer = newTextBuffer()
	}
	es.state.buffer.replaceAll(msg.content)
	es.state.dirty = false
	es.state.lastModified = msg.modTime
	es.state.fileMode = msg.mode
	es.state.lastSavedContent = msg.content
	es.setStatusSuccess(fmt.Sprintf("Opened %s", filepath.Base(msg.path)))
	es.recomputeViewport()
}

func (es *EditorScreen) handleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if es.state.loading {
		return false, nil
	}
	if es.searchInput.Focused() {
		return es.handleSearchKey(msg)
	}

	key := msg.String()
	switch key {
	case "ctrl+s":
		if es.state.filePath == "" {
			es.setStatusError("No file selected")
			return true, nil
		}
		return true, es.saveFile()
	case "ctrl+z":
		es.undo()
		return true, nil
	case "ctrl+y", "ctrl+shift+z":
		es.redo()
		return true, nil
	case "ctrl+c":
		es.copySelection()
		return true, nil
	case "ctrl+x":
		return true, es.cutSelection()
	case "ctrl+v":
		return true, es.pasteClipboard()
	case "ctrl+f":
		es.enterSearchMode()
		return true, nil
	case "ctrl+e":
		return true, es.launchExternalEditor()
	case "tab":
		return true, es.insertIndent()
	case "backspace":
		return true, es.deleteBackward()
	case "delete":
		return true, es.deleteForward()
	case "enter":
		return true, es.insertNewLine()
	case "esc":
		es.state.buffer.clearAnchor()
		return true, nil
	case "/":
		es.enterSearchMode()
		return true, nil
	case "n":
		if es.state.searchQuery != "" {
			es.gotoNextMatch(1)
			return true, nil
		}
	case "shift+n":
		if es.state.searchQuery != "" {
			es.gotoNextMatch(-1)
			return true, nil
		}
	case "alt+down":
		es.gotoDiagnostic(true)
		return true, nil
	case "alt+up":
		es.gotoDiagnostic(false)
		return true, nil
	}

	switch key {
	case "left":
		es.moveCursorLeft(false)
		return true, nil
	case "right":
		es.moveCursorRight(false)
		return true, nil
	case "up":
		es.moveCursorUp(false)
		return true, nil
	case "down":
		es.moveCursorDown(false)
		return true, nil
	case "shift+left":
		es.moveCursorLeft(true)
		return true, nil
	case "shift+right":
		es.moveCursorRight(true)
		return true, nil
	case "shift+up":
		es.moveCursorUp(true)
		return true, nil
	case "shift+down":
		es.moveCursorDown(true)
		return true, nil
	case "home":
		es.moveCursorHome(false)
		return true, nil
	case "end":
		es.moveCursorEnd(false)
		return true, nil
	case "shift+home":
		es.moveCursorHome(true)
		return true, nil
	case "shift+end":
		es.moveCursorEnd(true)
		return true, nil
	case "pgup":
		es.pageUp(false)
		return true, nil
	case "pgdown":
		es.pageDown(false)
		return true, nil
	case "shift+pgup":
		es.pageUp(true)
		return true, nil
	case "shift+pgdown":
		es.pageDown(true)
		return true, nil
	}

	if msg.Type == tea.KeyRunes {
		text := string(msg.Runes)
		if text != "" {
			return true, es.insertText(text)
		}
	}

	return false, nil
}

func (es *EditorScreen) handleSearchKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter":
		es.state.searchQuery = es.searchInput.Value()
		es.updateSearchMatches()
		es.searchInput.Blur()
		es.gotoNextMatch(1)
		return true, nil
	case "esc":
		es.searchInput.Blur()
		es.state.searchQuery = ""
		es.state.searchMatches = nil
		return true, nil
	}
	var cmd tea.Cmd
	es.searchInput, cmd = es.searchInput.Update(msg)
	es.state.searchQuery = es.searchInput.Value()
	es.updateSearchMatches()
	return true, cmd
}

func (es *EditorScreen) insertIndent() tea.Cmd {
	count := es.cfg.Editor.TabSize
	if count <= 0 {
		count = 4
	}
	if es.cfg.Editor.UseSpaces {
		return es.insertText(strings.Repeat(" ", count))
	}
	return es.insertText("\t")
}

func (es *EditorScreen) insertText(text string) tea.Cmd {
	es.beginMutation()
	es.state.buffer.insertString(text)
	return es.afterMutation()
}

func (es *EditorScreen) insertNewLine() tea.Cmd {
	es.beginMutation()
	es.state.buffer.insertRune('\n')
	return es.afterMutation()
}

func (es *EditorScreen) deleteBackward() tea.Cmd {
	es.beginMutation()
	es.state.buffer.deleteBackward()
	return es.afterMutation()
}

func (es *EditorScreen) deleteForward() tea.Cmd {
	es.beginMutation()
	es.state.buffer.deleteForward()
	return es.afterMutation()
}

func (es *EditorScreen) copySelection() {
	es.state.clipboard = es.state.buffer.selectedText()
	if es.state.clipboard != "" {
		es.setStatusMessage("Copied")
	}
}

func (es *EditorScreen) cutSelection() tea.Cmd {
	if !es.state.buffer.hasSelection() {
		return nil
	}
	es.state.clipboard = es.state.buffer.selectedText()
	es.beginMutation()
	es.state.buffer.deleteSelection()
	cmd := es.afterMutation()
	es.setStatusMessage("Cut")
	return cmd
}

func (es *EditorScreen) pasteClipboard() tea.Cmd {
	if es.state.clipboard == "" {
		return nil
	}
	return es.insertText(es.state.clipboard)
}

func (es *EditorScreen) moveCursorLeft(selecting bool) {
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorLeft()
	es.ensureCursorVisible()
}

func (es *EditorScreen) moveCursorRight(selecting bool) {
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorRight()
	es.ensureCursorVisible()
}

func (es *EditorScreen) moveCursorUp(selecting bool) {
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorUp()
	es.ensureCursorVisible()
}

func (es *EditorScreen) moveCursorDown(selecting bool) {
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorDown()
	es.ensureCursorVisible()
}

func (es *EditorScreen) moveCursorHome(selecting bool) {
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorTo(es.state.buffer.cursor.line, 0)
	es.ensureCursorVisible()
}

func (es *EditorScreen) moveCursorEnd(selecting bool) {
	line := es.state.buffer.cursor.line
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorTo(line, es.state.buffer.lineLength(line))
	es.ensureCursorVisible()
}

func (es *EditorScreen) pageUp(selecting bool) {
	if es.state.viewportHeight <= 0 {
		es.state.viewportHeight = 1
	}
	target := es.state.buffer.cursor.line - es.state.viewportHeight
	if target < 0 {
		target = 0
	}
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorTo(target, es.state.buffer.cursor.col)
	es.ensureCursorVisible()
}

func (es *EditorScreen) pageDown(selecting bool) {
	if es.state.viewportHeight <= 0 {
		es.state.viewportHeight = 1
	}
	target := es.state.buffer.cursor.line + es.state.viewportHeight
	if target > es.state.buffer.lineCount()-1 {
		target = es.state.buffer.lineCount() - 1
	}
	if selecting && !es.state.buffer.hasSelection() {
		es.state.buffer.setAnchor()
	}
	if !selecting {
		es.state.buffer.clearAnchor()
	}
	es.state.buffer.moveCursorTo(target, es.state.buffer.cursor.col)
	es.ensureCursorVisible()
}

func (es *EditorScreen) ensureCursorVisible() {
	line := es.state.buffer.cursor.line
	if line < es.state.scrollOffset {
		es.state.scrollOffset = line
	}
	if es.state.viewportHeight > 0 {
		if line >= es.state.scrollOffset+es.state.viewportHeight {
			es.state.scrollOffset = line - es.state.viewportHeight + 1
		}
	}
	col := es.state.buffer.cursor.col
	available := es.contentWidth()
	if col < es.state.horizontalOffset {
		es.state.horizontalOffset = col
	} else if available > 0 && col >= es.state.horizontalOffset+available {
		es.state.horizontalOffset = col - available + 1
	}
}

func (es *EditorScreen) beginMutation() {
	snap := editorSnapshot{
		lines:  es.state.buffer.cloneLines(),
		cursor: es.state.buffer.cursor,
		dirty:  es.state.dirty,
	}
	if es.state.buffer.anchor != nil {
		anchor := *es.state.buffer.anchor
		snap.anchor = &anchor
	}
	es.state.undoStack = append(es.state.undoStack, snap)
	if len(es.state.undoStack) > es.state.maxHistory {
		es.state.undoStack = es.state.undoStack[1:]
	}
	es.state.redoStack = nil
}

func (es *EditorScreen) afterMutation() tea.Cmd {
	es.state.dirty = true
	es.ensureCursorVisible()
	es.updateSearchMatches()
	return es.scheduleAutoSave()
}

func (es *EditorScreen) undo() {
	if len(es.state.undoStack) == 0 {
		return
	}
	current := editorSnapshot{
		lines:  es.state.buffer.cloneLines(),
		cursor: es.state.buffer.cursor,
		dirty:  es.state.dirty,
	}
	if es.state.buffer.anchor != nil {
		anchor := *es.state.buffer.anchor
		current.anchor = &anchor
	}
	es.state.redoStack = append(es.state.redoStack, current)

	snap := es.state.undoStack[len(es.state.undoStack)-1]
	es.state.undoStack = es.state.undoStack[:len(es.state.undoStack)-1]
	es.restoreSnapshot(snap)
}

func (es *EditorScreen) redo() {
	if len(es.state.redoStack) == 0 {
		return
	}
	current := editorSnapshot{
		lines:  es.state.buffer.cloneLines(),
		cursor: es.state.buffer.cursor,
		dirty:  es.state.dirty,
	}
	if es.state.buffer.anchor != nil {
		anchor := *es.state.buffer.anchor
		current.anchor = &anchor
	}
	es.state.undoStack = append(es.state.undoStack, current)

	snap := es.state.redoStack[len(es.state.redoStack)-1]
	es.state.redoStack = es.state.redoStack[:len(es.state.redoStack)-1]
	es.restoreSnapshot(snap)
}

func (es *EditorScreen) restoreSnapshot(snap editorSnapshot) {
	es.state.buffer.lines = snap.lines
	es.state.buffer.cursor = snap.cursor
	if snap.anchor != nil {
		anchor := *snap.anchor
		es.state.buffer.anchor = &anchor
	} else {
		es.state.buffer.anchor = nil
	}
	es.state.dirty = snap.dirty
	es.ensureCursorVisible()
	es.updateSearchMatches()
}

func (es *EditorScreen) scheduleAutoSave() tea.Cmd {
	if !es.state.autoSaveEnabled {
		return nil
	}
	es.state.autoSaveToken++
	token := es.state.autoSaveToken
	es.state.pendingAutoSave = token
	delay := es.state.autoSaveDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return autoSaveTickMsg{token: token} })
}

func (es *EditorScreen) writeAutoSave() error {
	if es.state.filePath == "" {
		return nil
	}
	path := es.autoSavePath()
	data := []byte(es.state.buffer.fullText())
	return os.WriteFile(path, data, 0644)
}

func (es *EditorScreen) autoSavePath() string {
	if es.state.filePath == "" {
		return ""
	}
	dir := filepath.Dir(es.state.filePath)
	base := filepath.Base(es.state.filePath)
	return filepath.Join(dir, fmt.Sprintf(".%s.autosave", base))
}

func (es *EditorScreen) removeAutoSave() {
	if path := es.autoSavePath(); path != "" {
		_ = os.Remove(path)
	}
}

func (es *EditorScreen) saveFile() tea.Cmd {
	content := es.state.buffer.fullText()
	path := es.state.filePath
	mode := es.state.fileMode
	return func() tea.Msg {
		if path == "" {
			return saveCompletedMsg{err: errors.New("no file path")}
		}
		perm := mode
		if perm == 0 {
			perm = 0644
		}
		err := os.WriteFile(path, []byte(content), perm)
		if err == nil {
			_ = os.Chtimes(path, time.Now(), time.Now())
		}
		return saveCompletedMsg{err: err}
	}
}

func (es *EditorScreen) setStatusMessage(msg string) {
	es.state.statusMessage = msg
	es.state.statusAt = time.Now()
}

func (es *EditorScreen) setStatusError(msg string) {
	es.state.statusMessage = es.statusErrorStyle.Render(msg)
	es.state.statusAt = time.Now()
}

func (es *EditorScreen) setStatusSuccess(msg string) {
	es.state.statusMessage = es.statusSuccessStyle.Render(msg)
	es.state.statusAt = time.Now()
}

func (es *EditorScreen) renderHeader() string {
	if es.state.filePath == "" {
		return es.theme.TitleStyle.Render("Editor")
	}
	name := filepath.Base(es.state.filePath)
	if es.state.dirty {
		name += "*"
	}
	info := fmt.Sprintf("%s (%d lines)", name, es.state.buffer.lineCount())
	return es.theme.TitleStyle.Render(info)
}

func (es *EditorScreen) renderBuffer() string {
	if es.state.viewportHeight <= 0 {
		es.recomputeViewport()
	}
	top := es.state.scrollOffset
	bottom := top + es.state.viewportHeight
	if bottom > es.state.buffer.lineCount() {
		bottom = es.state.buffer.lineCount()
	}
	gutterWidth := es.gutterWidth()
	contentWidth := es.contentWidth()
	var lines []string
	for i := top; i < bottom; i++ {
		rendered := es.renderLine(i, gutterWidth, contentWidth)
		lines = append(lines, rendered)
	}
	for len(lines) < es.state.viewportHeight {
		lines = append(lines, strings.Repeat(" ", es.Width()))
	}
	return strings.Join(lines, "\n")
}

func (es *EditorScreen) renderFooter() string {
	status := es.state.statusMessage
	if status == "" {
		if es.state.filePath != "" {
			status = fmt.Sprintf("%s:%d:%d", filepath.Base(es.state.filePath), es.state.buffer.cursor.line+1, es.state.buffer.cursor.col+1)
		} else {
			status = "No file"
		}
	}
	diagCount := len(es.state.diagnosticLines)
	if diagCount > 0 {
		status += fmt.Sprintf(" • %d diagnostics", diagCount)
	}
	if es.state.dirty {
		status += " • Modified"
	}
	if es.searchInput.Focused() {
		return es.theme.StatusBarStyle.Width(es.Width()).Render(es.searchInput.View())
	}
	return es.theme.StatusBarStyle.Width(es.Width()).Render(status)
}

func (es *EditorScreen) gutterWidth() int {
	digits := 1
	if count := es.state.buffer.lineCount(); count > 0 {
		digits = len(fmt.Sprintf("%d", count))
	}
	// line numbers + space + indicator + space
	return digits + 3
}

func (es *EditorScreen) contentWidth() int {
	gutter := es.gutterWidth()
	width := es.Width() - gutter
	if width < 1 {
		width = 1
	}
	return width
}

func (es *EditorScreen) recomputeViewport() {
	header := 2 // header + footer
	if es.Height() <= header {
		es.state.viewportHeight = 1
		return
	}
	es.state.viewportHeight = es.Height() - header
	if es.Width() > 4 {
		es.searchInput.Width = es.Width() - 4
	} else {
		es.searchInput.Width = es.Width()
	}
	es.ensureCursorVisible()
}

func (es *EditorScreen) renderLine(lineIdx, gutterWidth, contentWidth int) string {
	lineNumber := fmt.Sprintf("%*d", gutterWidth-3, lineIdx+1)
	diagIndicator := " "
	if diags := es.state.diagnostics[lineIdx]; len(diags) > 0 {
		sev := diags[0].Severity
		switch sev {
		case SeverityError:
			diagIndicator = es.gutterProblemError.Render("●")
		case SeverityWarning:
			diagIndicator = es.gutterProblemWarning.Render("▲")
		default:
			diagIndicator = es.gutterStyle.Render("●")
		}
	} else {
		diagIndicator = es.gutterStyle.Render("·")
	}

	lnStyle := es.lineNumberStyle
	if lineIdx == es.state.buffer.cursor.line {
		lnStyle = es.activeLineNumberStyle
	}
	gutter := fmt.Sprintf("%s %s ", lnStyle.Render(lineNumber), diagIndicator)

	content := es.renderLineContent(lineIdx, contentWidth)
	return gutter + content
}

func (es *EditorScreen) renderLineContent(lineIdx, contentWidth int) string {
	lineRunes := es.state.buffer.lines[lineIdx]
	start := es.state.horizontalOffset
	if start < 0 {
		start = 0
	}
	end := start + contentWidth
	if end > len(lineRunes) {
		end = len(lineRunes)
	}
	visible := append([]rune{}, lineRunes[start:end]...)

	styles := make([]lipgloss.Style, len(visible))
	applied := make([]bool, len(visible))
	tokens := es.computeHighlightTokens(string(lineRunes))
	for _, token := range tokens {
		for i := token.start; i < token.end; i++ {
			if i >= start && i < end {
				styles[i-start] = token.style
				applied[i-start] = true
			}
		}
	}

	if es.state.searchQuery != "" {
		for _, rng := range es.state.searchMatches {
			if rng.start.line != lineIdx {
				continue
			}
			st := max(rng.start.col, start)
			en := min(rng.end.col, end)
			for i := st; i < en; i++ {
				styles[i-start] = es.searchHighlightStyle
				applied[i-start] = true
			}
		}
	}

	if es.state.buffer.hasSelection() {
		selStart, selEnd := es.state.buffer.selectionRange()
		if lineIdx >= selStart.line && lineIdx <= selEnd.line {
			startCol := selStart.col
			endCol := selEnd.col
			if lineIdx > selStart.line {
				startCol = 0
			}
			if lineIdx < selEnd.line {
				endCol = len(lineRunes)
			}
			st := max(startCol, start)
			en := min(endCol, end)
			for i := st; i < en; i++ {
				styles[i-start] = es.selectionStyle
				applied[i-start] = true
			}
		}
	}

	cursorCol := es.state.buffer.cursor.col
	cursorOnLine := (lineIdx == es.state.buffer.cursor.line)
	builder := strings.Builder{}
	col := start
	for idx, r := range visible {
		style := styles[idx]
		if !applied[idx] {
			style = es.theme.TextStyle
		}
		if cursorOnLine && col == cursorCol {
			builder.WriteString(es.cursorStyle.Render(string(r)))
		} else {
			builder.WriteString(style.Render(string(r)))
		}
		col++
	}
	if cursorOnLine && cursorCol >= end {
		builder.WriteString(es.cursorStyle.Render(" "))
	}
	rendered := builder.String()
	width := lipgloss.Width(rendered)
	if width < contentWidth {
		rendered += strings.Repeat(" ", contentWidth-width)
	}
	return rendered
}

func (es *EditorScreen) computeHighlightTokens(line string) []highlightToken {
	if !es.cfg.Editor.SyntaxHighlight {
		return nil
	}
	tokens := make([]highlightToken, 0, 8)
	addMatches := func(re *regexp.Regexp, style lipgloss.Style) {
		matches := re.FindAllStringIndex(line, -1)
		for _, match := range matches {
			start := utf8.RuneCountInString(line[:match[0]])
			end := utf8.RuneCountInString(line[:match[1]])
			tokens = append(tokens, highlightToken{start: start, end: end, style: style})
		}
	}
	addMatches(es.commentRegex, es.gutterProblemWarning)
	addMatches(es.blockComment, es.gutterProblemWarning)
	addMatches(es.stringRegex, es.theme.HighlightStyle)
	addMatches(es.numberRegex, es.theme.WarningStyle)
	addMatches(es.keywordRegex, es.theme.SuccessStyle)
	return tokens
}

func (es *EditorScreen) updateSearchMatches() {
	es.state.searchMatches = nil
	query := es.state.searchQuery
	if query == "" {
		return
	}
	for lineIdx, lineRunes := range es.state.buffer.lines {
		text := string(lineRunes)
		offset := 0
		for {
			idx := strings.Index(text[offset:], query)
			if idx == -1 {
				break
			}
			idx += offset
			startCol := utf8.RuneCountInString(text[:idx])
			endCol := startCol + utf8.RuneCountInString(query)
			es.state.searchMatches = append(es.state.searchMatches, textPosRange{
				start: textPos{line: lineIdx, col: startCol},
				end:   textPos{line: lineIdx, col: endCol},
			})
			offset = idx + len(query)
		}
	}
	if es.state.searchIndex >= len(es.state.searchMatches) {
		es.state.searchIndex = 0
	}
}

func (es *EditorScreen) gotoNextMatch(direction int) {
	if len(es.state.searchMatches) == 0 {
		es.setStatusMessage("No matches")
		return
	}
	es.state.searchIndex += direction
	if es.state.searchIndex < 0 {
		es.state.searchIndex = len(es.state.searchMatches) - 1
	}
	if es.state.searchIndex >= len(es.state.searchMatches) {
		es.state.searchIndex = 0
	}
	match := es.state.searchMatches[es.state.searchIndex]
	es.state.buffer.moveCursorTo(match.start.line, match.start.col)
	es.state.buffer.setAnchor()
	es.state.buffer.moveCursorTo(match.end.line, match.end.col)
	es.ensureCursorVisible()
	es.setStatusMessage(fmt.Sprintf("Match %d/%d", es.state.searchIndex+1, len(es.state.searchMatches)))
}

// SetDiagnostics устанавливает диагностические сообщения для файла.
func (es *EditorScreen) SetDiagnostics(diags []EditorDiagnostic) {
	es.state.diagnostics = make(map[int][]EditorDiagnostic)
	es.state.diagnosticLines = nil
	for _, d := range diags {
		line := d.Line
		if line < 0 {
			line = 0
		}
		es.state.diagnostics[line] = append(es.state.diagnostics[line], d)
	}
	for line := range es.state.diagnostics {
		es.state.diagnosticLines = append(es.state.diagnosticLines, line)
	}
	sort.Ints(es.state.diagnosticLines)
}

func (es *EditorScreen) gotoDiagnostic(forward bool) {
	if len(es.state.diagnosticLines) == 0 {
		es.setStatusMessage("No diagnostics")
		return
	}
	current := es.state.buffer.cursor.line
	idx := sort.SearchInts(es.state.diagnosticLines, current)
	if forward {
		if idx < len(es.state.diagnosticLines) && es.state.diagnosticLines[idx] == current {
			idx++
		}
		if idx >= len(es.state.diagnosticLines) {
			idx = 0
		}
	} else {
		if idx >= len(es.state.diagnosticLines) || es.state.diagnosticLines[idx] > current {
			idx--
		}
		if idx < 0 {
			idx = len(es.state.diagnosticLines) - 1
		}
	}
	line := es.state.diagnosticLines[idx]
	es.state.buffer.moveCursorTo(line, 0)
	es.state.buffer.clearAnchor()
	es.ensureCursorVisible()
	messages := es.state.diagnostics[line]
	if len(messages) > 0 {
		es.setStatusMessage(es.diagnosticMessageStyle.Render(messages[0].Message))
	}
}

func (es *EditorScreen) enterSearchMode() {
	es.searchInput.SetValue(es.state.searchQuery)
	es.searchInput.CursorEnd()
	if es.Width() > 4 {
		es.searchInput.Width = es.Width() - 4
	}
	es.searchInput.Focus()
}

func (es *EditorScreen) launchExternalEditor() tea.Cmd {
	if es.state.filePath == "" {
		es.setStatusError("No file to open")
		return nil
	}
	if es.state.dirty {
		es.state.launchExternalAfterSave = true
		return es.saveFile()
	}
	es.state.launchExternalAfterSave = false
	return es.runExternalEditorCommand()
}

func (es *EditorScreen) runExternalEditorCommand() tea.Cmd {
	cmdStr := es.cfg.Editor.ExternalEditor
	if cmdStr == "" {
		cmdStr = os.Getenv("EDITOR")
	}
	if cmdStr == "" {
		es.setStatusError("External editor not configured")
		return nil
	}
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		es.setStatusError("External editor not configured")
		return nil
	}
	es.state.externalRunning = true
	es.setStatusMessage("Waiting for external editor to close...")
	cmd := exec.Command(parts[0], append(parts[1:], es.state.filePath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalEditorClosedMsg{err: err}
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
