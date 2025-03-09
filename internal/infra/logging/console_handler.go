package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const (
	ansiCodeReset     = "\033[0m"
	ansiCodeRed       = "\033[31m"
	ansiCodeGreen     = "\033[32m"
	ansiCodeYellow    = "\033[33m"
	ansiCodeBlue      = "\033[34m"
	ansiCodePurple    = "\033[35m"
	ansiCodeCyan      = "\033[36m"
	ansiCodeWhite     = "\033[37m"
	ansiCodeGray      = "\033[90m"
	ansiCodeBold      = "\033[1m"
	ansiCodeItalic    = "\033[3m"
	ansiCodeUnderline = "\033[4m"
)

const (
	ansiCodeDebug = ansiCodeCyan
	ansiCodeInfo  = ansiCodeGreen
	ansiCodeWarn  = ansiCodeYellow
	ansiCodeError = ansiCodeRed
)

//nolint:gochecknoglobals
var ansiCodeMap = map[slog.Level]string{
	slog.LevelDebug: ansiCodeDebug,
	slog.LevelInfo:  ansiCodeInfo,
	slog.LevelWarn:  ansiCodeWarn,
	slog.LevelError: ansiCodeError,
}

// ConsoleHandler implements slog.Handler to format log records with ansiCodes
// and human-readable output suitable for development environments.
type ConsoleHandler struct {
	// Output is the destination for log output (typically os.Stdout or os.Stderr)
	Output io.Writer
	// Level is the minimum level for log records to be processed
	Level slog.Leveler
	// PkgLevels maps package names to minimum log levels
	PkgLevels map[string]slog.Level

	attrs  []slog.Attr
	groups []string
}

var _ slog.Handler = (*ConsoleHandler)(nil)

// Handle implements slog.Handler by formatting the log record with ansiCodes,
// timestamps, and source file information.
//
//nolint:funlen
func (h *ConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	// collect attrs
	var attrs []slog.Attr

	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)

		return true
	})

	attrs = append(attrs, h.attrs...)

	// determine pkg
	var pkg string

	for _, attr := range attrs {
		if attr.Key == "name" {
			pkg = attr.Value.String()

			break
		}
	}

	// abort if pkg level is too low
	pkgParts := strings.Split(pkg, ".")

	for i := 0; i <= len(pkgParts); i++ {
		var key string
		if i < len(pkgParts) {
			key = strings.Join(pkgParts[:len(pkgParts)-i], ".")
		}

		level, ok := h.PkgLevels[key]
		if !ok {
			continue
		}

		if r.Level.Level() < level {
			return nil
		}

		if ok {
			break
		}
	}

	// format log message
	logMessage := ansiCodeGray + r.Time.Format("15:04:05.000000") + ansiCodeReset
	logMessage += " " + ansiCodeMap[r.Level] + "[" + r.Level.String() + "]" + ansiCodeReset
	logMessage += " " + r.Message

	var prefix string

	if len(h.groups) > 0 {
		prefix = strings.Join(h.groups, ".") + "."
	}

	if len(attrs) > 0 {
		logMessage += " " + ansiCodeGray + "|" + ansiCodeReset
		logMessage += h.renderAttrs(prefix, attrs)
	}

	// format caller
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	fn := strings.Split(f.Function, string(os.PathSeparator))

	logMessage += "\n-> " + ansiCodeGray + fn[len(fn)-1] + "()"
	logMessage += " in " + ansiCodeUnderline + f.File + ":" + strconv.Itoa(f.Line) + ansiCodeReset

	fmt.Fprintln(h.Output, logMessage)

	return nil
}

func (h *ConsoleHandler) renderAttrs(prefix string, attrs []slog.Attr) (out string) {
	for _, attr := range attrs {
		if attr.Value.Kind() == slog.KindGroup {
			out += h.renderAttrs(prefix+attr.Key+".", attr.Value.Group())

			continue
		}

		out += " " + prefix + attr.Key
		out += "=" + ansiCodeGray + attr.Value.String() + ansiCodeReset
	}

	return
}

// WithAttrs implements slog.Handler.WithAttrs.
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) Handler {
	return &ConsoleHandler{
		Output:    h.Output,
		Level:     h.Level,
		PkgLevels: h.PkgLevels,
		attrs:     append(h.attrs, attrs...),
		groups:    h.groups,
	}
}

// WithGroup implements slog.Handler.WithGroup.
func (h *ConsoleHandler) WithGroup(name string) Handler {
	return &ConsoleHandler{
		Output:    h.Output,
		Level:     h.Level,
		PkgLevels: h.PkgLevels,
		attrs:     h.attrs,
		groups:    append(h.groups, name),
	}
}

// Enabled implements slog.Handler.Enabled.
func (h *ConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Level.Level() <= level
}
