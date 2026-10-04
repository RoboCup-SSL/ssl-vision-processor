package multicast

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/gamecontroller"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// These tests swap Endpoint.open for a fake rather than joining a real
// multicast group, which sandboxes often block.

// fakeNet records what an Endpoint opened and lets the test deliver datagrams
// to whichever socket is current.
type fakeNet struct {
	mu      sync.Mutex
	opened  []string
	stopped []string
	receive func([]byte, *net.UDPAddr)
	sent    [][]byte
	open    chan string
}

func newFakeNet() *fakeNet {
	return &fakeNet{open: make(chan string, 8)}
}

func (f *fakeNet) opener(address string, _ []Interface, opts Options, receive func([]byte, *net.UDPAddr)) (func([]byte), func()) {
	f.mu.Lock()
	f.opened = append(f.opened, address)
	f.receive = receive
	f.mu.Unlock()

	f.open <- address

	var send func([]byte)
	if opts.Send {
		send = func(data []byte) {
			f.mu.Lock()
			defer f.mu.Unlock()

			f.sent = append(f.sent, data)
		}
	}

	return send, func() {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.stopped = append(f.stopped, address)
	}
}

func (f *fakeNet) deliver(data []byte) {
	f.mu.Lock()
	receive := f.receive
	f.mu.Unlock()

	receive(data, &net.UDPAddr{IP: net.IPv4(10, 0, 0, 7), Port: 10006})
}

func (f *fakeNet) waitOpen(t *testing.T, want string) {
	t.Helper()

	select {
	case got := <-f.open:
		if got != want {
			t.Fatalf("opened %s, want %s", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s to open", want)
	}
}

func startEndpoint(t *testing.T, address string, opts Options, consume Consumer) (*Endpoint, *fakeNet, func()) {
	t.Helper()

	f := newFakeNet()
	e := NewEndpoint("test", address, []Interface{{Name: "eth0", Address: "10.0.0.2", Used: true}}, opts, consume)
	e.open = f.opener

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- e.Run(ctx) }()

	f.waitOpen(t, address)

	return e, f, func() {
		cancel()

		if err := <-done; err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	}
}

func countAll(data []byte) bool { return true }

func TestEndpointReopensOnNewAddress(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, countAll)

	e.SetAddress("224.5.23.9:10010")
	f.waitOpen(t, "224.5.23.9:10010")
	stop()

	f.mu.Lock()
	defer f.mu.Unlock()

	wantStopped := []string{"224.5.23.2:10006", "224.5.23.9:10010"}
	if len(f.stopped) != 2 || f.stopped[0] != wantStopped[0] || f.stopped[1] != wantStopped[1] {
		t.Fatalf("stopped %v, want %v", f.stopped, wantStopped)
	}
}

func TestEndpointSameAddressIsNoop(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, countAll)

	e.SetAddress("224.5.23.2:10006")

	select {
	case got := <-f.open:
		t.Fatalf("reopened on %s for an unchanged address", got)
	case <-time.After(100 * time.Millisecond):
	}

	stop()
}

func TestEndpointStatusCountsHeardAndResetsOnMove(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, func(data []byte) bool {
		return string(data) == "valid"
	})
	defer stop()

	f.deliver([]byte("valid"))
	f.deliver([]byte("noise"))

	s := e.Status(time.Now())
	if s.Heard != 1 || !s.Receiving || s.Source != "10.0.0.7" || s.Address != "224.5.23.2:10006" {
		t.Fatalf("status = %+v, want 1 heard from 10.0.0.7, receiving", s)
	}

	if e.Status(time.Now().Add(ReceivingWindow + time.Second)).Receiving {
		t.Fatal("still receiving after the window passed")
	}

	e.SetAddress("224.5.23.9:10010")

	if s := e.Status(time.Now()); s.Heard != 0 || s.LastHeard != nil || s.Address != "224.5.23.9:10010" {
		t.Fatalf("status after move = %+v, want reset counters on the new address", s)
	}
}

func TestEndpointDropsLatePacketFromOldSocket(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, countAll)
	defer stop()

	f.mu.Lock()
	oldReceive := f.receive
	f.mu.Unlock()

	e.SetAddress("224.5.23.9:10010")
	f.waitOpen(t, "224.5.23.9:10010")

	oldReceive([]byte("late"), nil)

	if s := e.Status(time.Now()); s.Heard != 0 {
		t.Fatalf("late packet from the old socket counted: %+v", s)
	}
}

