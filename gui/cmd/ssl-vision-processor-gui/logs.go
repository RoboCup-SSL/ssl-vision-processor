package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
)

// maxLogsWait is the longest GET /api/logs waits, under the minute proxies
// commonly time out at.
const maxLogsWait = 30

// logsRequest picks which of the host's log records to send.
type logsRequest struct {
	// After is the seq of the last record the client has; only later ones are
	// sent. Absent or 0 for every record kept.
	After int64 `query:"after"`
	// Level is the lowest level sent, debug if absent.
	Level string `query:"level" enum:"debug,info,warn,error"`
	// Limit is the most records sent, the newest ones. Absent or 0 for no
	// limit.
	Limit int `query:"limit"`
	// Wait is how many seconds, up to 30, to wait for a record when none
	// matches yet, for clients that want records as they're logged. Absent
	// or 0 answers at once.
	Wait int `query:"wait"`
}

// logsResponse is a page of the host's log records.
type logsResponse struct {
	// Records are oldest first.
	Records []logging.Record `json:"records"`
	// Next is the after to ask with next time.
	Next int64 `json:"next"`
	// Missed is true when matching records after after aren't in records:
	// the host has dropped them, or they're beyond limit. The host keeps the
	// last 1000 of each level.
	Missed bool `json:"missed"`
}

// logs serves the host's recent log records, its supervised vision_processors'
// output included, whatever level its console logs at. Poll with after set to
// the last response's next to follow the log; add wait to be answered as soon
// as a record is logged.
func (s *VisionServer) logs(r *http.Request, in logsRequest) (logsResponse, error) {
	level := slog.LevelDebug

	if in.Level != "" {
		var err error
		if level, err = logging.ParseLevel(in.Level); err != nil {
			return logsResponse{}, httpjson.Errorf(httpjson.CodeBadRequest, "level: %v", err)
		}
	}

	if in.Limit < 0 {
		return logsResponse{}, httpjson.Errorf(httpjson.CodeBadRequest, "limit: want 0 or more, got %d", in.Limit)
	}

	if in.Wait < 0 || in.Wait > maxLogsWait {
		return logsResponse{}, httpjson.Errorf(httpjson.CodeBadRequest, "wait: want 0 to %d seconds, got %d", maxLogsWait, in.Wait)
	}

	page := logging.History(r.Context(), in.After, level, in.Limit, time.Duration(in.Wait)*time.Second)

	return logsResponse{Records: page.Records, Next: page.Next, Missed: page.Missed}, nil
}
