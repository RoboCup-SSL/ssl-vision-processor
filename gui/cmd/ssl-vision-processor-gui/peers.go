package main

import (
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/peers"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
)

// peerWatches is who the host hears sending what on its groups, so two
// senders claiming one thing are logged (IP, port, hostname) and alerted.
type peerWatches struct {
	names       *peers.Names
	calibration *peers.Watch
	detections  *peers.Watch
	geometry    *peers.Watch
	referee     *peers.Watch
	video       *peers.Watch
	rejected    rejectLog
}

func newPeerWatches() *peerWatches {
	names := peers.NewNames()

	return &peerWatches{
		names: names,
		// vision_processor sends its calibration once, after calibrating; a
		// host republishing one sends it every second. Five minutes catches
		// either next to the other.
		calibration: peers.NewWatch(peers.Options{Kind: peers.KindCalibration, Window: 5 * time.Minute}, names),
		detections:  peers.NewWatch(peers.Options{Kind: peers.KindDetections, Window: 3 * time.Second, MinCount: 2}, names),
		// vision_processor sends the field back now and then; only another
		// host publishes it every second, as this one does.
		geometry: peers.NewWatch(peers.Options{Kind: peers.KindGeometry, Window: 10 * time.Second, MinCount: 3, WithSelf: true}, names),
		referee:  peers.NewWatch(peers.Options{Kind: peers.KindReferee, Window: 3 * time.Second, MinCount: 2}, names),
		video:    peers.NewWatch(peers.Options{Kind: peers.KindVideo, Window: 3 * time.Second, MinCount: 10}, names),
	}
}

// Conflicts lists what's claimed by too many senders right now. Detections
// aren't included: the camera_id_conflict alert covers them.
func (p *peerWatches) Conflicts(now time.Time) []peers.Conflict {
	out := []peers.Conflict{}

	for _, w := range []*peers.Watch{p.calibration, p.geometry, p.referee, p.video} {
		out = append(out, w.Conflicts(now)...)
	}

	p.detections.Conflicts(now) // logs any that ended

	return out
}

// absorber passes on to geom the calibrations in each geometry packet that
// could be a real camera's, recording who sent what.
func (p *peerWatches) absorber(geom *geometry.Geometry) func(*vision.SSL_GeometryData, *net.UDPAddr) {
	return func(data *vision.SSL_GeometryData, from *net.UDPAddr) {
		now := time.Now()

		if data.GetField() != nil {
			p.geometry.Record("field geometry", nil, from, now)
		}

		accepted := &vision.SSL_GeometryData{}

		for _, calib := range data.GetCalib() {
			id := int(calib.GetCameraId())

			if err := geometry.CheckCalibration(calib); err != nil {
				p.rejected.log(p.names, id, from, err)

				continue
			}

			p.calibration.Record(fmt.Sprintf("camera %d calibration", id), &id, from, now)
			accepted.Calib = append(accepted.Calib, calib)
		}

		if len(accepted.Calib) > 0 {
			geom.Absorb(accepted)
		}
	}
}

// detection records who sent a camera's detection frame.
func (p *peerWatches) detection(cameraID uint32, from *net.UDPAddr, now time.Time) {
	id := int(cameraID)
	p.detections.Record(fmt.Sprintf("camera %d detections", id), &id, from, now)
}

// videoPacket records who sent a packet of a camera's stream.
func (p *peerWatches) videoPacket(cameraID int, from *net.UDPAddr) {
	p.video.Record(fmt.Sprintf("camera %d video", cameraID), &cameraID, from, time.Now())
}

// rejectRepeat is how often a sender that keeps sending the same rejected
// calibration is logged again.
const rejectRepeat = time.Minute

// rejectLog logs rejected calibrations: the first of each sender, camera and
// reason at once, repeats once a minute with how many there were.
type rejectLog struct {
	mu   sync.Mutex
	seen map[string]*rejected
}

type rejected struct {
	logged time.Time
	count  int
}

func (r *rejectLog) log(names *peers.Names, cameraID int, from *net.UDPAddr, reason error) {
	key := fmt.Sprintf("%v|%d|%v", from, cameraID, reason)
	now := time.Now()

	r.mu.Lock()

	if r.seen == nil {
		r.seen = map[string]*rejected{}
	}

	e := r.seen[key]
	if e == nil {
		e = &rejected{}
		r.seen[key] = e
	}

	e.count++

	if !e.logged.IsZero() && now.Sub(e.logged) < rejectRepeat {
		r.mu.Unlock()

		return
	}

	count := e.count
	e.logged, e.count = now, 0
	r.mu.Unlock()

	// Naming the sender can wait on DNS; never in the receive path.
	go func() {
		s := names.Describe(from)
		slog.Warn("rejected calibration: not a real camera's, so not published",
			"cam", cameraID, "reason", reason, "from", s.Address, "host", s.Host, "local", s.Local, "times", count)
	}()
}
