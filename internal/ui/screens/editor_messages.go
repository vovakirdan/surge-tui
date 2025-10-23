package screens

import "time"

type editorFileLoadedMsg struct {
	Path  string
	Lines []string
	Stats editorStats
}

type editorFileErrorMsg struct {
	Path string
	Err  error
}

func (es *EditorScreen) viewportPosition() (current int, total int) {
	total = len(es.lines)
	current = min(es.scroll + 1, total)
	return
}

// Helpers for external status updates
func (es *EditorScreen) lastStatus() (string, time.Time) {
	return es.status, es.statusAt
}
