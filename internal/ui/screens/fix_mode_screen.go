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

	"surge-tui/internal/core/surge"
	"surge-tui/internal/platform"
	"surge-tui/internal/ui/components"
)

// FixModeScreen отображает доступные авто-фиксы и позволяет их применять.
type FixModeScreen struct {
	BaseScreen

	projectPath string
	client      *surge.Client

	loading bool
	err     error

	entries  []fixEntry
	selected int
	scroll   int

	includeSuggested bool

	statusMsg string
	statusAt  time.Time

	confirm *components.ConfirmDialog

	previewCache map[string]*diffPreview

	pendingFocus *fixFocusRequest

	cancel context.CancelFunc
}

type fixEntry struct {
	FilePath   string
	Diagnostic surge.DiagnosticJSON
	Fix        surge.FixJSON
}

type fixesLoadedMsg struct {
	entries []fixEntry
	err     error
}

type fixAppliedMsg struct {
	err   error
	count int // 1 для одиночного, >=0 для количества, -1 неизвестно
}

type fixApplyAllMsg struct {
	confirmed bool
}

type diffPreview struct {
	Diff string
	Err  error
}

type fixFocusRequest struct {
	File  string
	FixID string
}

const (
	fixDiffAddColor  = "#22C55E"
	fixDiffDelColor  = "#F87171"
	fixDiffMetaColor = "#94A3B8"
	fixDiffWarnColor = "#FBBF24"
)

// NewFixModeScreen создаёт новый экран Fix Mode.
func NewFixModeScreen(projectPath string, client *surge.Client) *FixModeScreen {
	dialog := components.NewConfirmDialog("Apply All Fixes", "Apply all available fixes? This cannot be undone.")
	dialog.ConfirmText = "Apply"
	dialog.CancelText = "Cancel"

	return &FixModeScreen{
		BaseScreen:       NewBaseScreen("Fix Mode"),
		projectPath:      projectPath,
		client:           client,
		includeSuggested: true,
		selected:         0,
		scroll:           0,
		confirm:          dialog,
		previewCache:     make(map[string]*diffPreview),
	}
}

// Init запускает первоначальную загрузку.
func (fs *FixModeScreen) Init() tea.Cmd {
	return fs.loadFixes()
}

// OnEnter перезагружает фиксы при возврате на экран.
func (fs *FixModeScreen) OnEnter() tea.Cmd {
	return fs.loadFixes()
}

// Update обрабатывает сообщения.
func (fs *FixModeScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if fs.confirm != nil && fs.confirm.Visible {
		if cmd := fs.confirm.Update(msg); cmd != nil {
			return fs, cmd
		}
		return fs, nil
	}

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		fs.SetSize(m.Width, m.Height-1)
		return fs, nil
	case tea.KeyMsg:
		return fs.handleKey(m)
	case fixesLoadedMsg:
		fs.loading = false
		if fs.cancel != nil {
			fs.cancel = nil
		}
		if m.err != nil {
			fs.err = m.err
			fs.entries = nil
			fs.selected = 0
			fs.scroll = 0
			fs.previewCache = make(map[string]*diffPreview)
		} else {
			fs.err = nil
			fs.entries = m.entries
			fs.previewCache = make(map[string]*diffPreview)
			if fs.selected >= len(fs.entries) {
				fs.selected = len(fs.entries) - 1
			}
			if fs.selected < 0 {
				fs.selected = 0
			}
			fs.ensureSelectionVisible()
			fs.setStatus("Fix list updated")
			if fs.pendingFocus != nil && fs.applyFocus(*fs.pendingFocus) {
				fs.pendingFocus = nil
			}
		}
		return fs, nil
	case fixAppliedMsg:
		if fs.cancel != nil {
			fs.cancel = nil
		}
		if m.err != nil {
			fs.setStatus(fmt.Sprintf("Failed to apply fix: %v", m.err))
			return fs, nil
		}
		if m.count < 0 {
			fs.setStatus("Applied all fixes")
		} else if m.count <= 1 {
			fs.setStatus("Fix applied")
		} else {
			fs.setStatus(fmt.Sprintf("Applied %d fixes", m.count))
		}
		return fs, fs.loadFixes()
	case fixApplyAllMsg:
		if !m.confirmed {
			fs.setStatus("Cancelled")
			return fs, nil
		}
		return fs, fs.applyAll()
	}

	return fs, nil
}

// View рендерит экран.
func (fs *FixModeScreen) View() string {
	if fs.Width() == 0 {
		return "Initializing Fix Mode..."
	}
	if fs.loading {
		return fs.renderLoading()
	}
	if fs.err != nil {
		return fs.renderError()
	}
	if len(fs.entries) == 0 {
		return fs.renderEmpty()
	}
	return fs.renderContent()
}

func (fs *FixModeScreen) ShortHelp() string {
	return platform.ReplacePrimaryModifier("↑↓ Navigate • Enter Preview • a Apply • A Apply All • Ctrl+R Refresh")
}

