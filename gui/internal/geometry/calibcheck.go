package geometry

import (
	"errors"
	"fmt"
	"math"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
)

// CheckCalibration reports why calib can't be a real camera's calibration,
// or nil. Publishing one that isn't makes every vision_processor for that
// camera skip calibrating and use it, so a NaN, a test fixture or another
// host's mistake would stop that camera for good.
func CheckCalibration(calib *vision.SSL_GeometryCameraCalibration) error {
	values := map[string]float32{
		"focal_length": calib.GetFocalLength(), "principal_point_x": calib.GetPrincipalPointX(),
		"principal_point_y": calib.GetPrincipalPointY(), "distortion": calib.GetDistortion(),
		"q0": calib.GetQ0(), "q1": calib.GetQ1(), "q2": calib.GetQ2(), "q3": calib.GetQ3(),
		"tx": calib.GetTx(), "ty": calib.GetTy(), "tz": calib.GetTz(),
		"derived_camera_world_tx": calib.GetDerivedCameraWorldTx(),
		"derived_camera_world_ty": calib.GetDerivedCameraWorldTy(),
		"derived_camera_world_tz": calib.GetDerivedCameraWorldTz(),
	}

	var errs error

	for name, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			errs = errors.Join(errs, fmt.Errorf("%s is %v", name, v))
		}
	}

	if errs != nil {
		return errs
	}

	if calib.GetFocalLength() <= 0 {
		return fmt.Errorf("focal_length %v isn't positive", calib.GetFocalLength())
	}

	q := []float64{float64(calib.GetQ0()), float64(calib.GetQ1()), float64(calib.GetQ2()), float64(calib.GetQ3())}
	if norm := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3]); math.Abs(norm-1) > 0.05 {
		return fmt.Errorf("rotation quaternion has length %.3f, not 1", norm)
	}

	width, height := calib.GetPixelImageWidth(), calib.GetPixelImageHeight()
	if calib.PixelImageWidth != nil && calib.PixelImageHeight != nil && (width == 0 || height == 0) {
		return fmt.Errorf("image size %dx%d is empty", width, height)
	}

	if z := cameraHeight(calib); z <= 0 {
		return fmt.Errorf("puts the camera %.0f mm above the field, not over it", z)
	}

	return nil
}

// cameraHeight is the camera's height above the field in mm: derived_camera_
// world_tz when sent, else worked out from the rotation and translation, as
// -R^T t (ssl-vision's convention).
func cameraHeight(calib *vision.SSL_GeometryCameraCalibration) float64 {
	if calib.DerivedCameraWorldTz != nil {
		return float64(calib.GetDerivedCameraWorldTz())
	}

	q0, q1, q2, q3 := float64(calib.GetQ0()), float64(calib.GetQ1()), float64(calib.GetQ2()), float64(calib.GetQ3())
	t := [3]float64{float64(calib.GetTx()), float64(calib.GetTy()), float64(calib.GetTz())}

	// The rotation's third column, for (q0, q1, q2) the vector part and q3
	// the scalar part.
	col := [3]float64{
		2 * (q0*q2 + q1*q3),
		2 * (q1*q2 - q0*q3),
		1 - 2*(q0*q0+q1*q1),
	}

	return -(col[0]*t[0] + col[1]*t[1] + col[2]*t[2])
}
