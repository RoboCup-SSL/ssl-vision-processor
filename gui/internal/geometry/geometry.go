package geometry

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// PublishInterval is how often the merged wrapper packet is republished.
const PublishInterval = time.Second

// Geometry owns the field configuration and per-camera calibrations absorbed
// from vision processors on the network.
//
// proto.Marshal writes to the message's size cache, so this is not a
// read-without-locking type. mu guards every access to wrapper. encoded is the
// last marshalled snapshot, kept so Run never has to marshal on its own.
type Geometry struct {
	mu       sync.Mutex
	wrapper  *vision.SSL_WrapperPacket
	encoded  []byte
	path     string         // source file, for UpdateField's save-back
	optional optionalLines  // retained so it can be read back and re-saved
	extra    map[string]any // top-level YAML keys besides optional_field_lines/field (e.g. models), preserved verbatim on save
}

// New loads geometry.yml at path.
func New(path string) (*Geometry, error) {
	wrapper, optional, extra, err := Load(path)
	if err != nil {
		return nil, err
	}

	g := &Geometry{wrapper: wrapper, path: path, optional: optional, extra: extra}
	if err := g.reencode(); err != nil {
		return nil, err
	}

	slog.Info("loaded geometry", "path", path, "calibrations", len(wrapper.GetGeometry().GetCalib()))

	return g, nil
}

// reencode refreshes encoded. Callers must hold mu.
func (g *Geometry) reencode() error {
	data, err := proto.Marshal(g.wrapper)
	if err != nil {
		return err
	}

	g.encoded = data

	return nil
}

// Encoded returns the last marshalled SSL_WrapperPacket.
func (g *Geometry) Encoded() []byte {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.encoded
}

// Snapshot returns a deep copy of the current wrapper packet.
func (g *Geometry) Snapshot() *vision.SSL_WrapperPacket {
	g.mu.Lock()
	defer g.mu.Unlock()

	return proto.Clone(g.wrapper).(*vision.SSL_WrapperPacket)
}

// Absorb merges one incoming SSL_GeometryData into the held wrapper packet.
// Unknown camera IDs are appended, unchanged ones are left alone, changed ones
// are replaced. A calibration missing required fields is rejected before it
// can touch stored state, so a bad message never desyncs wrapper from encoded.
func (g *Geometry) Absorb(incoming *vision.SSL_GeometryData) {
	g.mu.Lock()
	defer g.mu.Unlock()

	changed := false

	for _, camera := range incoming.GetCalib() {
		if err := proto.CheckInitialized(camera); err != nil {
			slog.Warn("dropping incomplete calibration", "cam", camera.GetCameraId(), "err", err)
			continue
		}

		if g.mergeCalib(camera) {
			changed = true
		}
	}

	if !changed {
		return
	}

	if err := g.reencode(); err != nil {
		slog.Error("re-encoding wrapper packet", "err", err)
	}
}

// mergeCalib applies one camera's calibration, reporting whether it changed
// anything. Callers must hold mu.
func (g *Geometry) mergeCalib(camera *vision.SSL_GeometryCameraCalibration) bool {
	calib := g.wrapper.GetGeometry().GetCalib()

	for i, existing := range calib {
		if existing.GetCameraId() != camera.GetCameraId() {
			continue
		}

		if proto.Equal(existing, camera) {
			return false
		}

		calib[i] = proto.Clone(camera).(*vision.SSL_GeometryCameraCalibration)
		slog.Info("updated camera calibration", "cam", camera.GetCameraId())

		return true
	}

	g.wrapper.Geometry.Calib = append(calib, proto.Clone(camera).(*vision.SSL_GeometryCameraCalibration))
	slog.Info("added camera calibration", "cam", camera.GetCameraId())

	return true
}

// Run publishes immediately, then republishes via publish once per
// PublishInterval until ctx is cancelled. Publishing before the first wait
// (not after) matters here: real vision_processor instances on the network
// wait to receive the field-geometry template before they can calibrate, so
// a restart of this host should hand it to them right away, not after an
// extra idle PublishInterval.
func (g *Geometry) Run(ctx context.Context, publish func([]byte)) error {
	return g.run(ctx, PublishInterval, publish)
}

func (g *Geometry) run(ctx context.Context, interval time.Duration, publish func([]byte)) error {
	// Skipped if ctx is already cancelled -- Run should do nothing and return
	// immediately in that case, same as before this published up front.
	if ctx.Err() == nil {
		publish(g.Encoded())
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			publish(g.Encoded())
		}
	}
}
