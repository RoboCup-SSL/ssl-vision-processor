package livefield

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

var (
	t0    = time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	vpA   = net.IPv4(10, 0, 0, 5)
	vpB   = net.IPv4(10, 0, 0, 6)
	autoA = net.IPv4(10, 0, 0, 20)
)

func detection(cameraID uint32, frame uint32) *vision.SSL_DetectionFrame {
	return &vision.SSL_DetectionFrame{
		FrameNumber: proto.Uint32(frame),
		TCapture:    proto.Float64(1),
		TSent:       proto.Float64(1),
		CameraId:    proto.Uint32(cameraID),
	}
}

func tracked(uuid, name string, frame uint32) *vision.TrackerWrapperPacket {
	return &vision.TrackerWrapperPacket{
		Uuid:       proto.String(uuid),
		SourceName: proto.String(name),
		TrackedFrame: &vision.TrackedFrame{
			FrameNumber: proto.Uint32(frame),
			Timestamp:   proto.Float64(1),
		},
	}
}

func TestSnapshotKeepsTheLatestFramePerCameraAndAddress(t *testing.T) {
	f := NewFeed()

	f.RecordDetection(detection(1, 10), vpA, t0)
	f.RecordDetection(detection(1, 11), vpA, t0.Add(10*time.Millisecond))
	f.RecordDetection(detection(0, 5), vpA, t0.Add(20*time.Millisecond))
	// A second vision_processor claiming camera 1 is listed separately.
	f.RecordDetection(detection(1, 99), vpB, t0.Add(30*time.Millisecond))

	s := f.Snapshot(t0.Add(40 * time.Millisecond))

	if len(s.Cameras) != 3 {
		t.Fatalf("cameras = %+v, want 3", s.Cameras)
	}

	want := []struct {
		id      uint32
		address string
		frame   float64
		fps     int
	}{{0, "10.0.0.5", 5, 1}, {1, "10.0.0.5", 11, 2}, {1, "10.0.0.6", 99, 1}}

	for i, w := range want {
		c := s.Cameras[i]

		var frame struct{ FrameNumber float64 }
		if err := json.Unmarshal(c.Frame, &frame); err != nil {
			t.Fatalf("camera %d frame: %v (%s)", i, err, c.Frame)
		}

		if c.CameraID != w.id || c.Address != w.address || frame.FrameNumber != w.frame || c.FPS != w.fps || !c.Receiving {
			t.Errorf("cameras[%d] = %+v frame %v, want %+v", i, c, frame.FrameNumber, w)
		}
	}
}

func TestSilentSourcesLoseTheirFrameThenAreForgotten(t *testing.T) {
	f := NewFeed()
	f.RecordDetection(detection(0, 1), vpA, t0)
	f.RecordTracked(tracked("u1", "TIGERs AutoRef", 1), autoA, t0)

	s := f.Snapshot(t0.Add(Window + time.Millisecond))
	if len(s.Cameras) != 1 || s.Cameras[0].Receiving || s.Cameras[0].Frame != nil || s.Cameras[0].FPS != 0 {
		t.Fatalf("cameras = %+v, want one, silent, without a frame", s.Cameras)
	}

	if len(s.Trackers) != 1 || s.Trackers[0].Receiving || s.Trackers[0].Frame != nil {
		t.Fatalf("trackers = %+v, want one, silent, without a frame", s.Trackers)
	}

	if s := f.Snapshot(t0.Add(Forget + time.Millisecond)); len(s.Cameras) != 0 || len(s.Trackers) != 0 {
		t.Fatalf("snapshot = %+v, want everything forgotten", s)
	}
}

func TestTrackersAreKeyedByUUID(t *testing.T) {
	f := NewFeed()
	f.RecordTracked(tracked("u2", "ER-Force", 1), autoA, t0)
	f.RecordTracked(tracked("u1", "TIGERs AutoRef", 1), autoA, t0)
	f.RecordTracked(tracked("u1", "TIGERs AutoRef", 2), autoA, t0.Add(time.Millisecond))

	s := f.Snapshot(t0.Add(2 * time.Millisecond))
	if len(s.Trackers) != 2 || s.Trackers[0].SourceName != "ER-Force" || s.Trackers[1].UUID != "u1" || s.Trackers[1].FPS != 2 {
		t.Fatalf("trackers = %+v, want ER-Force then TIGERs (2 frames)", s.Trackers)
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Trackers []struct{ Frame struct{ FrameNumber float64 } }
	}
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.Trackers[1].Frame.FrameNumber != 2 {
		t.Fatalf("encoded = %s (%v), want TIGERs' latest frame inline", data, err)
	}
}

func TestATrackerPacketWithoutAFrameIsNotListed(t *testing.T) {
	f := NewFeed()
	f.RecordTracked(&vision.TrackerWrapperPacket{Uuid: proto.String("u1")}, autoA, t0)

	if s := f.Snapshot(t0); len(s.Trackers) != 0 {
		t.Fatalf("trackers = %+v, want none until a frame arrives", s.Trackers)
	}
}

func TestVersionChangesOnEveryRecord(t *testing.T) {
	f := NewFeed()
	v := f.Version()

	f.RecordDetection(detection(0, 1), vpA, t0)

	if f.Version() == v {
		t.Fatal("version unchanged after a detection")
	}
}

func TestFeedIsSafeForConcurrentUse(t *testing.T) {
	f := NewFeed()

	var wg sync.WaitGroup

	for i := range 4 {
		wg.Go(func() {
			for n := range 200 {
				now := t0.Add(time.Duration(n) * time.Millisecond)
				f.RecordDetection(detection(uint32(i), uint32(n)), vpA, now)
				f.RecordTracked(tracked("u1", "TIGERs AutoRef", uint32(n)), autoA, now)
				_ = f.Snapshot(now)
			}
		})
	}

	wg.Wait()
}
