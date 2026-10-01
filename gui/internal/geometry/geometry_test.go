package geometry

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// testGeometry loads a *copy* of the fixture into a temp dir, never the
// checked-in testdata/geometry.yml directly -- UpdateField writes back to
// whatever path it was loaded from, and a test calling it against the shared
// fixture would silently rewrite a tracked file on every run.
func testGeometry(t *testing.T) *Geometry {
	t.Helper()

	src, err := os.ReadFile("testdata/geometry.yml")
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "geometry.yml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	g, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	return g
}

// calib builds a fully populated calibration: SSL_GeometryCameraCalibration
// has a dozen required fields, and an incomplete one is rejected by Absorb.
func calib(camID uint32, focalLength float32) *vision.SSL_GeometryCameraCalibration {
	return &vision.SSL_GeometryCameraCalibration{
		CameraId:        proto.Uint32(camID),
		FocalLength:     proto.Float32(focalLength),
		PrincipalPointX: proto.Float32(0),
		PrincipalPointY: proto.Float32(0),
		Distortion:      proto.Float32(0),
		Q0:              proto.Float32(0),
		Q1:              proto.Float32(0),
		Q2:              proto.Float32(0),
		Q3:              proto.Float32(1),
		Tx:              proto.Float32(0),
		Ty:              proto.Float32(0),
		Tz:              proto.Float32(5000),
	}
}

func TestAbsorbAppendsUnknownCamera(t *testing.T) {
	g := testGeometry(t)

	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400)}})

	got := g.Snapshot().GetGeometry().GetCalib()
	if len(got) != 1 || got[0].GetCameraId() != 0 || got[0].GetFocalLength() != 400 {
		t.Fatalf("calib = %v, want one entry for camera 0", got)
	}
}

func TestAbsorbReplacesChangedCamera(t *testing.T) {
	g := testGeometry(t)

	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400)}})
	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 500)}})

	got := g.Snapshot().GetGeometry().GetCalib()
	if len(got) != 1 || got[0].GetFocalLength() != 500 {
		t.Fatalf("calib = %v, want camera 0 updated to 500", got)
	}
}

func TestAbsorbLeavesUnchangedCameraAlone(t *testing.T) {
	g := testGeometry(t)

	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400)}})
	before := g.Encoded()

	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400)}})
	after := g.Encoded()

	if string(before) != string(after) {
		t.Fatalf("encoding changed on a no-op absorb")
	}
}

func TestAbsorbHandlesMultipleCameras(t *testing.T) {
	g := testGeometry(t)

	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400), calib(1, 410)}})

	got := g.Snapshot().GetGeometry().GetCalib()
	if len(got) != 2 {
		t.Fatalf("calib = %v, want 2 cameras", got)
	}
}

func TestAbsorbRejectsIncompleteCalibration(t *testing.T) {
	g := testGeometry(t)

	incomplete := &vision.SSL_GeometryCameraCalibration{CameraId: proto.Uint32(0)}
	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{incomplete}})

	if got := g.Snapshot().GetGeometry().GetCalib(); len(got) != 0 {
		t.Fatalf("calib = %v, want incomplete calibration dropped", got)
	}
}

func TestEncodedMatchesSnapshot(t *testing.T) {
	g := testGeometry(t)
	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400)}})

	var fromWire vision.SSL_WrapperPacket
	if err := proto.Unmarshal(g.Encoded(), &fromWire); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !proto.Equal(&fromWire, g.Snapshot()) {
		t.Fatalf("Encoded() does not round-trip to Snapshot()")
	}
}

// Exercises the concurrency contract under -race: Absorb, Encoded and Snapshot
// from many goroutines at once.
func TestConcurrentAbsorbAndRead(t *testing.T) {
	g := testGeometry(t)

	var wg sync.WaitGroup

	for camID := range uint32(8) {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := range 20 {
				g.Absorb(&vision.SSL_GeometryData{
					Calib: []*vision.SSL_GeometryCameraCalibration{calib(camID, float32(400+i))},
				})
			}
		}()
	}

	for range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 50 {
				_ = g.Encoded()
				_ = g.Snapshot()
			}
		}()
	}

	wg.Wait()

	if got := len(g.Snapshot().GetGeometry().GetCalib()); got != 8 {
		t.Fatalf("calib count = %d, want 8", got)
	}
}

// Real vision_processor instances wait for the field-geometry template
// before they can calibrate -- Run must hand it to them immediately on
// startup, not only after the first interval elapses.
func TestRunPublishesImmediatelyBeforeTheFirstInterval(t *testing.T) {
	g := testGeometry(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	published := make(chan []byte, 1)

	// An interval far longer than the test's own timeout: if a publish
	// arrives at all, it can only be the immediate one, not a ticked one.
	go func() { _ = g.run(ctx, time.Hour, func(b []byte) { published <- b }) }()

	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("Run did not publish immediately")
	}
}

// A Run started with an already-cancelled context does nothing at all --
// confirms the immediate publish above doesn't fire unconditionally.
func TestRunSkipsTheImmediatePublishIfAlreadyCancelled(t *testing.T) {
	g := testGeometry(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	published := make(chan []byte, 1)

	err := g.run(ctx, time.Hour, func(b []byte) { published <- b })
	if err != context.Canceled { //nolint:errorlint // exact sentinel, not a wrapped error
		t.Fatalf("run returned %v, want context.Canceled", err)
	}

	select {
	case <-published:
		t.Fatal("run published despite an already-cancelled context")
	default:
	}
}

func TestRunPublishesUntilCancelled(t *testing.T) {
	g := testGeometry(t)

	ctx, cancel := context.WithCancel(context.Background())
	published := make(chan []byte, 8)

	done := make(chan error, 1)
	go func() { done <- g.run(ctx, time.Millisecond, func(b []byte) { published <- b }) }()

	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("Run did not publish before timeout")
	}

	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after cancel")
	}
}
