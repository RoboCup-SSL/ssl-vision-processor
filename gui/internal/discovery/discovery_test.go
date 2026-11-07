package discovery

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

func announce(name string, cam uint32) *vision.SSL_VPConfig {
	return &vision.SSL_VPConfig{Instance: proto.String(name), CameraId: proto.Uint32(cam)}
}

func addr(s string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr("udp4", s)
	if err != nil {
		panic(err)
	}

	return a
}

func TestInstancesSortedByCameraThenName(t *testing.T) {
	r := NewRegistry()
	now := time.Now()

	r.Record(announce("vp-b", 1), addr("10.0.0.2:5000"), now)
	r.Record(announce("vp-c", 0), addr("10.0.0.3:5000"), now)
	r.Record(announce("vp-a", 1), addr("10.0.0.1:5000"), now)

	got := r.Instances(now)

	want := []string{"vp-c", "vp-a", "vp-b"}
	if len(got) != len(want) {
		t.Fatalf("got %d instances, want %d", len(got), len(want))
	}

	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("instance %d is %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestInstanceGoesStaleThenForgotten(t *testing.T) {
	r := NewRegistry()
	start := time.Now()

	r.Record(announce("vp", 0), addr("10.0.0.1:5000"), start)

	if got := r.Instances(start.Add(Stale)); len(got) != 1 || !got[0].Live {
		t.Fatalf("at Stale: got %+v, want one live instance", got)
	}

	if got := r.Instances(start.Add(Stale + time.Millisecond)); len(got) != 1 || got[0].Live {
		t.Fatalf("past Stale: got %+v, want one instance, not live", got)
	}

	if got := r.Instances(start.Add(Forget + time.Millisecond)); len(got) != 0 {
		t.Fatalf("past Forget: got %+v, want none", got)
	}
}

func TestLatestAnnounceWins(t *testing.T) {
	r := NewRegistry()
	now := time.Now()

	r.Record(announce("vp", 0), addr("10.0.0.1:5000"), now)
	r.Record(announce("vp", 2), addr("10.0.0.9:6000"), now.Add(time.Second))

	got := r.Instances(now.Add(time.Second))
	if len(got) != 1 {
		t.Fatalf("got %d instances, want 1", len(got))
	}

	if got[0].CameraID != 2 || got[0].Address != "10.0.0.9:6000" {
		t.Errorf("got camera %d from %s, want camera 2 from 10.0.0.9:6000", got[0].CameraID, got[0].Address)
	}
}

func TestUnnamedAnnounceDropped(t *testing.T) {
	r := NewRegistry()
	now := time.Now()

	r.Record(&vision.SSL_VPConfig{Instance: proto.String("")}, addr("10.0.0.1:5000"), now)

	if got := r.Instances(now); len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}

func TestConfigIsProtojson(t *testing.T) {
	r := NewRegistry()
	now := time.Now()

	c := announce("vp", 1)
	c.Camera = &vision.SSL_VPConfigCamera{Driver: vision.SSL_VPConfigCameraDriver_OPENCV.Enum()}
	r.Record(c, addr("10.0.0.1:5000"), now)

	data, err := json.Marshal(r.Instances(now)[0])
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Config struct {
			Instance string `json:"instance"`
			CameraID int    `json:"cameraId"`
			Camera   struct {
				Driver string `json:"driver"`
			} `json:"camera"`
		} `json:"config"`
	}

	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("%s: %v", data, err)
	}

	if got.Config.Instance != "vp" || got.Config.CameraID != 1 || got.Config.Camera.Driver != "OPENCV" {
		t.Errorf("config came out as %s", data)
	}
}
