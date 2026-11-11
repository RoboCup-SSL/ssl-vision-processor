package logging

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

// useHistory gives one test an empty history, and a logger writing to it.
func useHistory(t *testing.T) *slog.Logger {
	t.Helper()

	previous := history
	history = newHistoryLog()
	t.Cleanup(func() { history = previous })

	return slog.New(historyHandler{log: history})
}

func TestHistoryKeepsEachLevelSeparately(t *testing.T) {
	log := useHistory(t)

	log.Warn("early warning")

	for i := range historyLimit + 5 {
		log.Debug("noise", "n", i)
	}

	page := History(context.Background(), 0, slog.LevelWarn, 0, 0)
	if len(page.Records) != 1 || page.Records[0].Message != "early warning" || page.Missed {
		t.Errorf("warnings = %+v, missed %v; the debug records pushed the warning out", page.Records, page.Missed)
	}

	page = History(context.Background(), 0, slog.LevelDebug, 0, 0)
	if len(page.Records) != historyLimit+1 || !page.Missed {
		t.Fatalf("got %d records, missed %v; want %d and missed", len(page.Records), page.Missed, historyLimit+1)
	}

	if page.Records[0].Message != "early warning" || page.Records[1].Attrs[0].Value != "5" {
		t.Errorf("oldest = %+v, %+v; want the warning, then debug n=5", page.Records[0], page.Records[1])
	}

	if !slices.IsSortedFunc(page.Records, func(a, b Record) int { return int(a.Seq - b.Seq) }) {
		t.Error("records aren't in order")
	}
}

func TestHistoryFollowsFromAfter(t *testing.T) {
	log := useHistory(t)

	log.Info("first")
	start := History(context.Background(), 0, slog.LevelDebug, 0, 0)

	log.Info("second")
	log.Error("third")

	page := History(context.Background(), start.Next, slog.LevelDebug, 1, 0)
	if len(page.Records) != 1 || page.Records[0].Message != "third" || !page.Missed {
		t.Errorf("limit 1 = %+v, missed %v; want the newest and missed", page.Records, page.Missed)
	}

	if page = History(context.Background(), page.Next, slog.LevelDebug, 0, 0); len(page.Records) != 0 || page.Missed {
		t.Errorf("caught up = %+v, missed %v; want nothing", page.Records, page.Missed)
	}
}

func TestHistoryFlattensGroupsAndTruncates(t *testing.T) {
	log := useHistory(t)

	log.With("instance", "vp-1").WithGroup("cam").Info("frame",
		"id", 3,
		slog.Group("size", "w", 640, "h", 480),
		slog.Group("empty"),
		"config", strings.Repeat("x", 3*recentLineLimit),
	)

	r := History(context.Background(), 0, slog.LevelDebug, 0, 0).Records[0]

	var keys []string
	for _, a := range r.Attrs {
		keys = append(keys, a.Key)
	}

	if want := []string{"instance", "cam.id", "cam.size.w", "cam.size.h", "cam.config"}; !slices.Equal(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}

	if last := r.Attrs[len(r.Attrs)-1].Value; len(last) > recentLineLimit+len("…") {
		t.Errorf("config is %d bytes, want it truncated", len(last))
	}

	if r.Level != "info" || !strings.HasPrefix(r.Source, "logging/history_test.go:") {
		t.Errorf("level %q, source %q", r.Level, r.Source)
	}
}

func TestHistoryWaitsForAMatchingRecord(t *testing.T) {
	log := useHistory(t)

	start := History(context.Background(), 0, slog.LevelDebug, 0, 0)

	done := make(chan HistoryPage)
	go func() { done <- History(context.Background(), start.Next, slog.LevelWarn, 0, 10*time.Second) }()

	// An info record doesn't match, so it keeps waiting.
	log.Info("not this")
	time.Sleep(20 * time.Millisecond)
	log.Warn("this")

	select {
	case page := <-done:
		if len(page.Records) != 1 || page.Records[0].Message != "this" {
			t.Errorf("records = %+v, want the warning", page.Records)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting after the warning")
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { done <- History(ctx, start.Next+10, slog.LevelError, 0, 10*time.Second) }()
	cancel()

	select {
	case page := <-done:
		if len(page.Records) != 0 {
			t.Errorf("records = %+v after cancel, want none", page.Records)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting after the context was cancelled")
	}
}
