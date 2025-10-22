package surge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Client клиент для взаимодействия с surge CLI
type Client struct {
	binaryPath string
	timeout    time.Duration
}

// NewClient создает новый клиент surge
func NewClient(binaryPath string) *Client {
	return &Client{
		binaryPath: binaryPath,
		timeout:    30 * time.Second, // По умолчанию 30 секунд
	}
}

// SetTimeout устанавливает таймаут для операций
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// CheckAvailable проверяет доступность surge CLI
func (c *Client) CheckAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.binaryPath, "--version")
	return cmd.Run()
}

// GetVersion возвращает версию surge
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// BuildProject запускает сборку проекта
func (c *Client) BuildProject(ctx context.Context, projectPath string) (*BuildResult, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, "build", "--format=json", projectPath)

	result := &BuildResult{
		ProjectPath: projectPath,
		StartTime:   time.Now(),
	}

	// Запускаем команду и читаем вывод
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return result, err
	}

	if err := cmd.Start(); err != nil {
		return result, err
	}

	// Читаем stdout для диагностик
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if diagnostic := c.parseDiagnostic(line); diagnostic != nil {
				result.Diagnostics = append(result.Diagnostics, *diagnostic)
			}
		}
	}()

	// Читаем stderr для ошибок
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			result.ErrorOutput = append(result.ErrorOutput, scanner.Text())
		}
	}()

	err = cmd.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Success = (err == nil)

	if err != nil {
		result.Error = err
	}

	return result, nil
}

// GetFixes получает список автофиксов для проекта
func (c *Client) GetFixes(ctx context.Context, projectPath string) ([]AutoFix, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, "fix", "--list", "--format=json", projectPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var fixes []AutoFix
	if err := json.Unmarshal(output, &fixes); err != nil {
		return nil, err
	}

	return fixes, nil
}

// ApplyFix применяет автофикс
func (c *Client) ApplyFix(ctx context.Context, projectPath, fixID string) error {
	cmd := exec.CommandContext(ctx, c.binaryPath, "fix", "--apply", fixID, projectPath)
	return cmd.Run()
}

// parseDiagnostic парсит строку диагностики из JSON
func (c *Client) parseDiagnostic(line string) *Diagnostic {
	var diagnostic Diagnostic
	if err := json.Unmarshal([]byte(line), &diagnostic); err != nil {
		return nil
	}
	return &diagnostic
}

// BuildResult результат сборки
type BuildResult struct {
	ProjectPath   string
	Success       bool
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	Diagnostics   []Diagnostic
	ErrorOutput   []string
	Error         error
}

// Diagnostic диагностическое сообщение
type Diagnostic struct {
	Level     string    `json:"level"`     // error, warning, info
	Code      string    `json:"code"`      // Код ошибки
	Message   string    `json:"message"`   // Текст сообщения
	File      string    `json:"file"`      // Путь к файлу
	Line      int       `json:"line"`      // Номер строки
	Column    int       `json:"column"`    // Номер колонки
	Span      SpanInfo  `json:"span"`      // Информация о диапазоне
	Fixes     []AutoFix `json:"fixes"`     // Доступные автофиксы
}

// SpanInfo информация о диапазоне в файле
type SpanInfo struct {
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
	EndLine     int `json:"end_line"`
	EndColumn   int `json:"end_column"`
}

// AutoFix автоматическое исправление
type AutoFix struct {
	ID          string      `json:"id"`          // Уникальный ID фикса
	Type        string      `json:"type"`        // replace, delete, insert
	Description string      `json:"description"` // Описание исправления
	File        string      `json:"file"`        // Файл для исправления
	Span        SpanInfo    `json:"span"`        // Диапазон для исправления
	NewText     string      `json:"new_text"`    // Новый текст (для replace/insert)
	Preview     FixPreview  `json:"preview"`     // Предпросмотр изменений
}

// FixPreview предпросмотр изменений
type FixPreview struct {
	Before []string `json:"before"` // Строки до изменения
	After  []string `json:"after"`  // Строки после изменения
	Context int     `json:"context"` // Количество строк контекста
}

// String возвращает строковое представление диагностики
func (d *Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s [%s]",
		d.File, d.Line, d.Column, d.Level, d.Message, d.Code)
}

// IsError проверяет, является ли диагностика ошибкой
func (d *Diagnostic) IsError() bool {
	return d.Level == "error"
}

// IsWarning проверяет, является ли диагностика предупреждением
func (d *Diagnostic) IsWarning() bool {
	return d.Level == "warning"
}