package surge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

// Diagnose запускает `surge diag --format=json` по пути к файлу или директории.
// Для директории CLI возвращает JSON-объект вида map[string]DiagnosticsOutput.
// Для файла — объект DiagnosticsOutput.
func (c *Client) Diagnose(ctx context.Context, targetPath string, withNotes, withFixes bool) (*DiagResponse, error) {
	if targetPath == "" {
		targetPath = "."
	}

	args := []string{"diag", "--format", "json"}
	if withNotes {
		args = append(args, "--with-notes")
	}
	if withFixes {
		args = append(args, "--suggest")
	}
	args = append(args, targetPath)

	cmd := exec.CommandContext(ctx, c.binaryPath, args...)
	out, err := cmd.CombinedOutput()

	resp := &DiagResponse{Raw: out, ExitCode: 0}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			resp.ExitCode = ee.ExitCode()
		} else {
			resp.Err = err
			return resp, err
		}
	}

	// Попытка распарсить как пакет результатов (директория)
	var batch map[string]DiagnosticsOutput
	if json.Unmarshal(out, &batch) == nil && len(batch) > 0 {
		resp.Batch = batch
		return resp, nil
	}

	// Падение на map — не обязательно ошибка: возможно одиночный файл
	var single DiagnosticsOutput
	if uerr := json.Unmarshal(out, &single); uerr != nil {
		resp.Err = uerr
		return resp, uerr
	}
	resp.Single = &single
	return resp, nil
}

// BuildProject запускает сборку проекта
func (c *Client) BuildProject(ctx context.Context, projectPath string) (*BuildResult, error) {
	// Note: current surge build is a stub and doesn't support --format=json yet.
	// We invoke without format flag and attempt to parse JSON lines if present.
	cmd := exec.CommandContext(ctx, c.binaryPath, "build", projectPath)

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

// InitProject initializes a surge project at the given path.
func (c *Client) InitProject(ctx context.Context, projectPath string) error {
	cmd := exec.CommandContext(ctx, c.binaryPath, "init", projectPath)
	return cmd.Run()
}

// ListFixes возвращает доступные фиксы через `surge diag --format=json --suggest`.
// Для одиночного файла вернёт карту с одним ключом — путем файла.
func (c *Client) ListFixes(ctx context.Context, targetPath string) (map[string][]FixJSON, error) {
	resp, err := c.Diagnose(ctx, targetPath, false, true)
	if err != nil {
		return nil, err
	}

	fixesByFile := make(map[string][]FixJSON)

	if len(resp.Batch) > 0 {
		// Директория: пройдём по каждому файлу
		for displayPath, out := range resp.Batch {
			for _, dj := range out.Diagnostics {
				if len(dj.Fixes) == 0 {
					continue
				}
				file := dj.Location.File
				if file == "" {
					file = displayPath
				}
				fixesByFile[file] = append(fixesByFile[file], dj.Fixes...)
			}
		}
		return fixesByFile, nil
	}

	if resp.Single != nil {
		// Одиночный файл
		// Пытаемся определить имя файла из первой диагностики
		file := targetPath
		for _, dj := range resp.Single.Diagnostics {
			if dj.Location.File != "" {
				file = dj.Location.File
				break
			}
		}
		for _, dj := range resp.Single.Diagnostics {
			if len(dj.Fixes) == 0 {
				continue
			}
			fixesByFile[file] = append(fixesByFile[file], dj.Fixes...)
		}
		return fixesByFile, nil
	}

	return fixesByFile, nil
}

// ApplyFixByID применяет фикс с указанным ID к одному файлу.
// Эквивалентно: `surge fix --id <id> <file.sg>`.
func (c *Client) ApplyFixByID(ctx context.Context, filePath, fixID string) error {
	if fixID == "" {
		return fmt.Errorf("empty fix id")
	}
	cmd := exec.CommandContext(ctx, c.binaryPath, "fix", "--id", fixID, filePath)
	return cmd.Run()
}

// ApplyAllFixes применяет все безопасные фиксы (к файлу или директории).
func (c *Client) ApplyAllFixes(ctx context.Context, targetPath string) error {
	cmd := exec.CommandContext(ctx, c.binaryPath, "fix", "--all", targetPath)
	return cmd.Run()
}

// ApplyOneFix применяет один первый доступный фикс (к файлу или директории).
func (c *Client) ApplyOneFix(ctx context.Context, targetPath string) error {
	cmd := exec.CommandContext(ctx, c.binaryPath, "fix", "--once", targetPath)
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
	ProjectPath string
	Success     bool
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Diagnostics []Diagnostic
	ErrorOutput []string
	Error       error
}

// Diagnostic диагностическое сообщение
type Diagnostic struct {
	Level   string    `json:"level"`   // error, warning, info
	Code    string    `json:"code"`    // Код ошибки
	Message string    `json:"message"` // Текст сообщения
	File    string    `json:"file"`    // Путь к файлу
	Line    int       `json:"line"`    // Номер строки
	Column  int       `json:"column"`  // Номер колонки
	Span    SpanInfo  `json:"span"`    // Информация о диапазоне
	Fixes   []FixJSON `json:"fixes"`   // Доступные автофиксы
}

// SpanInfo информация о диапазоне в файле
type SpanInfo struct {
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
	EndLine     int `json:"end_line"`
	EndColumn   int `json:"end_column"`
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

// ===== JSON контракты для surge diag --format=json =====

// LocationJSON описывает местоположение в файле
type LocationJSON struct {
	File      string `json:"file"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
	StartLine uint32 `json:"start_line,omitempty"`
	StartCol  uint32 `json:"start_col,omitempty"`
	EndLine   uint32 `json:"end_line,omitempty"`
	EndCol    uint32 `json:"end_col,omitempty"`
}

// NoteJSON — дополнительная заметка
type NoteJSON struct {
	Message  string       `json:"message"`
	Location LocationJSON `json:"location"`
}

// FixEditJSON — одно редактирование
type FixEditJSON struct {
	Location LocationJSON `json:"location"`
	NewText  string       `json:"new_text"`
	OldText  string       `json:"old_text,omitempty"`
}

// FixJSON — описание автофикса
type FixJSON struct {
	ID            string        `json:"id,omitempty"`
	Title         string        `json:"title"`
	Kind          string        `json:"kind"`
	Applicability string        `json:"applicability"`
	IsPreferred   bool          `json:"is_preferred,omitempty"`
	BuildError    string        `json:"build_error,omitempty"`
	Edits         []FixEditJSON `json:"edits,omitempty"`
}

// DiagnosticJSON — диагностика (JSON)
type DiagnosticJSON struct {
	Severity string       `json:"severity"`
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	Location LocationJSON `json:"location"`
	Notes    []NoteJSON   `json:"notes,omitempty"`
	Fixes    []FixJSON    `json:"fixes,omitempty"`
}

// DiagnosticsOutput — корень JSON для одного файла
type DiagnosticsOutput struct {
	Diagnostics []DiagnosticJSON `json:"diagnostics"`
	Count       int              `json:"count"`
}

// DiagResponse — объединённый ответ от diag
type DiagResponse struct {
	Single   *DiagnosticsOutput
	Batch    map[string]DiagnosticsOutput
	ExitCode int
	Raw      []byte
	Err      error
}
