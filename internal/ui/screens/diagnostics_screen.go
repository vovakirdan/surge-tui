package screens

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	core "surge-tui/internal/core/surge"
)

// DiagnosticsScreen отображает результаты `surge diag` и позволяет прыгать к ошибкам.
type DiagnosticsScreen struct {
	BaseScreen

	projectPath string
	client      *core.Client

	running     bool
	err         error
	status      string
	diagnostics []DiagnosticEntry
	selected    int
	scroll      int

	lastRun      time.Time
	runDuration  time.Duration
	exitCode     int
	errorCount   int
	warningCount int
	infoCount    int

	includeNotes bool
	includeFixes bool

	cancel context.CancelFunc
}

// DiagnosticEntry представляет одну диагностику с нормализованными полями.
type DiagnosticEntry struct {
	Severity string
	Code     string
	Message  string
	File     string // отображаемый путь (относительный)
	AbsPath  string // абсолютный путь
	Line     int
	Column   int
	Notes    []string
	HasFixes bool
}

type diagnosticsResultMsg struct {
	entries  []DiagnosticEntry
	duration time.Duration
	exitCode int
	err      error
}

// NewDiagnosticsScreen создаёт экран диагностики.
func NewDiagnosticsScreen(projectPath string, client *core.Client) *DiagnosticsScreen {
	return &DiagnosticsScreen{
		BaseScreen:   NewBaseScreen("Diagnostics"),
		projectPath:  projectPath,
		client:       client,
		status:       "Diagnostics will run shortly…",
		selected:     0,
		scroll:       0,
		includeNotes: true,
		includeFixes: false,
	}
}

// Init запускает первоначальную диагностику.
func (ds *DiagnosticsScreen) Init() tea.Cmd {
	return ds.runDiagnostics()
}

// OnEnter повторно запускает диагностику при первом входе или по заверщению первого запуска.
func (ds *DiagnosticsScreen) OnEnter() tea.Cmd {
	if ds.running {
		return nil
	}
	return ds.runDiagnostics()
}

// Update обрабатывает сообщения.
func (ds *DiagnosticsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		ds.SetSize(m.Width, m.Height-1)
		return ds, nil
	case tea.KeyMsg:
		return ds.handleKey(m)
	case diagnosticsResultMsg:
		ds.running = false
		if ds.cancel != nil {
			ds.cancel = nil
		}

		if m.err != nil {
			if errors.Is(m.err, context.Canceled) {
				// Игнорируем отменённый запуск.
				return ds, nil
			}
			ds.err = m.err
			ds.status = fmt.Sprintf("Diagnostics failed: %v", m.err)
			ds.diagnostics = nil
			ds.errorCount, ds.warningCount, ds.infoCount = 0, 0, 0
			return ds, nil
		}

		ds.err = nil
		ds.diagnostics = m.entries
		ds.exitCode = m.exitCode
		ds.runDuration = m.duration
		ds.lastRun = time.Now()
		ds.status = ds.successStatus()
		ds.selected = 0
		ds.scroll = 0
		ds.recountSeverities()
		return ds, nil
	}

	return ds, nil
}

func (ds *DiagnosticsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	if ds.running {
		switch msg.String() {
		case "esc":
			ds.cancelRunning()
			return ds, nil
		case "f5", "ctrl+r":
			return ds, nil
		}
	}

	switch msg.String() {
	case "f5", "ctrl+r":
		return ds, ds.runDiagnostics()
	case "up", "k":
		ds.moveSelection(-1)
	case "down", "j":
		ds.moveSelection(1)
	case "pgup", "ctrl+u":
		ds.moveSelection(-ds.pageSize())
	case "pgdown", "ctrl+d":
		ds.moveSelection(ds.pageSize())
	case "home", "g":
		ds.setSelection(0)
	case "end", "G":
		ds.setSelection(len(ds.diagnostics) - 1)
	case "enter":
		return ds, ds.openSelectedLocation()
	case "n":
		ds.includeNotes = !ds.includeNotes
		ds.status = fmt.Sprintf("Notes %s", ternary(ds.includeNotes, "enabled", "hidden"))
	default:
		return ds, nil
	}

	return ds, nil
}

