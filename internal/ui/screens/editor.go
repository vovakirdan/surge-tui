package screens

import (
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"surge-tui/internal/platform"
)

// EditorScreen предоставляет просмотр и (в будущем) редактирование файла.
type EditorScreen struct {
	BaseScreen

	filePath string
	lines    []string
	scroll   int

	loading bool
	err     error

	stats    editorStats
	status   string
	statusAt time.Time

	softWrap bool
}

type editorStats struct {
	size      int64
	lineCount int
	runeCount int
	modTime   time.Time
}

// NewEditorScreen создает редактор без открытого файла.
func NewEditorScreen() *EditorScreen {
	return &EditorScreen{
		BaseScreen: NewBaseScreen("Editor"),
		lines:      []string{},
		scroll:     0,
		loading:    false,
		err:        nil,
		softWrap:   false,
	}
}

func (es *EditorScreen) Init() tea.Cmd {
	return nil
}

func (es *EditorScreen) OnEnter() tea.Cmd {
	if es.filePath == "" {
		es.setStatus("Select a file to view")
	}
	return nil
}

func (es *EditorScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		es.SetSize(m.Width, m.Height-1)
		return es, nil
	case tea.KeyMsg:
		return es.handleKey(m)
	case editorFileLoadedMsg:
		es.loading = false
		es.err = nil
		es.filePath = m.Path
		es.lines = m.Lines
		es.stats = m.Stats
		es.scroll = 0
		es.setStatus("Loaded")
		return es, nil
	case editorFileErrorMsg:
		es.loading = false
		es.err = m.Err
		es.filePath = m.Path
		es.lines = nil
		es.setStatus("")
		return es, nil
	}
	return es, nil
}

func (es *EditorScreen) View() string {
	if es.Width() == 0 {
		return "Initializing editor..."
	}
	if es.loading {
		return es.renderLoading()
	}
	if es.err != nil {
		return es.renderError()
	}
	if es.filePath == "" {
		return es.renderEmpty()
	}
	return es.renderContent()
}

func (es *EditorScreen) ShortHelp() string {
	return platform.ReplacePrimaryModifier("↑↓ Scroll • PgUp/PgDn • Ctrl+R Reload • g/G Top/Bottom")
}

func (es *EditorScreen) FullHelp() []string {
	help := es.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Editor Screen:",
		"  ↑/↓ or j/k - Scroll",
		"  PgUp/PgDn - Scroll by half page",
		"  g - Go to top",
		"  G - Go to bottom",
		platform.ReplacePrimaryModifier("  Ctrl+R - Reload current file"),
	}...)
	return help
}

// OpenFile загружает файл и возвращает команду для Bubble Tea.
func (es *EditorScreen) OpenFile(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	es.loading = true
	es.err = nil
	abs := path
	if p, err := filepath.Abs(path); err == nil {
		abs = p
	}
	es.filePath = abs
	return func() tea.Msg {
		data, err := os.ReadFile(abs)
		if err != nil {
			return editorFileErrorMsg{Path: abs, Err: err}
		}
		info, err := os.Stat(abs)
		if err != nil {
			return editorFileErrorMsg{Path: abs, Err: err}
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines := strings.Split(text, "\n")
		if len(lines) == 0 {
			lines = []string{""}
		}
		stats := editorStats{
			size:      info.Size(),
			lineCount: len(lines),
			runeCount: utf8.RuneCountInString(text),
			modTime:   info.ModTime(),
		}
		return editorFileLoadedMsg{Path: abs, Lines: lines, Stats: stats}
	}
}

func (es *EditorScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	if es.loading {
		return es, nil
	}
	key := platform.CanonicalKeyForLookup(msg.String())
	switch key {
	case "up", "k":
		es.scrollUp(1)
		return es, nil
	case "down", "j":
		es.scrollDown(1)
		return es, nil
	case "pgup", "ctrl+u":
		es.scrollUp(es.pageStep())
		return es, nil
	case "pgdown", "ctrl+d":
		es.scrollDown(es.pageStep())
		return es, nil
	case "g":
		es.scroll = 0
		return es, nil
	case "G":
		es.scrollBottom()
		return es, nil
	case "ctrl+r":
		if es.filePath != "" {
			return es, es.OpenFile(es.filePath)
		}
		return es, nil
	}
	return es, nil
}

func (es *EditorScreen) scrollUp(n int) {
	if n < 1 {
		n = 1
	}
	es.scroll -= n
	if es.scroll < 0 {
		es.scroll = 0
	}
}

func (es *EditorScreen) scrollDown(n int) {
	if n < 1 {
		n = 1
	}
	maxScroll := es.maxScroll()
	es.scroll += n
	if es.scroll > maxScroll {
		es.scroll = maxScroll
	}
}

func (es *EditorScreen) scrollBottom() {
	es.scroll = es.maxScroll()
}

func (es *EditorScreen) pageStep() int {
	h := es.contentHeight()
	if h <= 0 {
		return 1
	}
	step := max(h/2, 1)
	return step
}

func (es *EditorScreen) contentHeight() int {
	return max(es.Height()-3, 1) // header + status lines
}

func (es *EditorScreen) maxScroll() int {
	if len(es.lines) == 0 {
		return 0
	}
	h := es.contentHeight()
	max := len(es.lines) - h
	if max < 0 {
		max = 0
	}
	return max
}

func (es *EditorScreen) setStatus(msg string) {
	es.status = msg
	es.statusAt = time.Now()
}

func (es *EditorScreen) statusLine() string {
	if es.status == "" {
		return ""
	}
	if time.Since(es.statusAt) > 3*time.Second {
		es.status = ""
		return ""
	}
	return es.status
}
