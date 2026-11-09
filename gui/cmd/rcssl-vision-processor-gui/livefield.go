package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/livefield"
)

// liveFieldTopic carries livefield.Snapshot: every camera's latest detection
// frame and every autoref's latest tracked frame.
const liveFieldTopic = "live_field.frames"

// Detections arrive at 60 to 75 Hz per camera; publishing faster than a
// browser draws gains nothing.
const (
	liveFieldInterval = time.Second / 60
	// liveFieldIdle republishes an unchanged feed, so receiving flags and
	// rates stay current when every source has gone quiet.
	liveFieldIdle = time.Second
)

// publishLiveField publishes the feed's snapshot while anyone subscribes,
// whenever a frame has arrived since the last one, until ctx is done.
func publishLiveField(ctx context.Context, wsHub *hub.Hub, live *livefield.Feed) error {
	ticker := time.NewTicker(liveFieldInterval)
	defer ticker.Stop()

	var (
		version   uint64
		published time.Time
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			if !wsHub.Watched(liveFieldTopic) {
				continue
			}

			v := live.Version()
			if v == version && now.Sub(published) < liveFieldIdle {
				continue
			}

			data, err := json.Marshal(live.Snapshot(now))
			if err != nil {
				slog.Error("marshalling live field", "err", err)

				continue
			}

			wsHub.Publish(liveFieldTopic, data)
			version, published = v, now
		}
	}
}

// getLiveField serves every camera's latest detection frame and every
// autoref's latest tracked frame, as the live_field.frames topic carries
// them.
//
// Sources silent for a second are listed without a frame, and dropped after
// 30 seconds. Detection positions are in mm, tracked positions in m.
func (s *VisionServer) getLiveField(*http.Request, none) (livefield.Snapshot, error) {
	return s.status.LiveField(), nil
}