func (ds *DiagnosticsScreen) View() string {
	if ds.Width() == 0 {
		return "Initializing diagnostics view..."
	}
	return ds.render()
}

func (ds *DiagnosticsScreen) ShortHelp() string {
	return "F5 Run diag • ↑↓ Select • Enter Open • n Toggle notes"
}

func (ds *DiagnosticsScreen) FullHelp() []string {
	help := ds.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Diagnostics Screen:",
		"  F5 / Ctrl+R - Run diagnostics",
		"  ↑/↓ or j/k - Move selection",
		"  PgUp/PgDn - Scroll page",
		"  Enter - Open location in workspace",
		"  n - Toggle notes visibility",
		"  Esc - Cancel running diagnostics / back",
	}...)
	return help
}

func (ds *DiagnosticsScreen) runDiagnostics() tea.Cmd {
	if ds.client == nil {
		ds.err = errors.New("surge client not configured")
		ds.status = "Surge client unavailable"
		return nil
	}

	if ds.cancel != nil {
		ds.cancel()
		ds.cancel = nil
	}

	ds.running = true
	ds.err = nil
	ds.status = "Running diagnostics…"

	projectPath := ds.projectPath
	includeNotes := ds.includeNotes
	includeFixes := ds.includeFixes
	client := ds.client

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	ds.cancel = cancel
	start := time.Now()

	return func() tea.Msg {
		defer cancel()
		resp, err := client.Diagnose(ctx, projectPath, includeNotes, includeFixes)
		duration := time.Since(start)
		var entries []DiagnosticEntry
		exitCode := 0
		if resp != nil {
			entries = ds.normalizeResponse(resp)
			exitCode = resp.ExitCode
		}
		return diagnosticsResultMsg{
			entries:  entries,
			duration: duration,
			exitCode: exitCode,
			err:      err,
		}
	}
}

func (ds *DiagnosticsScreen) normalizeResponse(resp *core.DiagResponse) []DiagnosticEntry {
	var entries []DiagnosticEntry
	if resp == nil {
		return entries
	}

	appendEntry := func(displayPath string, out core.DiagnosticsOutput) {
		for _, diag := range out.Diagnostics {
			entry := DiagnosticEntry{
				Severity: strings.ToLower(diag.Severity),
				Code:     diag.Code,
				Message:  diag.Message,
				Notes:    nil,
				HasFixes: len(diag.Fixes) > 0,
			}

			filePath := diag.Location.File
			if filePath == "" {
				filePath = displayPath
			}
			if filePath == "" {
				filePath = "unknown"
			}

			abs := filePath
			if !filepath.IsAbs(abs) && ds.projectPath != "" {
				abs = filepath.Join(ds.projectPath, filePath)
			}
			abs = filepath.Clean(abs)

			entry.AbsPath = abs

			// Build display path relative to project.
			display := filePath
			if filepath.IsAbs(display) && ds.projectPath != "" {
				if rel, err := filepath.Rel(ds.projectPath, abs); err == nil {
					display = rel
				} else {
					display = filepath.Base(abs)
				}
			}
			entry.File = display

			entry.Line = clampInt(int(diag.Location.StartLine), 1, 1<<31-1)
			entry.Column = clampInt(int(diag.Location.StartCol), 1, 1<<31-1)
			if entry.Line == 0 {
				entry.Line = clampInt(int(diag.Location.EndLine), 1, 1<<31-1)
			}
			if entry.Column == 0 {
				entry.Column = clampInt(int(diag.Location.EndCol), 1, 1<<31-1)
			}

			if len(diag.Notes) > 0 && ds.includeNotes {
				for _, note := range diag.Notes {
					if note.Message != "" {
						entry.Notes = append(entry.Notes, note.Message)
					}
				}
			}

			entries = append(entries, entry)
		}
	}

	if len(resp.Batch) > 0 {
		for path, out := range resp.Batch {
			appendEntry(path, out)
		}
	} else if resp.Single != nil {
		appendEntry("", *resp.Single)
	}

	sortDiagnostics(entries)
	return entries
}

func (ds *DiagnosticsScreen) moveSelection(delta int) {
	if len(ds.diagnostics) == 0 {
		ds.selected = 0
		ds.scroll = 0
		return
	}
	ds.selected = clampInt(ds.selected+delta, 0, len(ds.diagnostics)-1)
	ds.ensureSelectionVisible()
}

