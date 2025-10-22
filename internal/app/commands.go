package app

import (
    tea "github.com/charmbracelet/bubbletea"
)

// Command describes an executable action, optionally bound to a key and/or screen.
type Command struct {
    ID       string
    Title    string
    Key      string
    Screen   *ScreenType // nil → global
    Enabled  func(*App) bool
    Run      func(*App) tea.Cmd
}

// CommandRegistry stores commands and resolves them by key and screen.
type CommandRegistry struct {
    byID map[string]*Command
    byKey map[string][]*Command
}

func NewCommandRegistry() *CommandRegistry {
    return &CommandRegistry{
        byID: make(map[string]*Command),
        byKey: make(map[string][]*Command),
    }
}

func (r *CommandRegistry) Register(cmd *Command) {
    if cmd == nil || cmd.ID == "" {
        return
    }
    r.byID[cmd.ID] = cmd
    if cmd.Key != "" {
        r.byKey[cmd.Key] = append(r.byKey[cmd.Key], cmd)
    }
}

// Resolve returns the first matching command for key and screen.
func (r *CommandRegistry) Resolve(key string, screen ScreenType) *Command {
    cmds := r.byKey[key]
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

