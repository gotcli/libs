package logger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Loggers struct {
	Inbound  *slog.Logger
	Outbound *slog.Logger
	Service  *slog.Logger
	files    []*os.File
}

var current *Loggers

type requestIDKey struct{}

type GORMLogger struct {
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func Setup(directory string) (*Loggers, error) {
	if directory == "" {
		directory = "logs"
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, "application.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, err
	}
	application := slog.New(slog.NewJSONHandler(io.MultiWriter(os.Stdout, file), &slog.HandlerOptions{Level: slog.LevelInfo}))
	loggers := &Loggers{Inbound: application, Outbound: application, Service: application, files: []*os.File{file}}
	current = loggers
	return loggers, nil
}

func Current() *Loggers { return current }
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

func NewGORMLogger(slowThreshold time.Duration) gormlogger.Interface {
	if slowThreshold <= 0 {
		slowThreshold = 200 * time.Millisecond
	}
	return &GORMLogger{level: gormlogger.Info, slowThreshold: slowThreshold}
}

func (l *GORMLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *GORMLogger) Info(ctx context.Context, _ string, _ ...any) {
	if l.level >= gormlogger.Info {
		databaseLog(ctx, slog.LevelInfo, "database info", nil)
	}
}
func (l *GORMLogger) Warn(ctx context.Context, _ string, _ ...any) {
	if l.level >= gormlogger.Warn {
		databaseLog(ctx, slog.LevelWarn, "database warning", nil)
	}
}
func (l *GORMLogger) Error(ctx context.Context, _ string, _ ...any) {
	if l.level >= gormlogger.Error {
		databaseLog(ctx, slog.LevelError, "database error", nil)
	}
}

func (l *GORMLogger) Trace(ctx context.Context, started time.Time, query func() (string, int64), err error) {
	if l.level == gormlogger.Silent {
		return
	}
	_, rows := query()
	duration := time.Since(started)
	attrs := []any{"event", "database", "request_id", RequestID(ctx), "duration_ms", duration.Milliseconds(), "rows_affected", rows, "slow_query", duration > l.slowThreshold}
	switch {
	case err != nil && l.level >= gormlogger.Error && !errors.Is(err, gorm.ErrRecordNotFound):
		databaseLog(ctx, slog.LevelError, "database query failed", append(attrs, "error", err.Error()))
	case duration > l.slowThreshold && l.level >= gormlogger.Warn:
		databaseLog(ctx, slog.LevelWarn, "slow database query", attrs)
	case l.level >= gormlogger.Info:
		databaseLog(ctx, slog.LevelInfo, "database query completed", attrs)
	}
}

func databaseLog(ctx context.Context, level slog.Level, message string, attrs []any) {
	if current == nil || current.Service == nil {
		return
	}
	current.Service.Log(ctx, level, message, attrs...)
}

func (l *Loggers) Close() error {
	var first error
	for _, file := range l.files {
		if err := file.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func ServiceResult(ctx context.Context, service, operation string, started time.Time, result any, err error) {
	if current == nil || current.Service == nil {
		return
	}
	attrs := []any{"event", "service", "request_id", RequestID(ctx), "service", service, "operation", operation, "duration_ms", time.Since(started).Milliseconds()}
	if err != nil {
		current.Service.ErrorContext(ctx, "service operation failed", append(attrs, "error", err.Error())...)
		return
	}
	current.Service.InfoContext(ctx, "service operation completed", append(attrs, "result_data", safeResult(result))...)
}

func safeResult(value any) any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return map[string]any{"log_error": "result is not JSON serializable"}
	}
	if len(encoded) > 64*1024 {
		return map[string]any{"truncated": true, "size_bytes": len(encoded)}
	}
	var result any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return map[string]any{"log_error": "result cannot be normalized"}
	}
	return redact(result)
}

func redact(value any) any {
	switch current := value.(type) {
	case map[string]any:
		for key, item := range current {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "authorization") || strings.Contains(lower, "cookie") {
				current[key] = "[REDACTED]"
				continue
			}
			current[key] = redact(item)
		}
	case []any:
		for index, item := range current {
			current[index] = redact(item)
		}
	}
	return value
}
