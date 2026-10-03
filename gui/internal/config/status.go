package config

import (
	"fmt"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
)

// Calibration states reported per camera.
const (
	CalibrationNone   = "none"   // nothing received, nothing locked
	CalibrationLive   = "live"   // received from the network, not locked
	CalibrationLocked = "locked" // locked in the document
)

// CameraStatus is everything about a camera derived from the document plus
// the live network state -- never stored.
type CameraStatus struct {
	CameraID    int    `json:"cameraId"`
	Calibration string `json:"calibration"`
	// Live is the latest calibration received from the network (proto field
	// names), whether or not one is locked -- what "Lock" would store.
	Live map[string]any `json:"live,omitempty"`
	// LiveResolution is the image size the live calibration was solved at,
	// [0, 0] if unknown.
	LiveResolution [2]int    `json:"liveResolution"`
	Warnings       []Warning `json:"warnings"`
}

// Warning is one problem worth surfacing on a camera, with a stable code the
// frontend can key UI (e.g. a "rescale corners" button) off.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func cameraStatus(doc Document, c Camera, live *vision.SSL_GeometryCameraCalibration) CameraStatus {
	status := CameraStatus{CameraID: c.CameraID, Calibration: CalibrationNone, Warnings: []Warning{}}

	warn := func(code, format string, args ...any) {
		status.Warnings = append(status.Warnings, Warning{Code: code, Message: fmt.Sprintf(format, args...)})
	}

	if live != nil {
		status.Calibration = CalibrationLive
		status.Live, _ = protoMap(live)
		status.LiveResolution = [2]int{int(live.GetPixelImageWidth()), int(live.GetPixelImageHeight())}
	}

	liveRes := status.LiveResolution
	liveKnown := liveRes[0] > 0 && liveRes[1] > 0

	if c.Calibration != nil {
		status.Calibration = CalibrationLocked

		if c.Calibration.FieldHash != fieldHash(doc.Field) {
			warn("field_changed", "The field dimensions changed after this calibration was locked; it was solved against the old field lines.")
		}

		if locked, err := c.Calibration.Proto(); err == nil && liveKnown {
			lockedRes := [2]int{int(locked.GetPixelImageWidth()), int(locked.GetPixelImageHeight())}
			if lockedRes[0] > 0 && lockedRes[1] > 0 && !sameAspect(lockedRes, liveRes) {
				warn("calibration_aspect_mismatch", "Locked calibration was solved at %dx%d but the camera now reports %dx%d, a different aspect ratio. vision_processor can't rescale across aspect ratios; unlock and recalibrate.", lockedRes[0], lockedRes[1], liveRes[0], liveRes[1])
			}
		}
	}

	if c.Seed != nil && len(c.Seed.LineCorners) == 4 {
		seedRes := c.Seed.Resolution

		switch {
		case seedRes[0] == 0 || seedRes[1] == 0:
			warn("seed_resolution_unknown", "Line corners were saved without the image resolution they were picked at, so a resolution change can't be detected. Re-save them from the corner picker.")
		case liveKnown && seedRes != liveRes && sameAspect(seedRes, liveRes):
			warn("seed_rescalable", "Line corners were picked at %dx%d but the camera now reports %dx%d. Same aspect ratio, so they can be rescaled.", seedRes[0], seedRes[1], liveRes[0], liveRes[1])
		case liveKnown && seedRes != liveRes:
			warn("seed_aspect_mismatch", "Line corners were picked at %dx%d but the camera now reports %dx%d, a different aspect ratio. Re-pick them.", seedRes[0], seedRes[1], liveRes[0], liveRes[1])
		}
	}

	return status
}

func sameAspect(a, b [2]int) bool {
	return a[0]*b[1] == b[0]*a[1]
}