func (fs *FixModeScreen) FullHelp() []string {
	help := fs.BaseScreen.FullHelp()
	help = append(help, []string{
		"",
		"Fix Mode:",
		"  ↑/↓ or j/k - Navigate fixes",
		"  PgUp/PgDn - Page",
		"  Enter - Preview details",
		"  a - Apply selected fix",
		"  A - Apply all fixes",
		platform.ReplacePrimaryModifier("  Ctrl+R - Reload"),
		"  Tab - Toggle suggested fixes",
	}...)
	return help
}

func (fs *FixModeScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := platform.CanonicalKeyForLookup(msg.String())
	if fs.loading {
		switch key {
		case "ctrl+r":
			return fs, fs.loadFixes()
		}
		return fs, nil
	}

	switch key {
	case "ctrl+r":
		return fs, fs.loadFixes()
	case "up", "k":
		fs.moveSelection(-1)
	case "down", "j":
		fs.moveSelection(1)
	case "pgup", "ctrl+u":
		fs.moveSelection(-fs.pageSize())
	case "pgdown", "ctrl+d":
		fs.moveSelection(fs.pageSize())
	case "home", "g":
		fs.setSelection(0)
	case "end", "G":
		fs.setSelection(len(fs.entries) - 1)
	case "enter":
		// Preview is generated on render, so nothing extra for now.
		return fs, nil
	case "a":
		return fs, fs.applySelected()
	case "A":
		if fs.confirm != nil {
			fs.confirm.Description = "Apply all available fixes in project?"
			ch := fs.confirm.Show()
			return fs, func() tea.Msg {
				confirmed := <-ch
				return fixApplyAllMsg{confirmed: confirmed}
			}
		}
		return fs, fs.applyAll()
	case "tab":
		fs.includeSuggested = !fs.includeSuggested
		if fs.includeSuggested {
			fs.setStatus("Showing suggested fixes")
		} else {
			fs.setStatus("Only safe fixes")
		}
		return fs, fs.loadFixes()
	}

	return fs, nil
}

func (fs *FixModeScreen) loadFixes() tea.Cmd {
	if fs.client == nil {
		fs.err = errors.New("surge client not configured")
		return nil
	}
	if fs.cancel != nil {
		fs.cancel()
		fs.cancel = nil
	}
	fs.loading = true
	fs.err = nil

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	fs.cancel = cancel
	projectPath := fs.projectPath
	includeSuggested := fs.includeSuggested
	client := fs.client

	return func() tea.Msg {
		defer cancel()
		resp, err := client.Diagnose(ctx, projectPath, true, true)
		if err != nil {
			return fixesLoadedMsg{err: err}
		}
		entries := buildFixEntries(resp, includeSuggested)
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].FilePath != entries[j].FilePath {
				return entries[i].FilePath < entries[j].FilePath
			}
			if entries[i].Diagnostic.Code != entries[j].Diagnostic.Code {
				return entries[i].Diagnostic.Code < entries[j].Diagnostic.Code
			}
			return entries[i].Fix.Title < entries[j].Fix.Title
		})
		return fixesLoadedMsg{entries: entries}
	}
}

func buildFixEntries(resp *surge.DiagResponse, includeSuggested bool) []fixEntry {
	if resp == nil {
		return nil
	}
	var entries []fixEntry

	appendDiag := func(filePath string, out surge.DiagnosticsOutput) {
		for _, diag := range out.Diagnostics {
			if len(diag.Fixes) == 0 {
				continue
			}
			for _, fix := range diag.Fixes {
				if !includeSuggested && strings.EqualFold(fix.Applicability, "suggested") {
					continue
				}
				entry := fixEntry{
					FilePath:   choosePath(filePath, diag.Location.File),
					Diagnostic: diag,
					Fix:        fix,
				}
				entries = append(entries, entry)
			}
		}
	}

	if len(resp.Batch) > 0 {
		for path, out := range resp.Batch {
			appendDiag(path, out)
		}
	} else if resp.Single != nil {
		appendDiag("", *resp.Single)
	}

	return entries
}

