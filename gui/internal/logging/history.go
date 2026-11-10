package logging

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"
)

// historyLimit is how many records history keeps per level, so a burst of
// debug records can't push out the warnings before it.
const historyLimit = 1000

// Record is one log record, as GET /api/logs serves it.
type Record struct {
	// Seq orders records. It's unique across host restarts too: a new run
	// numbers from its start time in microseconds, above any earlier run's.
	Seq int64 `json:"seq"`
	// Time is when it was logged, RFC 3339 with milliseconds.
	Time    string `json:"time"`
	Level   string `json:"level" enum:"debug,info,warn,error"`
	Message string `json:"message"`
	// Source is the file and line that logged it, such as
	// supervisor/supervisor.go:554; empty if unknown.
	Source string `json:"source"`
	// Attrs are its attributes in order, a group's keys prefixed with the
	// group's name and a dot.
	Attrs []Attr `json:"attrs"`
}

// Attr is one of a record's attributes.
type Attr struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// levels are the names Record.Level uses, by the index levelIndex gives.
var levels = [...]string{"debug", "info", "warn", "error"}

// levelIndex buckets a level into levels: anything below info is debug,
// anything from error up is error.
func levelIndex(level slog.Level) int {
	switch {
	case level < slog.LevelInfo:
		return 0
	case level < slog.LevelWarn:
		return 1
	case level < slog.LevelError:
		return 2
	default:
		return 3
	}
}

// history holds the last historyLimit records of each level, for GET
// /api/logs. Like recent, it keeps everything whatever -logLevel says.
var history = newHistoryLog()

type historyLog struct {
	mu sync.Mutex
	// seq is the last Seq given out.
	seq   int64
	rings [len(levels)]historyRing
	// changed is closed and replaced on every record, waking Wait.
	changed chan struct{}
}

type historyRing struct {
	records []Record
	// evicted is the Seq of the newest record dropped to stay in the limit.
	evicted int64
}

func newHistoryLog() *historyLog {
	return &historyLog{seq: time.Now().UnixMicro(), changed: make(chan struct{})}
}

func (h *historyLog) add(level int, r Record) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.seq++
	r.Seq = h.seq

	ring := &h.rings[level]
	ring.records = append(ring.records, r)

	if over := len(ring.records) - historyLimit; over > 0 {
		ring.evicted = ring.records[over-1].Seq
		ring.records = ring.records[over:]
	}

	close(h.changed)
	h.changed = make(chan struct{})
}

// HistoryPage is what History returns.
type HistoryPage struct {
	// Records are oldest first.
	Records []Record
	// Next is the after for the next call: the last Seq given out, at any
	// level.
	Next int64
	// Missed is true when some records after after, at these levels, aren't
	// in Records: dropped from the history, or beyond limit.
	Missed bool
}

func (h *historyLog) since(after int64, level slog.Level, limit int) (HistoryPage, <-chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()

	page := HistoryPage{Records: []Record{}, Next: h.seq}

	for i := levelIndex(level); i < len(h.rings); i++ {
		ring := &h.rings[i]
		if ring.evicted > after {
			page.Missed = true
		}

		first := sort.Search(len(ring.records), func(j int) bool { return ring.records[j].Seq > after })
		page.Records = append(page.Records, ring.records[first:]...)
	}

	slices.SortFunc(page.Records, func(a, b Record) int { return cmp.Compare(a.Seq, b.Seq) })

	if limit > 0 && len(page.Records) > limit {
		page.Records = page.Records[len(page.Records)-limit:]
		page.Missed = true
	}

	return page, h.changed
}

// History is the records logged after the Seq after at level or above,
// oldest first: the newest limit of them, or all if limit is 0. While none
// match it waits for one, up to wait or until ctx is done.
func History(ctx context.Context, after int64, level slog.Level, limit int, wait time.Duration) HistoryPage {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	for {
		page, changed := history.since(after, level, limit)
		if len(page.Records) > 0 || page.Missed || wait <= 0 {
			return page
		}

		select {
		case <-changed:
		case <-timer.C:
			return page
		case <-ctx.Done():
			return page
		}
	}
}

// historyHandler turns records into Records for history. It's enabled at
// every level, so the GUI's log panel can show debug records even when the
// console and file don't.
type historyHandler struct {
	log *historyLog
	// attrs are WithAttrs's, already prefixed with their groups.
	attrs []Attr
	// prefix is WithGroup's groups, each followed by a dot.
	prefix string
}

func (h historyHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h historyHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make([]Attr, len(h.attrs), len(h.attrs)+r.NumAttrs())
	copy(attrs, h.attrs)

	r.Attrs(func(a slog.Attr) bool {
		attrs = appendAttr(attrs, h.prefix, a)

		return true
	})

	h.log.add(levelIndex(r.Level), Record{
		Time:    r.Time.Format("2006-01-02T15:04:05.000Z07:00"),
		Level:   levels[levelIndex(r.Level)],
		Message: truncate(r.Message),
		Source:  source(r.PC),
		Attrs:   attrs,
	})

	return nil
}

func (h historyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := h
	out.attrs = slices.Clone(h.attrs)

	for _, a := range attrs {
		out.attrs = appendAttr(out.attrs, h.prefix, a)
	}

	return out
}

func (h historyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	out := h
	out.prefix += name + "."

	return out
}

// appendAttr appends a to attrs as slog's text handler would write it: groups
// flattened into dotted keys, empty attributes and groups left out.
func appendAttr(attrs []Attr, prefix string, a slog.Attr) []Attr {
	a.Value = a.Value.Resolve()

	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}

		for _, sub := range a.Value.Group() {
			attrs = appendAttr(attrs, prefix, sub)
		}

		return attrs
	}

	if a.Equal(slog.Attr{}) {
		return attrs
	}

	return append(attrs, Attr{Key: prefix + a.Key, Value: truncate(a.Value.String())})
}

// source is the file, with its directory, and line of pc.
func source(pc uintptr) string {
	if pc == 0 {
		return ""
	}

	frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	if frame.File == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s:%d", filepath.Base(filepath.Dir(frame.File)), filepath.Base(frame.File), frame.Line)
}

// truncate cuts a value as recent cuts a line, so a whole config in an
// attribute doesn't fill the panel.
func truncate(s string) string {
	if len(s) > recentLineLimit {
		return s[:recentLineLimit] + "…"
	}

	return s
}
