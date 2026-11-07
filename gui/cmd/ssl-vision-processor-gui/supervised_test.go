package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/peers"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// TestFakeVisionProcessor stands in for vision_processor under -vp: it notes
// each start beside its config and runs until SIGTERM.
func TestFakeVisionProcessor(t *testing.T) {
	if os.Getenv("FAKE_VISION_PROCESSOR") != "1" {
		return
	}

	path := os.Args[len(os.Args)-1]

	f, _ := os.OpenFile(path+".starts", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	f.WriteString("start\n")
	f.Close()

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)

	select {
	case <-term:
	case <-time.After(30 * time.Second):
	}

	os.Exit(0)
}

// An edit to a startup-only setting through the API restarts the camera's
// vision_processor; an edit to a live one only rewrites its config.yml.
func TestEditRestartsSupervisedVisionProcessor(t *testing.T) {
	t.Setenv("FAKE_VISION_PROCESSOR", "1")

	h, srv := testHostWith(t, hostOptions{
		imgDir:    t.TempDir(),
		vpCommand: []string{os.Args[0], "-test.run=^TestFakeVisionProcessor$", "--"},
	}, func(d *config.Document) {
		d.Cameras[0].ConfigPath = "cam0.yml"
	})
	cfg := h.store.InDir("cam0.yml")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- h.procs.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	starts := func() int {
		data, _ := os.ReadFile(cfg + ".starts")

		return strings.Count(string(data), "start")
	}

	waitFor := func(what string, cond func() bool) {
		t.Helper()

		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}

			time.Sleep(20 * time.Millisecond)
		}
	}

	running := func() bool {
		st := h.procs.Statuses()

		return len(st) == 1 && st[0].State == "running"
	}

	waitFor("first start", func() bool { return starts() == 1 && running() })

	resp, err := http.Get(srv.URL + "/api/processes")
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	checkResponse(t, "/api/processes", body)

	if r := patch(t, srv, `{"defaults":{"thresholds":{"score":9}}}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("live patch = %d", r.StatusCode)
	}

	time.Sleep(1500 * time.Millisecond)

	if n := starts(); n != 1 {
		t.Fatalf("a live setting restarted it: %d starts", n)
	}

	if data, _ := os.ReadFile(cfg); !strings.Contains(string(data), "score: 9") {
		t.Errorf("config.yml wasn't rewritten:\n%s", data)
	}

	if r := patch(t, srv, `{"defaults":{"camera":{"exposure":7}}}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("startup patch = %d", r.StatusCode)
	}

	waitFor("restart after edit", func() bool { return starts() == 2 && running() })

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/processes/0/restart", nil)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("restart: %v %v", resp, err)
	}

	waitFor("manual restart", func() bool { return starts() == 3 && running() })
}

func TestRestartUnsupervisedIs404(t *testing.T) {
	h, _ := testServer(t)

	if rec := do(t, h, http.MethodPost, "/api/processes/0/restart", nil); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// A field change unlocks the camera's calibration, solved against the old
// field, and restarts its vision_processor so it calibrates again.
func TestFieldChangeUnlocksAndRestarts(t *testing.T) {
	t.Setenv("FAKE_VISION_PROCESSOR", "1")

	h, srv := testHostWith(t, hostOptions{
		imgDir:    t.TempDir(),
		vpCommand: []string{os.Args[0], "-test.run=^TestFakeVisionProcessor$", "--"},
	}, func(d *config.Document) {
		d.Cameras[0].ConfigPath = "cam0.yml"
	})
	cfg := h.store.InDir("cam0.yml")

	if doc, _ := h.store.Working(); doc.Cameras[0].Calibration == nil {
		t.Fatal("fixture camera 0 has no locked calibration")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- h.procs.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	starts := func() int {
		data, _ := os.ReadFile(cfg + ".starts")

		return strings.Count(string(data), "start")
	}

	waitFor := func(what string, cond func() bool) {
		t.Helper()

		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}

			time.Sleep(20 * time.Millisecond)
		}
	}

	waitFor("first start", func() bool { return starts() == 1 })

	if r := patch(t, srv, `{"field":{"fieldLength":8000}}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("field patch = %d", r.StatusCode)
	}

	waitFor("restart", func() bool { return starts() == 2 })

	if doc, _ := h.store.Working(); doc.Cameras[0].Calibration != nil {
		t.Error("calibration still locked after the field changed")
	}

	if h.geom.LiveCalibration(0) != nil {
		t.Error("host still holds a calibration for camera 0")
	}
}

// The host never publishes a calibration that can't be a real camera's (the
// test fixture's sits under the field), whoever sends it; a real one is
// absorbed, and a second sender for it is a conflict.
func TestAbsorberFiltersAndWatchesCalibrations(t *testing.T) {
	h, _ := testHost(t)
	w := newPeerWatches()
	absorb := w.absorber(h.geom)

	fixture := &vision.SSL_GeometryCameraCalibration{
		CameraId: proto.Uint32(3), FocalLength: proto.Float32(1184.5924),
		PrincipalPointX: proto.Float32(960), PrincipalPointY: proto.Float32(540), Distortion: proto.Float32(-0.1),
		Q0: proto.Float32(0), Q1: proto.Float32(0), Q2: proto.Float32(0), Q3: proto.Float32(1),
		Tx: proto.Float32(0), Ty: proto.Float32(0), Tz: proto.Float32(4000),
	}
	absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{fixture}}, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 41234})

	if h.geom.LiveCalibration(3) != nil {
		t.Fatal("absorbed a calibration under the field")
	}

	good := proto.Clone(fixture).(*vision.SSL_GeometryCameraCalibration)
	good.Q0, good.Q3 = proto.Float32(1), proto.Float32(0) // looking down from 4 m

	vp := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 10006}
	other := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: 50000}

	absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{good}}, vp)

	if h.geom.LiveCalibration(3) == nil {
		t.Fatal("didn't absorb a real calibration")
	}

	if c := w.Conflicts(time.Now()); len(c) != 0 {
		t.Fatalf("one sender is a conflict: %v", c)
	}

	absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{good}}, other)

	c := w.Conflicts(time.Now())
	if len(c) != 1 || c[0].Kind != peers.KindCalibration || *c[0].CameraID != 3 || len(c[0].Senders) != 2 {
		t.Fatalf("Conflicts = %+v, want camera 3's calibration from both", c)
	}
}
