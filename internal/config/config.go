package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config конфигурация приложения
type Config struct {
	// Внешний вид
	Theme string `yaml:"theme"` // "dark" или "light"

	// Пути
	SurgeBinary    string `yaml:"surge_binary"`    // Путь к бинарю surge
	DefaultProject string `yaml:"default_project"` // Путь к проекту по умолчанию

	// Редактор
	Editor EditorConfig `yaml:"editor"`

	// Горячие клавиши
	Keybindings map[string]string `yaml:"keybindings"`

	// Производительность
	Performance PerformanceConfig `yaml:"performance"`

	// Логирование
	Logging LoggingConfig `yaml:"logging"`
}

// EditorConfig настройки редактора
type EditorConfig struct {
	TabSize         int    `yaml:"tab_size"`
	UseSpaces       bool   `yaml:"use_spaces"`
	AutoSave        bool   `yaml:"auto_save"`
	AutoSaveDelay   int    `yaml:"auto_save_delay"` // в секундах
	ExternalEditor  string `yaml:"external_editor"` // команда для внешнего редактора
	SyntaxHighlight bool   `yaml:"syntax_highlight"`
}

// PerformanceConfig настройки производительности
type PerformanceConfig struct {
	MaxFileSize   int64 `yaml:"max_file_size"`   // Максимальный размер файла в байтах
	MaxLogEntries int   `yaml:"max_log_entries"` // Максимальное количество записей в логе
	RefreshRate   int   `yaml:"refresh_rate"`    // Частота обновления UI в миллисекундах
	MemoryLimit   int64 `yaml:"memory_limit"`    // Лимит памяти в байтах
}

// LoggingConfig настройки логирования
type LoggingConfig struct {
	Level    string `yaml:"level"`     // debug, info, warn, error
	FilePath string `yaml:"file_path"` // Путь к файлу логов
	MaxSize  int64  `yaml:"max_size"`  // Максимальный размер файла логов в байтах
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		Theme:          "dark",
		SurgeBinary:    "surge", // Ищем в PATH
		DefaultProject: "",

		Editor: EditorConfig{
			TabSize:         4,
			UseSpaces:       true,
			AutoSave:        true,
			AutoSaveDelay:   30,
			ExternalEditor:  os.Getenv("EDITOR"),
			SyntaxHighlight: true,
		},

		Keybindings: map[string]string{
			"quit":               "ctrl+q",
			"command_palette":    "ctrl+p",
			"help":               "f1",
			"settings":           "ctrl+comma",
			"workspace":          "esc",
			"fix_mode":           "ctrl+f",
			"save":               "ctrl+s",
			"build":              "ctrl+b",
			"undo":               "ctrl+z",
			"redo":               "ctrl+y",
			"external_editor":    "ctrl+e",
			"switch_screen":      "tab",
			"switch_screen_back": "shift+tab",
			"init_project":       "ctrl+i",
		},

		Performance: PerformanceConfig{
			MaxFileSize:   10 * 1024 * 1024, // 10 MB
			MaxLogEntries: 1000,
			RefreshRate:   50,                // 20 FPS
			MemoryLimit:   512 * 1024 * 1024, // 512 MB
		},

		Logging: LoggingConfig{
			Level:    "info",
			FilePath: "",               // Будет определен автоматически
			MaxSize:  10 * 1024 * 1024, // 10 MB
		},
	}
}

// Load загружает конфигурацию из файла
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Определяем путь к конфигурационному файлу
	configPath, err := getConfigPath()
	if err != nil {
		return cfg, err // Возвращаем конфиг по умолчанию
	}

	// Если файл не существует, создаем его с настройками по умолчанию
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := cfg.Save(configPath); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	// Читаем файл конфигурации
	data, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, err
	}

	// Парсим YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, err
	}

	// Устанавливаем пути по умолчанию если они не заданы
	if cfg.Logging.FilePath == "" {
		cfg.Logging.FilePath = getDefaultLogPath()
	}

	// Заполняем отсутствующие привязки клавиш значениями по умолчанию
	defaults := DefaultConfig()
	cfg.applyKeybindingDefaults(defaults.Keybindings)

	// Валидация значений и нормализация дефолтов
	_ = cfg.Validate()

	return cfg, nil
}

func (c *Config) applyKeybindingDefaults(defaults map[string]string) {
	if defaults == nil {
		return
	}
	if c.Keybindings == nil {
		c.Keybindings = make(map[string]string, len(defaults))
	}
	for key, value := range defaults {
		current, ok := c.Keybindings[key]
		if !ok || strings.TrimSpace(current) == "" {
			c.Keybindings[key] = value
		}
	}
}

// Save сохраняет конфигурацию в файл
func (c *Config) Save(path string) error {
	// Создаем директорию если ее нет
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Маршалим в YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	// Записываем файл
	return os.WriteFile(path, data, 0644)
}

// SaveDefault сохраняет конфигурацию в стандартное место
func (c *Config) SaveDefault() error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	return c.Save(configPath)
}

// getConfigPath возвращает путь к конфигурационному файлу
func getConfigPath() (string, error) {
	// Пробуем получить XDG_CONFIG_HOME
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		// Используем ~/.config
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(configDir, "surge-tui", "config.yaml"), nil
}

// getDefaultLogPath возвращает путь к файлу логов по умолчанию
func getDefaultLogPath() string {
	// Пробуем получить XDG_CACHE_HOME
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		// Используем ~/.cache
		homeDir, _ := os.UserHomeDir()
		cacheDir = filepath.Join(homeDir, ".cache")
	}

	return filepath.Join(cacheDir, "surge-tui", "app.log")
}

// ValidateSurgeBinary проверяет доступность surge binary с таймаутом.
func (c *Config) ValidateSurgeBinary(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, c.SurgeBinary, "--version")
	return cmd.Run()
}

// Validate проверяет корректность конфигурации
func (c *Config) Validate() error {
	// Проверяем тему
	if c.Theme != "dark" && c.Theme != "light" {
		c.Theme = "dark"
	}

	// Проверяем размер табуляции
	if c.Editor.TabSize < 1 || c.Editor.TabSize > 16 {
		c.Editor.TabSize = 4
	}

	// Проверяем задержку автосохранения
	if c.Editor.AutoSaveDelay < 1 {
		c.Editor.AutoSaveDelay = 30
	}

	// Проверяем лимиты производительности
	if c.Performance.MaxFileSize < 1024 {
		c.Performance.MaxFileSize = 10 * 1024 * 1024
	}

	if c.Performance.RefreshRate < 10 || c.Performance.RefreshRate > 1000 {
		c.Performance.RefreshRate = 50
	}

	// Проверяем уровень логирования
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[c.Logging.Level] {
		c.Logging.Level = "info"
	}

	return nil
}
