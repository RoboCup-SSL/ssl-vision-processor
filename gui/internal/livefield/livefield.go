// Package livefield keeps what the Live Field page draws: the latest
// detection frame from each vision_processor and the latest tracked frame
// from each autoref, with when the host received them and how often they
// arrive. Frames are kept as received and encoded as canonical protojson
// only when a snapshot asks for them, once per new frame.
package livefield

import (
	"cmp"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Window is how far back FPS is measured, and how recently a source must
// have been heard to count as receiving. A source that isn't receiving is
// listed without its frame, so nothing stale is drawn.
const Window = time.Second

// Forget drops a source silent this long, so the page's source list doesn't
// flicker when an autoref restarts but stops showing one that's gone.
const Forget = 30 * time.Second

// DetectionFrame is an SSL_DetectionFrame as canonical protojson. Positions
// are in mm.
type DetectionFrame []byte

func (f DetectionFrame) MarshalJSON() ([]byte, error) { return f, nil }

// TrackedFrame is a TrackedFrame as canonical protojson. Positions are in m.
type TrackedFrame []byte

func (f TrackedFrame) MarshalJSON() ([]byte, error) { return f, nil }

// Snapshot is every camera and autoref heard within Forget.
type Snapshot struct {
	// At is the host's clock when the snapshot was taken. Compare it with
	// receivedAt for a source's age; the senders' own timestamps come from
	// other machines' clocks.
	At       time.Time `json:"at"`
	Cameras  []Camera  `json:"cameras"`
	Trackers []Tracker `json:"trackers"`
}

// Camera is one camera_id heard from one address.
type Camera struct {
	CameraID   uint32    `json:"cameraId"`
	Address    string    `json:"address"`
	FPS        int       `json:"fps"`
	ReceivedAt time.Time `json:"receivedAt"`
	Receiving  bool      `json:"receiving"`
	// Frame is the latest detection frame, present while receiving.
	Frame DetectionFrame `json:"frame,omitempty"`
}

// Tracker is one autoref, by the uuid it sends.
type Tracker struct {
	UUID       string    `json:"uuid"`
	SourceName string    `json:"sourceName"`
	Address    string    `json:"address"`
	FPS        int       `json:"fps"`
	ReceivedAt time.Time `json:"receivedAt"`
	Receiving  bool      `json:"receiving"`
	// Frame is the latest tracked frame, present while receiving.
	Frame TrackedFrame `json:"frame,omitempty"`
}

type cameraKey struct {
	cameraID uint32
	address  string
}

type entry struct {
	address  string
	name     string
	frame    proto.Message
	encoded  []byte      // frame as protojson, nil until a snapshot needs it
	arrivals []time.Time // within Window, newest last
	last     time.Time
}

func (e *entry) record(frame proto.Message, from net.IP, now time.Time) {
	e.frame, e.encoded = frame, nil
	e.address = from.String()
	e.arrivals = append(trim(e.arrivals, now), now)
	e.last = now
}

// json is the frame as protojson, encoded on first use.
func (e *entry) json() []byte {
	if e.encoded == nil {
		data, err := protojson.Marshal(e.frame)
		if err != nil {
			slog.Warn("encoding live field frame", "err", err)

			return nil
		}

		e.encoded = data
	}

	return e.encoded
}

// Feed is safe for concurrent use.
type Feed struct {
	mu       sync.Mutex
	cameras  map[cameraKey]*entry
	trackers map[string]*entry
	version  uint64
}

func NewFeed() *Feed {
	return &Feed{cameras: map[cameraKey]*entry{}, trackers: map[string]*entry{}}
}

// RecordDetection keeps frame as its camera's latest from from. The feed
// keeps frame itself, so the caller mustn't change it afterwards.
func (f *Feed) RecordDetection(frame *vision.SSL_DetectionFrame, from net.IP, now time.Time) {
	k := cameraKey{frame.GetCameraId(), from.String()}

	f.mu.Lock()
	defer f.mu.Unlock()

	e := f.cameras[k]
	if e == nil {
		e = &entry{}
		f.cameras[k] = e
	}

	e.record(frame, from, now)
	f.version++
}

// RecordTracked keeps packet's frame as its autoref's latest. A packet
// without a frame only refreshes the source's name.
func (f *Feed) RecordTracked(packet *vision.TrackerWrapperPacket, from net.IP, now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()

	e := f.trackers[packet.GetUuid()]
	if e == nil {
		e = &entry{}
		f.trackers[packet.GetUuid()] = e
	}

	e.name = packet.GetSourceName()

	if frame := packet.GetTrackedFrame(); frame != nil {
		e.record(frame, from, now)
	} else if e.frame == nil {
		// Nothing to draw yet: don't list it.
		delete(f.trackers, packet.GetUuid())

		return
	}

	f.version++
}

// Version changes whenever a frame is recorded, so a publisher can skip
// snapshots that would repeat the last one.
func (f *Feed) Version() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.version
}

// Snapshot lists every camera and autoref heard within Forget, cameras by
// camera_id then address and autorefs by name then uuid.
func (f *Feed) Snapshot(now time.Time) Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()

	s := Snapshot{At: now, Cameras: []Camera{}, Trackers: []Tracker{}}

	for k, e := range f.cameras {
		if now.Sub(e.last) > Forget {
			delete(f.cameras, k)

			continue
		}

		c := Camera{CameraID: k.cameraID, Address: e.address, ReceivedAt: e.last}
		c.FPS, c.Receiving = e.rate(now)

		if c.Receiving {
			c.Frame = e.json()
		}

		s.Cameras = append(s.Cameras, c)
	}

	for uuid, e := range f.trackers {
		if now.Sub(e.last) > Forget {
			delete(f.trackers, uuid)

			continue
		}

		t := Tracker{UUID: uuid, SourceName: e.name, Address: e.address, ReceivedAt: e.last}
		t.FPS, t.Receiving = e.rate(now)

		if t.Receiving {
			t.Frame = e.json()
		}

		s.Trackers = append(s.Trackers, t)
	}

	slices.SortFunc(s.Cameras, func(a, b Camera) int {
		return cmp.Or(cmp.Compare(a.CameraID, b.CameraID), cmp.Compare(a.Address, b.Address))
	})
	slices.SortFunc(s.Trackers, func(a, b Tracker) int {
		return cmp.Or(cmp.Compare(a.SourceName, b.SourceName), cmp.Compare(a.UUID, b.UUID))
	})

	return s
}

func (e *entry) rate(now time.Time) (fps int, receiving bool) {
	e.arrivals = trim(e.arrivals, now)

	return len(e.arrivals), now.Sub(e.last) <= Window
}

// trim drops arrivals older than Window.
func trim(arrivals []time.Time, now time.Time) []time.Time {
	i := 0
	for i < len(arrivals) && now.Sub(arrivals[i]) > Window {
		i++
	}

	return arrivals[i:]
}
