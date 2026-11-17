package logging

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestRecentKeepsTheLastWarningsAndErrors(t *testing.T) {
	t.Cleanup(func() { recent = recentLines{} })

	log := slog.New(recentHandler())
	log.Info("not kept")

	for i := range recentLimit + 5 {
		log.Warn("warning", "n", i)
	}

	log.Error("failed", "err", strings.Repeat("x", 3*recentLineLimit))

	got := Recent()
	if len(got) != recentLimit {
		t.Fatalf("kept %d lines, want %d", len(got), recentLimit)
	}

	if want := fmt.Sprintf("n=%d", 6); !strings.HasSuffix(got[0], want) {
		t.Errorf("oldest = %q, want it to end %s", got[0], want)
	}

	if last := got[len(got)-1]; !strings.Contains(last, "level=ERROR") || len(last) > recentLineLimit+len("…") {
		t.Errorf("newest = %.80q… (%d bytes), want a truncated error", last, len(last))
	}
}
