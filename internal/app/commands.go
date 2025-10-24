package app

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"surge-tui/internal/platform"
)

// Command describes an executable action, optionally bound to a key and/or screen.
type Command struct {
	ID      string
	Title   string
	Key     string
	Screen  *ScreenType // nil → global
	Enabled func(*App) bool
	Run     func(*App) tea.Cmd
}

// CommandRegistry stores commands and resolves them by key and screen.
type CommandRegistry struct {
	byID  map[string]*Command
	byKey map[string][]*Command
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		byID:  make(map[string]*Command),
		byKey: make(map[string][]*Command),
	}
}

func (r *CommandRegistry) Register(cmd *Command) {
	if cmd == nil || cmd.ID == "" {
		return
	}
	r.byID[cmd.ID] = cmd
	if cmd.Key != "" {
		canonical := platform.CanonicalKeyForLookup(cmd.Key)
		if canonical == "" {
			canonical = cmd.Key
		}
		r.byKey[canonical] = append(r.byKey[canonical], cmd)
	}
}

// Resolve returns the first matching command for key and screen.
func (r *CommandRegistry) Resolve(key string, screen ScreenType) *Command {
	canonical := platform.CanonicalKeyForLookup(key)
	if canonical == "" {
		canonical = key
	}
	cmds := r.byKey[canonical]
	if len(cmds) == 0 {
		return nil
	}
	// Prefer screen-specific command, fall back to global
	var global *Command
	for _, c := range cmds {
		if c.Screen == nil {
			global = c
			continue
		}
		if *c.Screen == screen {
			return c
		}
	}
	return global
}

// Get returns command by id.
func (r *CommandRegistry) Get(id string) *Command {
	if r == nil {
		return nil
	}
	return r.byID[id]
}

// All returns commands sorted by title.
func (r *CommandRegistry) All() []*Command {
	list := make([]*Command, 0, len(r.byID))
	for _, cmd := range r.byID {
		list = append(list, cmd)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Title < list[j].Title
	})
	return list
}

// Run executes command by id if enabled.
func (r *CommandRegistry) Run(id string, app *App) tea.Cmd {
	cmd := r.Get(id)
	if cmd == nil {
		return nil
	}
	if cmd.Enabled != nil && !cmd.Enabled(app) {
		return nil
	}
	if cmd.Run == nil {
		return nil
	}
	return cmd.Run(app)
}
