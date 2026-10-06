package geometry

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// PublishInterval is how often the merged wrapper packet is republished.
const PublishInterval = time.Second

// Geometry owns the live field template and the per-camera calibrations
// published with it. It holds no file state: internal/config owns vision.yml
// and pushes changes in through ApplyConfig/SetLocked.
//
// Each camera's published calibration is its locked one if set, otherwise
// the latest one absorbed from the network. A vision processor only skips
// calibrating at startup if the packet it receives holds a calibration for
// its camera_id, so publishing locked calibrations is what lets a restart of
// this host (or of a processor) avoid a needless recalibration.
//
// proto.Marshal writes to the message's size cache, so this is not a
// read-without-locking type. mu guards every field. encoded is the last
// marshalled snapshot, kept so Run never has to marshal on its own.
type Geometry struct {
	mu       sync.Mutex
	wrapper  *vision.SSL_WrapperPacket
	encoded  []byte
	absorbed map[uint32]*vision.SSL_GeometryCameraCalibration
	locked   map[uint32]*vision.SSL_GeometryCameraCalibration
}

// New builds a Geometry publishing the given field template and no
// calibrations.
func New(field FieldConfig, optional OptionalLinesConfig, models *vision.SSL_GeometryModels) (*Geometry, error) {
	if err := field.Validate(); err != nil {
		return nil, err
	}

	g := &Geometry{
		wrapper: &vision.SSL_WrapperPacket{
			Geometry: &vision.SSL_GeometryData{},
			Source:   vision.SSL_Source_SSL_SOURCE_VISION_PROCESSOR.Enum(),
		},
		absorbed: map[uint32]*vision.SSL_GeometryCameraCalibration{},
		locked:   map[uint32]*vision.SSL_GeometryCameraCalibration{},
	}

	g.applyFieldConfig(field, optional)
	g.wrapper.Geometry.Models = cloneModels(models)

	if err := g.reencode(); err != nil {
		return nil, err
	}

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

// ApplyConfig replaces the field template (dimensions, optional markings,
// ball models) and regenerates the derived field lines/arcs. Calibrations are
// untouched. On failure nothing changes.
func (g *Geometry) ApplyConfig(field FieldConfig, optional OptionalLinesConfig, models *vision.SSL_GeometryModels) error {
	if err := field.Validate(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	previousField := g.wrapper.Geometry.Field
	previousModels := g.wrapper.Geometry.Models

	g.applyFieldConfig(field, optional)
	g.wrapper.Geometry.Models = cloneModels(models)

	if err := g.reencode(); err != nil {
		g.wrapper.Geometry.Field = previousField
		g.wrapper.Geometry.Models = previousModels

		return fmt.Errorf("encode updated geometry: %w", err)
	}

	return nil
}

// SetLocked replaces the full set of locked calibrations, keyed by camera_id.
// Each must already be complete (proto.CheckInitialized).
func (g *Geometry) SetLocked(locked map[uint32]*vision.SSL_GeometryCameraCalibration) error {
	next := make(map[uint32]*vision.SSL_GeometryCameraCalibration, len(locked))

	for id, c := range locked {
		if err := proto.CheckInitialized(c); err != nil {
			return fmt.Errorf("locked calibration for camera %d: %w", id, err)
		}

		next[id] = proto.Clone(c).(*vision.SSL_GeometryCameraCalibration)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.locked = next

	return g.republishCalib()
}

// Unlock stops publishing any calibration for camID -- locked or absorbed --
// so the next time that camera's vision processor starts, it finds none for
// itself and calibrates. A processor that's already running keeps the model it
// has; the protocol has no way to tell it to recalibrate live.
func (g *Geometry) Unlock(camID uint32) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.locked, camID)
	delete(g.absorbed, camID)

	return g.republishCalib()
}

// LiveCalibration returns a copy of the latest calibration absorbed from the
// network for camID, or nil if none has arrived since startup (or since the
// last Unlock).
func (g *Geometry) LiveCalibration(camID uint32) *vision.SSL_GeometryCameraCalibration {
	g.mu.Lock()
	defer g.mu.Unlock()

	c, ok := g.absorbed[camID]
	if !ok {
		return nil
	}

	return proto.Clone(c).(*vision.SSL_GeometryCameraCalibration)
}

// Absorb records the calibrations in one incoming SSL_GeometryData. A
// calibration missing required fields is rejected before it can touch stored
// state, so a bad message never desyncs wrapper from encoded. Absorbed
// calibrations for a locked camera are kept (for LiveCalibration) but not
// published -- the locked one wins.
func (g *Geometry) Absorb(incoming *vision.SSL_GeometryData) {
	g.mu.Lock()
	defer g.mu.Unlock()

	changed := false

	for _, camera := range incoming.GetCalib() {
		if err := proto.CheckInitialized(camera); err != nil {
			slog.Warn("dropping incomplete calibration", "cam", camera.GetCameraId(), "err", err)
			continue
		}

		id := camera.GetCameraId()
		if existing, ok := g.absorbed[id]; ok && proto.Equal(existing, camera) {
			continue
		}

		g.absorbed[id] = proto.Clone(camera).(*vision.SSL_GeometryCameraCalibration)
		slog.Info("absorbed camera calibration", "cam", id)

		changed = true
	}

	if !changed {
		return
	}

	if err := g.republishCalib(); err != nil {
		slog.Error("re-encoding wrapper packet", "err", err)
	}
}

// republishCalib rebuilds the published calibration list from locked and
// absorbed, ordered by camera_id, and re-encodes. Callers must hold mu.
func (g *Geometry) republishCalib() error {
	ids := make([]uint32, 0, len(g.locked)+len(g.absorbed))
	for id := range g.locked {
		ids = append(ids, id)
	}

	for id := range g.absorbed {
		if _, ok := g.locked[id]; !ok {
			ids = append(ids, id)
		}
	}

	slices.Sort(ids)

	calib := make([]*vision.SSL_GeometryCameraCalibration, 0, len(ids))
	for _, id := range ids {
		if c, ok := g.locked[id]; ok {
			calib = append(calib, c)
		} else {
			calib = append(calib, g.absorbed[id])
		}
	}

	g.wrapper.Geometry.Calib = calib

	return g.reencode()
}

func cloneModels(models *vision.SSL_GeometryModels) *vision.SSL_GeometryModels {
	if models == nil {
		return nil
	}

	return proto.Clone(models).(*vision.SSL_GeometryModels)
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
