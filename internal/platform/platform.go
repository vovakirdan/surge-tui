package platform

import (
	"runtime"
	"sort"
	"strings"
	"unicode"
)

// IsMac reports whether we run on macOS (darwin).
func IsMac() bool {
	return runtime.GOOS == "darwin"
}

// PrimaryModifierKey returns the canonical lower-case modifier name used in keybindings.
func PrimaryModifierKey() string {
	if IsMac() {
		return "cmd"
	}
	return "ctrl"
}

// PrimaryModifierDisplay returns the human-readable modifier name for hints.
func PrimaryModifierDisplay() string {
	if IsMac() {
		return "Cmd"
	}
	return "Ctrl"
}

// ReplacePrimaryModifier swaps Ctrl with Cmd in the provided text when on macOS.
func ReplacePrimaryModifier(text string) string {
	if !IsMac() || text == "" {
		return text
	}
	replacer := strings.NewReplacer(
		"Ctrl+", "Cmd+",
		"ctrl+", "cmd+",
		"CTRL+", "CMD+",
	)
	return replacer.Replace(text)
}

var modifierOrder = map[string]int{
	"ctrl":   0,
	"cmd":    0, // treated same as ctrl for ordering
	"super":  1,
	"alt":    2,
	"option": 2,
	"meta":   2,
	"shift":  3,
}

// CanonicalKeyForLookup normalizes key descriptions so different aliases resolve consistently.
// It sorts modifiers in a stable order and maps cmd→ctrl so that mac bindings work on terminals
// which don't forward the command key.
func CanonicalKeyForLookup(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	mods, main := splitKey(key)
	if len(mods) == 0 && len(main) == 0 {
		return ""
	}

	for i, mod := range mods {
		switch mod {
		case "cmd", "command", "⌘":
			// Terminals typically deliver command as ctrl, keep lookup consistent.
			mods[i] = "ctrl"
		case "control":
			mods[i] = "ctrl"
		case "option", "opt", "⌥":
			mods[i] = "alt"
		case "meta", "win", "windows":
			mods[i] = "super"
		default:
			mods[i] = mod
		}
	}

	mods = uniqueStrings(mods)
	sort.SliceStable(mods, func(i, j int) bool {
		ai := modifierOrderValue(mods[i])
		aj := modifierOrderValue(mods[j])
		if ai == aj {
			return mods[i] < mods[j]
		}
		return ai < aj
	})

	if len(main) == 0 {
		return strings.Join(mods, "+")
	}

	main = normalizeMainParts(main)
	return strings.Join(append(mods, strings.Join(main, "+")), "+")
}

// DisplayKey formats a key binding for UI hints with platform-friendly modifier names.
func DisplayKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	parts := strings.Split(key, "+")
	display := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		plower := strings.ToLower(p)
		switch plower {
		case "ctrl", "control":
			if IsMac() {
				display = append(display, "Ctrl")
			} else {
				display = append(display, "Ctrl")
			}
		case "cmd", "command", "⌘":
			display = append(display, "Cmd")
		case "alt", "option", "opt", "⌥":
			if IsMac() {
				display = append(display, "Option")
			} else {
				display = append(display, "Alt")
			}
		case "shift", "⇧":
			display = append(display, "Shift")
		case "meta", "win", "windows":
			display = append(display, "Super")
		default:
			if len([]rune(p)) == 1 {
				display = append(display, strings.ToUpper(p))
			} else {
				runes := []rune(p)
				display = append(display, strings.ToUpper(string(runes[0]))+strings.ToLower(string(runes[1:])))
			}
		}
	}
	return strings.Join(display, "+")
}

// MatchesKey returns true if two key descriptions should be considered equivalent.
func MatchesKey(actual string, binding string) bool {
	return CanonicalKeyForLookup(actual) == CanonicalKeyForLookup(binding)
}

func splitKey(key string) (mods []string, main []string) {
	raw := strings.Split(key, "+")
	for _, part := range raw {
		if part == "" {
			continue
		}
		if part == " " {
			main = append(main, "space")
			continue
		}

		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch lower {
		case "ctrl", "control", "cmd", "command", "⌘", "alt", "option", "opt", "⌥", "shift", "⇧", "meta", "win", "windows", "super":
			mods = append(mods, lower)
		case "space":
			main = append(main, "space")
		default:
			if len([]rune(trimmed)) == 1 {
				r := []rune(trimmed)[0]
				if unicode.IsLetter(r) {
					main = append(main, strings.ToLower(string(r)))
					continue
				}
			}
			main = append(main, lower)
		}
	}
	return mods, main
}

func normalizeMainParts(parts []string) []string {
	if len(parts) == 0 {
		return parts
	}
	out := make([]string, len(parts))
	for i, part := range parts {
		switch part {
		case "command", "cmd", "⌘":
			out[i] = "cmd"
		case "control":
			out[i] = "ctrl"
		default:
			out[i] = part
		}
	}
	return out
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func modifierOrderValue(mod string) int {
	if v, ok := modifierOrder[mod]; ok {
		return v
	}
	return 10
}