func (ds *DiagnosticsScreen) setSelection(index int) {
	if len(ds.diagnostics) == 0 {
		ds.selected = 0
		ds.scroll = 0
		return
	}
	ds.selected = clampInt(index, 0, len(ds.diagnostics)-1)
	ds.ensureSelectionVisible()
}

func (ds *DiagnosticsScreen) ensureSelectionVisible() {
	visible := ds.listHeight()
	if visible <= 0 {
		ds.scroll = 0
		return
	}
	if ds.selected < ds.scroll {
		ds.scroll = ds.selected
	} else if ds.selected >= ds.scroll+visible {
		ds.scroll = ds.selected - visible + 1
	}
	if ds.scroll < 0 {
		ds.scroll = 0
	}
	maxScroll := len(ds.diagnostics) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ds.scroll > maxScroll {
		ds.scroll = maxScroll
	}
}

func (ds *DiagnosticsScreen) cancelRunning() {
	if ds.cancel != nil {
		ds.cancel()
		ds.cancel = nil
		ds.status = "Diagnostics cancelled"
		ds.running = false
	}
}

func (ds *DiagnosticsScreen) openSelectedLocation() tea.Cmd {
	if len(ds.diagnostics) == 0 || ds.selected < 0 || ds.selected >= len(ds.diagnostics) {
		return nil
	}
	entry := ds.diagnostics[ds.selected]
	abs := entry.AbsPath
	if abs == "" {
		return nil
	}
	line := entry.Line
	if line <= 0 {
		line = 1
	}
	column := entry.Column
	if column <= 0 {
		column = 1
	}
	return func() tea.Msg {
		return OpenLocationMsg{
			FilePath: abs,
			Line:     line,
			Column:   column,
		}
	}
}

func (ds *DiagnosticsScreen) pageSize() int {
	size := ds.listHeight()
	if size <= 2 {
		return 1
	}
	return size - 2
}

func (ds *DiagnosticsScreen) listHeight() int {
	h := ds.Height()
	if h <= 0 {
		return 0
	}
	detail := 6
	header := 5
	height := h - detail - header
	if height < 3 {
		height = 3
	}
	return height
}

func (ds *DiagnosticsScreen) successStatus() string {
	if len(ds.diagnostics) == 0 {
		return "No diagnostics reported"
	}
	return fmt.Sprintf("Diagnostics completed: %d issues (errors:%d warnings:%d)", len(ds.diagnostics), ds.errorCount, ds.warningCount)
}

func (ds *DiagnosticsScreen) recountSeverities() {
	var errorsCount, warningsCount, infosCount int
	for _, diag := range ds.diagnostics {
		switch strings.ToLower(diag.Severity) {
		case "error":
			errorsCount++
		case "warning":
			warningsCount++
		default:
			infosCount++
		}
	}
	ds.errorCount = errorsCount
	ds.warningCount = warningsCount
	ds.infoCount = infosCount
}

func sortDiagnostics(entries []DiagnosticEntry) {
	if len(entries) <= 1 {
		return
	}
	severityRank := map[string]int{
		"error":   0,
		"warning": 1,
		"note":    2,
		"info":    3,
		"":        4,
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		rankA := severityRank[strings.ToLower(a.Severity)]
		rankB := severityRank[strings.ToLower(b.Severity)]
		if rankA != rankB {
			return rankA < rankB
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Column != b.Column {
			return a.Column < b.Column
		}
		return a.Message < b.Message
	})
}

func (ds *DiagnosticsScreen) summaryCounts() string {
	return fmt.Sprintf("Errors: %d  Warnings: %d  Info: %d", ds.errorCount, ds.warningCount, ds.infoCount)
}

// SetProjectPath обновляет путь к проекту для последующих запусков.
func (ds *DiagnosticsScreen) SetProjectPath(path string) {
	ds.projectPath = path
}

// TriggerDiagnostics запускает диагностику вручную.
func (ds *DiagnosticsScreen) TriggerDiagnostics() tea.Cmd {
	if ds.running {
		return nil
	}
	return ds.runDiagnostics()
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func ternary(condition bool, a, b string) string {
	if condition {
		return a
	}
	return b
}
