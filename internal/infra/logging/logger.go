package logging

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Constants for log levels that match slog.Level values.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Type aliases for commonly used slog types.
type (
	Logger  = *slog.Logger
	Handler = slog.Handler
	Level   = slog.Level
)

//nolint:gochecknoglobals
var logLevelStrToLevel = map[string]Level{
	"debug": LevelDebug,
	"info":  LevelInfo,
	"warn":  LevelWarn,
	"error": LevelError,
}

// LoggerConfig holds configuration parameters for logging.
type LoggerConfig struct {
	// AppName is the application identifier added to all log entries
	AppName string

	// Output specifies where logs are written ("stdout", "stderr", "discard" or a file path)
	Output string `env:"OUTPUT" default:"stderr"`

	// Level sets the minimum log level ("debug", "info", "warn", "error")
	Level string `env:"LEVEL" default:"info"`

	// Filter specifies package-level logging overrides ("pkg:level,pkg:level")
	Filter string `env:"FILTER" default:""`

	// JSON enables JSON-formatted output instead of human-readable console output
	JSON bool `env:"JSON" default:"false"`

	OutputHandle io.Writer
}

//nolint:gochecknoglobals
var (
	Group      = slog.Group
	GroupValue = slog.GroupValue

	config     LoggerConfig
	configLock sync.Mutex
)

// Configure sets up global logging configuration for the application.
// It must be called before any loggers are created.
func Configure(ctx context.Context, cfg LoggerConfig, appName string) {
	configure(cfg, appName)

	GetLogger("infra.logging").With(Group("config",
		"appName", config.AppName,
		"output", config.Output,
		"level", config.Level,
		"filter", config.Filter,
		"json", config.JSON,
	)).DebugContext(ctx, "logging configured")
}

func configure(cfg LoggerConfig, appName string) {
	configLock.Lock()
	defer configLock.Unlock()

	config = cfg
	config.AppName = appName

	if cfg.OutputHandle == nil {
		switch cfg.Output {
		case "":
			fallthrough
		case "discard":
			config.OutputHandle = io.Discard
		case "stdout":
			config.OutputHandle = os.Stdout
		case "stderr":
			config.OutputHandle = os.Stderr
		default:
			file, err := os.OpenFile(config.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				panic(fmt.Errorf("failed to open log file: %w", err))
			}

			config.OutputHandle = file
		}
	}

	slog.SetLogLoggerLevel(parseLogLevel(config.Level, LevelInfo))
}

// GetLogLogger creates a standard library *log.Logger that writes through a slog.Logger.
// Useful for adapting third-party code that expects a *log.Logger.
func GetLogLogger(logger Logger, level Level) *log.Logger {
	handler := logger.With("stdlog", true).Handler()

	return slog.NewLogLogger(handler, level)
}

// GetLogger creates a new logger with the given name using the global configuration.
// The name is included in log entries to identify the source module.
func GetLogger(name string) Logger {
	output := getOutput()

	if output == io.Discard {
		return NewNopLogger()
	}

	levelVar := new(slog.LevelVar)
	levelVar.Set(parseLogLevel(config.Level, LevelInfo))

	var handler slog.Handler

	if config.JSON {
		//nolint:exhaustruct
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			AddSource: true,
			Level:     levelVar,
		})
	} else {
		//nolint:exhaustruct
		handler = &ConsoleHandler{
			Output:    output,
			Level:     levelVar,
			PkgLevels: config.getPkgLevels(),
		}
	}

	handler = NewTracingHandler(handler)

	logger := slog.New(handler)

	if config.AppName != "" {
		logger = logger.With("app", config.AppName)
	}

	return logger.With("logger", name)
}

func (cfg LoggerConfig) getPkgLevels() map[string]slog.Level {
	levels := make(map[string]slog.Level)

	for _, pkgLevel := range strings.Split(cfg.Filter, ",") {
		parts := strings.Split(pkgLevel, ":")
		if len(parts) != 2 {
			continue
		}

		levels[parts[0]] = parseLogLevel(parts[1], LevelDebug)
	}

	return levels
}

func getOutput() io.Writer {
	configLock.Lock()
	defer configLock.Unlock()

	if handle := config.OutputHandle; handle != nil {
		return handle
	}

	return io.Discard
}

func parseLogLevel(levelStr string, fallback Level) Level {
	levelStr = strings.TrimSpace(levelStr)
	levelStr = strings.ToLower(levelStr)

	level, ok := logLevelStrToLevel[levelStr]
	if !ok {
		return fallback
	}

	return level
}
