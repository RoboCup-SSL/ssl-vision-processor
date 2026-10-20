// Package logging configures the process wide slog handler: a condensed,
// coloured console for whoever is watching the terminal, and a full fidelity
// rotating file for reading after a match.
package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmittmann/tint"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	rotateMaxSizeMB  = 50
	rotateMaxBackups = 5
	rotateMaxAgeDays = 14
)

// logLevel is package level so that an API handler can raise the level at
// runtime through SetLevel. LevelVar is safe for concurrent use; a plain
// slog.Level is not.
var logLevel slog.LevelVar

// Config describes one process's logging. The zero value logs at info level to
// stderr only.
type Config struct {
	Level slog.Level

	// File is the rotating log file to write alongside stderr. Empty disables
	// file logging entirely.
	File string
}

// SetLevel changes the level of every handler installed by Setup. Safe to call
// from any goroutine, which is what makes a runtime log level endpoint possible.
func SetLevel(level slog.Level) {
	logLevel.Set(level)
}

// Level reports the level currently in effect.
func Level() slog.Level {
	return logLevel.Level()
}

// ParseLevel maps a flag value onto a slog.Level, case insensitively. An
// unrecognised value yields slog.LevelInfo along with an error, so a caller can
// carry on logging and still report the typo.
func ParseLevel(name string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", name)
	}
}

// Setup installs the process wide slog handler and returns a function that
// flushes and closes the log file.
//
// The handler is installed whether or not the log file could be opened, so a
// non-nil error means "file logging is off", not "logging is unusable" -- losing
// the file must not stop the GUI from starting at a venue. Callers should warn
// and carry on.
//
// slog.SetDefault also routes the standard log package through this handler, so
// log.Printf calls made after this point come out structured too.
func Setup(cfg Config) (func(), error) {
	logLevel.Set(cfg.Level)

	handlers := []slog.Handler{
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:      &logLevel,
			AddSource:  true,
			TimeFormat: "15:04:05.000",
			NoColor:    !colorSupported(os.Stderr),
		}),
	}
	closeLog := func() {}
	var setupErr error

	if cfg.File != "" {
		// lumberjack creates the file but not its parent directories.
		if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
			setupErr = fmt.Errorf("create log directory for %v: %w", cfg.File, err)
		} else {
			rotator := &lumberjack.Logger{
				Filename:   cfg.File,
				MaxSize:    rotateMaxSizeMB, // megabytes per file
				MaxBackups: rotateMaxBackups,
				MaxAge:     rotateMaxAgeDays, // days
				Compress:   false,
			}
			handlers = append(handlers, slog.NewTextHandler(rotator, &slog.HandlerOptions{
				Level:     &logLevel,
				AddSource: true,
			}))
			closeLog = func() {
				if err := rotator.Close(); err != nil {
					slog.Error("closing log file", "err", err)
				}
			}
		}
	}

	slog.SetDefault(slog.New(multiHandler(handlers)))

	return closeLog, setupErr
}

// colorSupported reports whether ANSI colour should be emitted to f. A pipe or
// a redirect to a file is not a terminal, and NO_COLOR is the cross-tool
// convention for turning colour off on one that is.
func colorSupported(f *os.File) bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}

	stat, err := f.Stat()

	return err == nil && stat.Mode()&os.ModeCharDevice != 0
}

// multiHandler fans one record out to several handlers. io.MultiWriter cannot
// do this job because both destinations would then share one format.
type multiHandler []slog.Handler

func (m multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m {
		if h.Enabled(ctx, level) {
			return true
		}
	}

	return false
}

func (m multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs error
	for _, h := range m {
		if !h.Enabled(ctx, record.Level) {
			continue
		}

		if err := h.Handle(ctx, record.Clone()); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func (m multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithAttrs(attrs)
	}

	return out
}

func (m multiHandler) WithGroup(name string) slog.Handler {
	out := make(multiHandler, len(m))
	for i, h := range m {
		out[i] = h.WithGroup(name)
	}

	return out
}
