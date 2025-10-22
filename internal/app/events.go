package app

import (
	"sync"
)

// Event представляет событие в системе
type Event interface {
	Type() string
	Data() interface{}
}

// EventHandler обработчик события
type EventHandler func(Event)

// EventBus шина событий для связи между компонентами
type EventBus struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

// NewEventBus создает новую шину событий
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// Subscribe подписывается на событие
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// Publish публикует событие
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	handlers := eb.handlers[event.Type()]
	eb.mu.RUnlock()

	// Запускаем обработчики в отдельных горутинах
	for _, handler := range handlers {
		go handler(event)
	}
}

// Unsubscribe отписывается от всех обработчиков события
func (eb *EventBus) Unsubscribe(eventType string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.handlers, eventType)
}

// Базовые типы событий

// BaseEvent базовая реализация события
type BaseEvent struct {
	EventType string
	EventData interface{}
}

func (e BaseEvent) Type() string      { return e.EventType }
func (e BaseEvent) Data() interface{} { return e.EventData }

// Конкретные события

// FileChangedEvent файл был изменен
type FileChangedEvent struct {
	BaseEvent
	FilePath string
	Action   string // "created", "modified", "deleted"
}

// NewFileChangedEvent создает событие изменения файла
func NewFileChangedEvent(filePath, action string) *FileChangedEvent {
	return &FileChangedEvent{
		BaseEvent: BaseEvent{
			EventType: "file.changed",
			EventData: map[string]string{"path": filePath, "action": action},
		},
		FilePath: filePath,
		Action:   action,
	}
}

// ProjectOpenedEvent проект был открыт
type ProjectOpenedEvent struct {
	BaseEvent
	ProjectPath string
}

// NewProjectOpenedEvent создает событие открытия проекта
func NewProjectOpenedEvent(projectPath string) *ProjectOpenedEvent {
	return &ProjectOpenedEvent{
		BaseEvent: BaseEvent{
			EventType: "project.opened",
			EventData: projectPath,
		},
		ProjectPath: projectPath,
	}
}

// BuildCompletedEvent сборка завершена
type BuildCompletedEvent struct {
	BaseEvent
	Success     bool
	Diagnostics []interface{} // TODO: определить тип диагностик
}

// NewBuildCompletedEvent создает событие завершения сборки
func NewBuildCompletedEvent(success bool, diagnostics []interface{}) *BuildCompletedEvent {
	return &BuildCompletedEvent{
		BaseEvent: BaseEvent{
			EventType: "build.completed",
			EventData: map[string]interface{}{
				"success":     success,
				"diagnostics": diagnostics,
			},
		},
		Success:     success,
		Diagnostics: diagnostics,
	}
}

// EditorFileOpenedEvent файл открыт в редакторе
type EditorFileOpenedEvent struct {
	BaseEvent
	FilePath string
}

// NewEditorFileOpenedEvent создает событие открытия файла в редакторе
func NewEditorFileOpenedEvent(filePath string) *EditorFileOpenedEvent {
	return &EditorFileOpenedEvent{
		BaseEvent: BaseEvent{
			EventType: "editor.file.opened",
			EventData: filePath,
		},
		FilePath: filePath,
	}
}

// EditorContentChangedEvent содержимое файла изменено в редакторе
type EditorContentChangedEvent struct {
	BaseEvent
	FilePath string
	Dirty    bool
}

// NewEditorContentChangedEvent создает событие изменения содержимого в редакторе
func NewEditorContentChangedEvent(filePath string, dirty bool) *EditorContentChangedEvent {
	return &EditorContentChangedEvent{
		BaseEvent: BaseEvent{
			EventType: "editor.content.changed",
			EventData: map[string]interface{}{
				"path":  filePath,
				"dirty": dirty,
			},
		},
		FilePath: filePath,
		Dirty:    dirty,
	}
}