package main

// The host hearing real multicast over loopback: detections and referee
// messages reach /api/network and /api/alerts. Skipped unless loopback
// multicast is on (scripts/lo-multicast.sh on).

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/gamecontroller"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast/mctest"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// loopbackHost is testHost with the vision and game controller sockets on
// fresh groups, on loopback only, with a two-team height table, running.
func loopbackHost(t *testing.T) (visionGroup, gcGroup string, srv *httptest.Server) {
	t.Helper()

	lo := mctest.RequireLoopback(t)
	visionGroup, gcGroup = mctest.Group(t), mctest.Group(t)

	heights := filepath.Join(t.TempDir(), "robot-heights.yml")
	if err := os.WriteFile(heights, []byte("ER-Force: 148\nTIGERs Mannheim: 143\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var skip []string

	for _, i := range multicast.ListInterfaces() {
		if i.Name != lo[0].Name {
			skip = append(skip, i.Name)
		}
	}

	h, srv := testHost(t, func(doc *config.Document) {
		vHost, vPort := splitGroup(t, visionGroup)
		gHost, gPort := splitGroup(t, gcGroup)
		doc.Defaults["network"] = map[string]any{"vision_ip": vHost, "vision_port": vPort, "gc_ip": gHost, "gc_port": gPort}
		doc.Defaults["bot_heights_file"] = heights

		auto := false
		doc.Host = &config.Host{Interfaces: &config.Interfaces{Auto: &auto, Skip: skip}}
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{}, 2)

	for _, e := range []*multicast.Endpoint{h.sockets.vision, h.sockets.gc} {
		go func() {
			_ = e.Run(ctx)
			done <- struct{}{}
		}()
	}

	t.Cleanup(func() {
		cancel()
		<-done
		<-done
	})

	return visionGroup, gcGroup, srv
}

func TestTheHostHearsDetectionsAndTheReferee(t *testing.T) {
	visionGroup, gcGroup, srv := loopbackHost(t)

	detection := func(camera uint32) []byte {
		return marshal(t, &vision.SSL_WrapperPacket{Detection: &vision.SSL_DetectionFrame{
			FrameNumber: proto.Uint32(1),
			TCapture:    proto.Float64(1),
			TSent:       proto.Float64(1),
			CameraId:    proto.Uint32(camera),
		}})
	}

	network := func() NetworkState {
		var n NetworkState
		if err := json.NewDecoder(mustGetBody(t, srv.URL+"/api/network")).Decode(&n); err != nil {
			t.Fatal(err)
		}

		return n
	}

	heard := func(camera uint32) func() bool {
		return func() bool {
			return slices.ContainsFunc(network().Cameras, func(s detections.Source) bool {
				return s.CameraID == camera && s.Address == "127.0.0.1" && s.Receiving
			})
		}
	}

	// Camera 1 is in the layout; camera 5 isn't (two regions).
	mctest.SendUntil(t, visionGroup, detection(1), 3*time.Second, heard(1))
	mctest.SendUntil(t, visionGroup, detection(5), 3*time.Second, heard(5))

	mctest.SendUntil(t, gcGroup, marshal(t, refereeMessage("ER-Force", "Nobody FC")), 3*time.Second, func() bool {
		return network().Referee.Yellow.Name == "ER-Force"
	})

	n := network()
	if !n.Vision.Receiving || !n.GC.Receiving {
		t.Errorf("vision receiving=%v, gc receiving=%v, want both", n.Vision.Receiving, n.GC.Receiving)
	}

	if h := n.Referee.Yellow.Height; h == nil || *h != 148 {
		t.Errorf("yellow ER-Force height = %v, want 148 from the table", h)
	}

	if n.Referee.Blue.Name != "Nobody FC" || n.Referee.Blue.Height != nil {
		t.Errorf("blue = %+v, want Nobody FC, not in the table", n.Referee.Blue)
	}

	// With real sockets and real traffic, the responses still match the spec.
	for _, path := range []string{"/api/network", "/api/alerts"} {
		body, err := io.ReadAll(mustGetBody(t, srv.URL+path))
		if err != nil {
			t.Fatal(err)
		}

		checkResponse(t, path, body)
	}

	var list []alerts.Alert
	if err := json.NewDecoder(mustGetBody(t, srv.URL+"/api/alerts")).Decode(&list); err != nil {
		t.Fatal(err)
	}

	ids := map[string]alerts.Alert{}
	for _, a := range list {
		ids[a.ID] = a
	}

	if _, ok := ids["unplaced-5-127.0.0.1"]; !ok {
		t.Errorf("alerts = %v, want camera 5 unplaced", keys(ids))
	}

	if a, ok := ids["team-height-blue"]; !ok || a.Params["issue"] != alerts.IssueNotInTable {
		t.Errorf("team-height-blue = %+v, want issue notInTable", a)
	}

	for _, quiet := range []string{"silent-vision", "team-height-yellow", "heights-unreadable"} {
		if _, ok := ids[quiet]; ok {
			t.Errorf("alerts include %s, want it gone now that packets arrive", quiet)
		}
	}
}

func refereeMessage(yellow, blue string) *gamecontroller.Referee {
	team := func(name string) *gamecontroller.Referee_TeamInfo {
		return &gamecontroller.Referee_TeamInfo{
			Name:        proto.String(name),
			Score:       proto.Uint32(0),
			RedCards:    proto.Uint32(0),
			YellowCards: proto.Uint32(0),
			Timeouts:    proto.Uint32(4),
			TimeoutTime: proto.Uint32(300),
			Goalkeeper:  proto.Uint32(0),
		}
	}

	return &gamecontroller.Referee{
		PacketTimestamp:  proto.Uint64(1),
		Stage:            gamecontroller.Referee_NORMAL_FIRST_HALF.Enum(),
		Command:          gamecontroller.Referee_HALT.Enum(),
		CommandCounter:   proto.Uint32(1),
		CommandTimestamp: proto.Uint64(1),
		Yellow:           team(yellow),
		Blue:             team(blue),
	}
}

func marshal(t *testing.T, m proto.Message) []byte {
	t.Helper()

	data, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func splitGroup(t *testing.T, group string) (string, int) {
	t.Helper()

	host, port, err := net.SplitHostPort(group)
	if err != nil {
		t.Fatal(err)
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}

	return host, p
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.Sort(out)

	return out
}

// mustGetBody is url's body, closed when t ends.
func mustGetBody(t *testing.T, url string) io.Reader {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { resp.Body.Close() })

	return resp.Body
}
