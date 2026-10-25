// Package detections tracks which vision_processors are sending detection
// frames: each camera_id heard, from which address, and how often. The Camera
// Layout page uses it to show which slots are live and to catch two
// processors claiming the same camera_id.
package detections

import (
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

// Window is how far back FPS is measured, and how recently a source must
// have been heard to count as receiving.
const Window = time.Second

// Forget drops a source silent this long, so one that moved to another
// camera_id or shut down stops showing.
const Forget = 30 * time.Second

// Source is one camera_id heard from one address.
type Source struct {
	CameraID  uint32    `json:"cameraId"`
	Address   string    `json:"address"`
	FPS       int       `json:"fps"`
	LastHeard time.Time `json:"lastHeard"`
	Receiving bool      `json:"receiving"`
}

type key struct {
	cameraID uint32
	address  string
}

// Tracker is safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	sources map[key][]time.Time // arrival times within Window, newest last
	last    map[key]time.Time
}

func NewTracker() *Tracker {
	return &Tracker{sources: map[key][]time.Time{}, last: map[key]time.Time{}}
}

// Record notes a detection frame for cameraID from from, arriving at now.
func (t *Tracker) Record(cameraID uint32, from net.IP, now time.Time) {
	k := key{cameraID, from.String()}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.sources[k] = append(trim(t.sources[k], now), now)
	t.last[k] = now
}

// Sources lists every source heard within Forget, by camera_id then address.
func (t *Tracker) Sources(now time.Time) []Source {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := []Source{}

	for k, last := range t.last {
		if now.Sub(last) > Forget {
			delete(t.last, k)
			delete(t.sources, k)

			continue
		}

		t.sources[k] = trim(t.sources[k], now)
		out = append(out, Source{
			CameraID:  k.cameraID,
			Address:   k.address,
			FPS:       len(t.sources[k]),
			LastHeard: last,
			Receiving: now.Sub(last) <= Window,
		})
	}

	slices.SortFunc(out, func(a, b Source) int {
		if a.CameraID != b.CameraID {
			return int(a.CameraID) - int(b.CameraID)
		}

		return strings.Compare(a.Address, b.Address)
	})

	return out
}

// trim drops arrivals older than Window before now.
func trim(times []time.Time, now time.Time) []time.Time {
	i := 0
	for i < len(times) && now.Sub(times[i]) > Window {
		i++
	}

	return times[i:]
}
