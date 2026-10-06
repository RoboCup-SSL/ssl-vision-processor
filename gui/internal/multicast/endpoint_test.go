package multicast

import (
	"context"
	"fmt"
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
	// problems is what each successive open reports as unhealthy; opens
	// past the end are healthy.
	problems []string
}

func newFakeNet() *fakeNet {
	return &fakeNet{open: make(chan string, 8)}
}

func (f *fakeNet) opener(address string, _ []Interface, opts Options, receive func([]byte, *net.UDPAddr)) sockets {
	f.mu.Lock()
	f.opened = append(f.opened, address)
	f.receive = receive

	problem := ""
	if len(f.problems) > 0 {
		problem, f.problems = f.problems[0], f.problems[1:]
	}
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

	return sockets{
		send: send,
		stop: func() {
			f.mu.Lock()
			defer f.mu.Unlock()

			f.stopped = append(f.stopped, address)
		},
		health: func() string { return problem },
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

func startEndpoint(t *testing.T, address string, opts Options, consume Consumer, problems ...string) (*Endpoint, *fakeNet, func()) {
	t.Helper()

	f := newFakeNet()
	f.problems = problems
	e := NewEndpoint("test", address, []Interface{{Name: "eth0", Address: "10.0.0.2", Used: true}}, opts, consume)
	e.open = f.opener
	e.retry = 20 * time.Millisecond

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

func countAll([]byte, *net.UDPAddr) bool { return true }

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
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, func(data []byte, _ *net.UDPAddr) bool {
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

	var tracked []string

	visionConsume := VisionConsumer(
		func(g *vision.SSL_GeometryData) { got = g },
		func(id uint32, from net.IP) { tracked = append(tracked, fmt.Sprintf("%d@%s", id, from)) },
	)
	from := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 5), Port: 10006}
	consume := func(data []byte) bool { return visionConsume(data, from) }

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
			CameraId:    proto.Uint32(2),
		},
	}

	if !consume(marshal(t, detection)) {
		t.Fatal("detection packet not counted as heard")
	}

	if len(tracked) != 1 || tracked[0] != "2@10.0.0.5" {
		t.Fatalf("tracked = %v, want camera 2 from 10.0.0.5 once", tracked)
	}

	if consume([]byte("not a protobuf message")) {
		t.Fatal("malformed data counted as heard")
	}
}

func TestRefereeConsumer(t *testing.T) {
	var yellow, blue string

	refereeConsume := RefereeConsumer(func(y, b string) { yellow, blue = y, b })
	consume := func(data []byte) bool { return refereeConsume(data, nil) }

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

	referee := &gamecontroller.Referee{
		PacketTimestamp:  proto.Uint64(1),
		Stage:            gamecontroller.Referee_NORMAL_FIRST_HALF.Enum(),
		Command:          gamecontroller.Referee_HALT.Enum(),
		CommandCounter:   proto.Uint32(1),
		CommandTimestamp: proto.Uint64(1),
		Yellow:           team("ER-Force"),
		Blue:             team("TIGERs Mannheim"),
	}

	if !consume(marshal(t, referee)) {
		t.Fatal("referee message not counted")
	}

	if yellow != "ER-Force" || blue != "TIGERs Mannheim" {
		t.Fatalf("tracked yellow=%q blue=%q, want ER-Force and TIGERs Mannheim", yellow, blue)
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

func TestEndpointRetriesUntilHealthy(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, countAll,
		"no usable network interface", "no usable network interface")
	defer stop()

	// The fake signals the open before the endpoint has checked its health.
	waitFor(t, "the problem to be reported", func() bool {
		return e.Status(time.Now()).Problem == "no usable network interface"
	})

	// Two failing opens, then a healthy one: the endpoint retries on its own
	// though nothing about the address or interfaces changed.
	f.waitOpen(t, "224.5.23.2:10006")
	f.waitOpen(t, "224.5.23.2:10006")

	waitFor(t, "the problem to clear after a healthy open", func() bool {
		return e.Status(time.Now()).Problem == ""
	})

	select {
	case got := <-f.open:
		t.Fatalf("reopened %s after becoming healthy", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestEndpointReopenForcesNewSockets(t *testing.T) {
	e, f, stop := startEndpoint(t, "224.5.23.2:10006", Options{}, countAll)
	defer stop()

	e.Reopen()
	f.waitOpen(t, "224.5.23.2:10006")
}

// The overnight crash: sockets opened with no usable interface (a suspend
// in progress), then reopened when the interface came back. With real
// sockets, not the fake: it was the stop of a receiver that never joined.
func TestEndpointSurvivesLosingEveryInterface(t *testing.T) {
	e := NewEndpoint("test", "224.5.23.250:19950", nil, Options{Send: true}, countAll)
	e.retry = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- e.Run(ctx) }()

	time.Sleep(60 * time.Millisecond) // a few retries with nothing usable

	if p := e.Status(time.Now()).Problem; p == "" {
		t.Error("no problem reported with no usable interface")
	}

	back, _ := Select(ListInterfaces(), true, nil)
	e.SetInterfaces(back) // whatever this machine has; it must not panic
	time.Sleep(60 * time.Millisecond)
	e.SetInterfaces(nil)
	time.Sleep(60 * time.Millisecond)

	cancel()

	if err := <-done; err != context.Canceled {
		t.Fatalf("Run returned %v", err)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(5 * time.Millisecond)
	}
}
