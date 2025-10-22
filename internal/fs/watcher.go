package fs

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher следит за изменениями файлов
type FileWatcher struct {
	watcher   *fsnotify.Watcher
	callbacks map[string][]FileChangeCallback
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// FileChangeCallback функция обратного вызова для изменений файлов
type FileChangeCallback func(event FileChangeEvent)

// FileChangeEvent событие изменения файла
type FileChangeEvent struct {
	Path      string        // Путь к файлу
	Operation FileOperation // Тип операции
	IsDir     bool          // Является ли директорией
}

// FileOperation тип операции с файлом
type FileOperation int

const (
	FileCreated FileOperation = iota
	FileModified
	FileDeleted
	FileRenamed
)

// NewFileWatcher создает новый наблюдатель за файлами
func NewFileWatcher(ctx context.Context) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	fw := &FileWatcher{
		watcher:   watcher,
		callbacks: make(map[string][]FileChangeCallback),
		ctx:       ctx,
		cancel:    cancel,
	}

	go fw.watchLoop()

	return fw, nil
}

// Watch начинает наблюдение за директорией
func (fw *FileWatcher) Watch(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	return fw.watcher.Add(path)
}

// Unwatch прекращает наблюдение за директорией
func (fw *FileWatcher) Unwatch(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	return fw.watcher.Remove(path)
}

// AddCallback добавляет обработчик для событий в указанной директории
func (fw *FileWatcher) AddCallback(path string, callback FileChangeCallback) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	fw.callbacks[path] = append(fw.callbacks[path], callback)
}

// RemoveCallbacks удаляет все обработчики для указанной директории
func (fw *FileWatcher) RemoveCallbacks(path string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	delete(fw.callbacks, path)
}

// Close закрывает наблюдатель
func (fw *FileWatcher) Close() error {
	fw.cancel()
	return fw.watcher.Close()
}

// watchLoop главный цикл наблюдения
func (fw *FileWatcher) watchLoop() {
	for {
		select {
		case <-fw.ctx.Done():
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			// TODO: логировать ошибку
			_ = err
		}
	}
}

// handleEvent обрабатывает событие изменения файла
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	changeEvent := fw.convertEvent(event)

	fw.mu.RLock()
	defer fw.mu.RUnlock()

	// Ищем обработчики для этого пути и его родительских директорий
	for watchPath, callbacks := range fw.callbacks {
		if fw.pathMatches(event.Name, watchPath) {
			for _, callback := range callbacks {
				go callback(changeEvent)
			}
		}
	}
}

// convertEvent конвертирует fsnotify.Event в FileChangeEvent
func (fw *FileWatcher) convertEvent(event fsnotify.Event) FileChangeEvent {
	var operation FileOperation

	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		operation = FileCreated
	case event.Op&fsnotify.Write == fsnotify.Write:
		operation = FileModified
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		operation = FileDeleted
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		operation = FileRenamed
	default:
		operation = FileModified
	}

	return FileChangeEvent{
		Path:      event.Name,
		Operation: operation,
		IsDir:     fw.isDirectory(event.Name),
	}
}

// pathMatches проверяет, соответствует ли путь файла наблюдаемой директории
func (fw *FileWatcher) pathMatches(filePath, watchPath string) bool {
	// Проверяем, находится ли файл в наблюдаемой директории или её поддиректориях
	rel, err := filepath.Rel(watchPath, filePath)
	if err != nil {
		return false
	}

	// Если путь начинается с "..", то файл находится вне наблюдаемой директории
	return !filepath.IsAbs(rel) && rel != ".." && rel != "."
}

// isDirectory проверяет, является ли путь директорией
func (fw *FileWatcher) isDirectory(path string) bool {
	// Эта функция может быть не точной для удаленных файлов
	// В идеале нужно кешировать информацию о типах файлов
	// TODO: улучшить определение типа файла
	return false
}

// String возвращает строковое представление операции
func (op FileOperation) String() string {
	switch op {
	case FileCreated:
		return "created"
	case FileModified:
		return "modified"
	case FileDeleted:
		return "deleted"
	case FileRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}