func TestEndpointSendGoesToCurrentSocket(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{Send: true}, countAll)
	defer stop()

	e.Send([]byte("geometry"))

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.sent) != 1 || string(f.sent[0]) != "geometry" {
		t.Fatalf("sent %q, want one geometry packet", f.sent)
	}
}

func marshal(t *testing.T, m proto.Message) []byte {
	t.Helper()

	data, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	return data
}

func TestVisionConsumerAbsorbsGeometryButCountsOnlyDetections(t *testing.T) {
	var got *vision.SSL_GeometryData

	consume := VisionConsumer(func(g *vision.SSL_GeometryData) { got = g })

	// SSL_GeometryData.field is a required proto2 field, so Marshal needs a
	// complete SSL_GeometryFieldSize.
	geometry := &vision.SSL_WrapperPacket{
		Geometry: &vision.SSL_GeometryData{
			Field: &vision.SSL_GeometryFieldSize{
				FieldLength:   proto.Int32(9000),
				FieldWidth:    proto.Int32(6000),
				GoalWidth:     proto.Int32(1000),
				GoalDepth:     proto.Int32(180),
				BoundaryWidth: proto.Int32(300),
			},
		},
	}

	if consume(marshal(t, geometry)) {
		t.Fatal("geometry-only packet counted as heard")
	}

	if got.GetField().GetFieldLength() != 9000 {
		t.Fatalf("absorb called with %v, want field_length 9000", got)
	}

	detection := &vision.SSL_WrapperPacket{
		Detection: &vision.SSL_DetectionFrame{
			FrameNumber: proto.Uint32(1),
			TCapture:    proto.Float64(1),
			TSent:       proto.Float64(1),
			CameraId:    proto.Uint32(0),
		},
	}

	if !consume(marshal(t, detection)) {
		t.Fatal("detection packet not counted as heard")
	}

	if consume([]byte("not a protobuf message")) {
		t.Fatal("malformed data counted as heard")
	}
}

func TestRefereeConsumer(t *testing.T) {
	consume := RefereeConsumer()

	team := func() *gamecontroller.Referee_TeamInfo {
		return &gamecontroller.Referee_TeamInfo{
			Name:        proto.String("t"),
			Score:       proto.Uint32(0),
			RedCards:    proto.Uint32(0),
			YellowCards: proto.Uint32(0),
			Timeouts:    proto.Uint32(4),
			TimeoutTime: proto.Uint32(300),
			Goalkeeper:  proto.Uint32(0),
		}
	}

	referee := &gamecontroller.Referee{
		PacketTimestamp:  proto.Uint64(1),
		Stage:            gamecontroller.Referee_NORMAL_FIRST_HALF.Enum(),
		Command:          gamecontroller.Referee_HALT.Enum(),
		CommandCounter:   proto.Uint32(1),
		CommandTimestamp: proto.Uint64(1),
		Yellow:           team(),
		Blue:             team(),
	}

	if !consume(marshal(t, referee)) {
		t.Fatal("referee message not counted")
	}

	if consume(marshal(t, &vision.SSL_WrapperPacket{})) {
		t.Fatal("wrapper packet counted as a referee message")
	}
}

func TestModeOf(t *testing.T) {
	cases := map[string]string{
		"224.5.23.2:10006":      "multicast",
		"239.1.2.3:10006":       "multicast",
		"255.255.255.255:10006": "port",
		"192.168.30.255:10003":  "port",
		"10.0.0.5:10006":        "port",
	}

	for address, want := range cases {
		if got := modeOf(address); got != want {
			t.Errorf("modeOf(%s) = %s, want %s", address, got, want)
		}
	}
}

func TestEndpointReopensOnlyWhenUsedInterfacesChange(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, countAll)
	defer stop()

	// Same used set, different unused details: no reopen.
	e.SetInterfaces([]Interface{
		{Name: "eth0", Address: "10.0.0.2", Used: true},
		{Name: "docker0", Address: "172.17.0.1"},
	})

	select {
	case got := <-f.open:
		t.Fatalf("reopened on %s for an unchanged selection", got)
	case <-time.After(100 * time.Millisecond):
	}

	e.SetInterfaces([]Interface{
		{Name: "eth0", Address: "10.0.0.2", Used: true},
		{Name: "docker0", Address: "172.17.0.1", Used: true},
	})
	f.waitOpen(t, "224.5.23.2:10006")
}
