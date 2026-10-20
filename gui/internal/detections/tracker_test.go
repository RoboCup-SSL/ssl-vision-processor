package detections

import (
	"net"
	"testing"
	"time"
)

func TestTrackerCountsFPSPerCameraAndAddress(t *testing.T) {
	tr := NewTracker()
	start := time.Unix(1000, 0)
	a, b := net.IPv4(10, 0, 0, 5), net.IPv4(10, 0, 0, 6)

	// 30 frames over the last second from a as camera 1; b also claims 1.
	for i := range 30 {
		tr.Record(1, a, start.Add(time.Duration(i)*time.Second/30))
	}

	tr.Record(1, b, start.Add(900*time.Millisecond))
	tr.Record(0, a, start.Add(900*time.Millisecond))

	got := tr.Sources(start.Add(time.Second))
	if len(got) != 3 {
		t.Fatalf("sources = %+v, want 3", got)
	}

	if got[0].CameraID != 0 || got[1].Address != "10.0.0.5" || got[1].FPS < 29 || got[2].Address != "10.0.0.6" {
		t.Errorf("sources = %+v, want camera 0, then camera 1 from .5 at ~30 fps and from .6", got)
	}
}

func TestTrackerGoesSilentThenForgets(t *testing.T) {
	tr := NewTracker()
	start := time.Unix(1000, 0)
	tr.Record(2, net.IPv4(10, 0, 0, 5), start)

	if s := tr.Sources(start.Add(5 * time.Second)); len(s) != 1 || s[0].Receiving || s[0].FPS != 0 {
		t.Fatalf("after 5 s: %+v, want one silent source", s)
	}

	if s := tr.Sources(start.Add(Forget + time.Second)); len(s) != 0 {
		t.Fatalf("after Forget: %+v, want none", s)
	}
}
