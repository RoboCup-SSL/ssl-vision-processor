package logging

import (
	"log/slog"
	"strings"
	"sync"
)

const (
	recentLimit = 50
	// A line is for its message and first attributes; a huge one (a whole
	// config in an attribute) would crowd out the rest of a system report.
	recentLineLimit = 2000
)

// recent holds the last warnings and errors as log lines, for the system
// report. It's in memory, so it works with file logging off.
var recent recentLines

// recentHandler formats warnings and errors into recent, in the log file's
// text format. Fixed at warn, whatever -logLevel says.
func recentHandler() slog.Handler {
	return slog.NewTextHandler(&recent, &slog.HandlerOptions{Level: slog.LevelWarn, AddSource: true})
}

// Recent is the host's last warnings and errors since it started, oldest
// first.
func Recent() []string {
	return recent.snapshot()
}

// recentLines is an io.Writer keeping the last recentLimit lines.
// slog.TextHandler writes each record in one Write call.
type recentLines struct {
	mu    sync.Mutex
	lines []string
}

func (r *recentLines) Write(p []byte) (int, error) {
	line := strings.TrimRight(string(p), "\n")
	if len(line) > recentLineLimit {
		line = line[:recentLineLimit] + "…"
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.lines = append(r.lines, line)
	if len(r.lines) > recentLimit {
		r.lines = r.lines[len(r.lines)-recentLimit:]
	}

	return len(p), nil
}

func (r *recentLines) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string{}, r.lines...)
}