func choosePath(display, reported string) string {
	path := reported
	if path == "" {
		path = display
	}
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

// SetProjectPath обновляет путь проекта, используемый экраном.
func (fs *FixModeScreen) SetProjectPath(path string) {
	fs.projectPath = path
}

func (fs *FixModeScreen) applySelected() tea.Cmd {
	if fs.client == nil || len(fs.entries) == 0 {
		return nil
	}
	entry := fs.entries[fs.selected]
	if entry.Fix.ID == "" {
		fs.setStatus("Fix has no ID")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	fs.cancel = cancel
	client := fs.client
	filePath := entry.FilePath
	if !filepath.IsAbs(filePath) {
		if abs, err := filepath.Abs(filePath); err == nil {
			filePath = abs
		}
	}
	fixID := entry.Fix.ID

	fs.setStatus("Applying fix...")
	return func() tea.Msg {
		defer cancel()
		err := client.ApplyFixByID(ctx, filePath, fixID)
		return fixAppliedMsg{err: err, count: 1}
	}
}

func (fs *FixModeScreen) applyAll() tea.Cmd {
	if fs.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	fs.cancel = cancel
	client := fs.client
	projectPath := fs.projectPath
	if projectPath == "" && len(fs.entries) > 0 {
		projectPath = filepath.Dir(fs.entries[0].FilePath)
	}

	fs.setStatus("Applying all fixes...")
	return func() tea.Msg {
		defer cancel()
		err := client.ApplyAllFixes(ctx, projectPath)
		return fixAppliedMsg{err: err, count: -1}
	}
}

// HasEntries reports whether there are fixes to interact with.
func (fs *FixModeScreen) HasEntries() bool {
	return len(fs.entries) > 0
}

// RefreshCmd triggers a reload of available fixes.
func (fs *FixModeScreen) RefreshCmd() tea.Cmd {
	return fs.loadFixes()
}

// ApplySelectedCmd applies the currently selected fix.
func (fs *FixModeScreen) ApplySelectedCmd() tea.Cmd {
	return fs.applySelected()
}

// ApplyAllCmd starts confirmation flow for applying all fixes (falls back to direct apply when no dialog).
func (fs *FixModeScreen) ApplyAllCmd() tea.Cmd {
	if fs.confirm != nil {
		fs.confirm.Description = "Apply all available fixes in project?"
		ch := fs.confirm.Show()
		return func() tea.Msg {
			confirmed := <-ch
			return fixApplyAllMsg{confirmed: confirmed}
		}
	}
	return fs.applyAll()
}

// ToggleSuggestedCmd toggles inclusion of suggested fixes and refreshes the list.
func (fs *FixModeScreen) ToggleSuggestedCmd() tea.Cmd {
	fs.includeSuggested = !fs.includeSuggested
	if fs.includeSuggested {
		fs.setStatus("Showing suggested fixes")
	} else {
		fs.setStatus("Only safe fixes")
	}
	return fs.loadFixes()
}

func (fs *FixModeScreen) FocusFix(filePath, fixID string) {
	if filePath == "" && fixID == "" {
		return
	}
	cleanFile := filePath
	if cleanFile != "" {
		cleanFile = filepath.Clean(cleanFile)
	}
	req := fixFocusRequest{
		File:  cleanFile,
		FixID: fixID,
	}
	if fs.applyFocus(req) {
		fs.pendingFocus = nil
		return
	}
	fs.pendingFocus = &req
}

func (fs *FixModeScreen) applyFocus(req fixFocusRequest) bool {
	if len(fs.entries) == 0 {
		return false
	}
	cleanFile := req.File
	if cleanFile != "" {
		cleanFile = filepath.Clean(cleanFile)
	}
	index := -1
	if req.FixID != "" {
		for i, entry := range fs.entries {
			if strings.EqualFold(entry.Fix.ID, req.FixID) {
				if cleanFile == "" || samePath(entry.FilePath, cleanFile) {
					index = i
					break
				}
			}
		}
	}
	if index == -1 && cleanFile != "" {
		for i, entry := range fs.entries {
			if samePath(entry.FilePath, cleanFile) {
				index = i
				break
			}
		}
	}
	if index == -1 {
		return false
	}
	fs.setSelection(index)
	fs.ensureSelectionVisible()
	return true
}

func (fs *FixModeScreen) moveSelection(delta int) {
	if len(fs.entries) == 0 {
		fs.selected = 0
		fs.scroll = 0
		return
	}
	fs.selected += delta
	if fs.selected < 0 {
		fs.selected = 0
	}
	if fs.selected >= len(fs.entries) {
		fs.selected = len(fs.entries) - 1
	}
	fs.ensureSelectionVisible()
}

func (fs *FixModeScreen) setSelection(index int) {
	if len(fs.entries) == 0 {
		fs.selected = 0
		fs.scroll = 0
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(fs.entries) {
		index = len(fs.entries) - 1
	}
	fs.selected = index
	fs.ensureSelectionVisible()
}

func (fs *FixModeScreen) pageSize() int {
	h := fs.listHeight()
	if h <= 1 {
		return 1
	}
	return h - 1
}

func (fs *FixModeScreen) ensureSelectionVisible() {
	visible := fs.listHeight()
	if visible <= 0 {
		fs.scroll = 0
		return
	}
	if fs.selected < fs.scroll {
		fs.scroll = fs.selected
	} else if fs.selected >= fs.scroll+visible {
		fs.scroll = fs.selected - visible + 1
	}
	if fs.scroll < 0 {
		fs.scroll = 0
	}
	maxScroll := max(len(fs.entries)-visible, 0)
	if fs.scroll > maxScroll {
		fs.scroll = maxScroll
	}
}

func (fs *FixModeScreen) setStatus(msg string) {
	fs.statusMsg = msg
	fs.statusAt = time.Now()
}

func (fs *FixModeScreen) statusLine() string {
	if fs.statusMsg == "" {
		return ""
	}
	if time.Since(fs.statusAt) > 4*time.Second {
		fs.statusMsg = ""
		return ""
	}
	return fs.statusMsg
}

func (fs *FixModeScreen) listHeight() int {
	h := fs.Height()
	if h <= 0 {
		return 0
	}
	// Reserve space for borders and preview
	height := h - 8
	if height < 3 {
		height = 3
	}
	return height
}
