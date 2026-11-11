package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
)

// installLogging routes slog through logging.Setup for one test, its console
// at error so the test's own records don't clutter the output.
func installLogging(t *testing.T) {
	t.Helper()

	previous := slog.Default()
	closeLog, err := logging.Setup(logging.Config{Level: slog.LevelError})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		closeLog()
		slog.SetDefault(previous)
	})
}

func TestLogsServesRecordsAboveTheConsoleLevel(t *testing.T) {
	h, _ := testServer(t)
	installLogging(t)

	var start logsResponse
	if err := json.NewDecoder(do(t, h, http.MethodGet, "/api/logs?limit=1", nil).Body).Decode(&start); err != nil {
		t.Fatal(err)
	}

	slog.Debug("debug record", "n", 1)
	slog.Warn("warn record", slog.Group("cam", "id", 3))

	after := "/api/logs?after=" + strconv.FormatInt(start.Next, 10)

	rec := do(t, h, http.MethodGet, after+"&level=warn", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", rec.Code, rec.Body)
	}

	checkResponse(t, "/api/logs", rec.Body.Bytes())

	var got logsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if len(got.Records) != 1 || got.Records[0].Message != "warn record" || got.Records[0].Level != "warn" {
		t.Fatalf("records = %+v, want just the warning", got.Records)
	}

	if want := []logging.Attr{{Key: "cam.id", Value: "3"}}; !slices.Equal(got.Records[0].Attrs, want) {
		t.Errorf("attrs = %+v, want %+v", got.Records[0].Attrs, want)
	}

	if got.Next <= start.Next || got.Missed {
		t.Errorf("next = %d (was %d), missed = %v", got.Next, start.Next, got.Missed)
	}

	// The console is at error, but the debug record is kept.
	if err := json.NewDecoder(do(t, h, http.MethodGet, after, nil).Body).Decode(&got); err != nil {
		t.Fatal(err)
	}

	if len(got.Records) != 2 || got.Records[0].Message != "debug record" {
		t.Errorf("records = %+v, want the debug record then the warning", got.Records)
	}
}

func TestLogsRejectsBadParameters(t *testing.T) {
	h, _ := testServer(t)

	for _, target := range []string{
		"/api/logs?level=verbose",
		"/api/logs?limit=-1",
		"/api/logs?wait=31",
		"/api/logs?after=soon",
	} {
		if rec := do(t, h, http.MethodGet, target, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want 400", target, rec.Code)
		}
	}
}
