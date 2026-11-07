package geometry

import (
	"math"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// realCalib is one the UC70 test camera produced.
func realCalib() *vision.SSL_GeometryCameraCalibration {
	return &vision.SSL_GeometryCameraCalibration{
		CameraId: proto.Uint32(0), FocalLength: proto.Float32(1145.78992),
		PrincipalPointX: proto.Float32(1920), PrincipalPointY: proto.Float32(1080), Distortion: proto.Float32(0),
		Q0: proto.Float32(-0.970117509), Q1: proto.Float32(0.231040925), Q2: proto.Float32(-0.060949523), Q3: proto.Float32(0.0421585),
		Tx: proto.Float32(3966.91333), Ty: proto.Float32(-1122.7002), Tz: proto.Float32(5118.81201),
		PixelImageWidth: proto.Uint32(3840), PixelImageHeight: proto.Uint32(2160),
	}
}

func TestCheckCalibration(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*vision.SSL_GeometryCameraCalibration)
		want string // "" for accepted
	}{
		"real camera":           {func(*vision.SSL_GeometryCameraCalibration) {}, ""},
		"derived height agrees": {func(c *vision.SSL_GeometryCameraCalibration) { c.DerivedCameraWorldTz = proto.Float32(4576.4) }, ""},
		"NaN": {func(c *vision.SSL_GeometryCameraCalibration) {
			c.Tx = proto.Float32(float32(math.NaN()))
		}, "tx is NaN"},
		"infinite": {func(c *vision.SSL_GeometryCameraCalibration) {
			c.FocalLength = proto.Float32(float32(math.Inf(1)))
		}, "focal_length is +Inf"},
		"no focal length": {func(c *vision.SSL_GeometryCameraCalibration) { c.FocalLength = proto.Float32(0) }, "focal_length"},
		"not a rotation": {func(c *vision.SSL_GeometryCameraCalibration) {
			c.Q0, c.Q1, c.Q2, c.Q3 = proto.Float32(0), proto.Float32(0), proto.Float32(0), proto.Float32(0)
		}, "quaternion"},
		"empty image": {func(c *vision.SSL_GeometryCameraCalibration) { c.PixelImageWidth = proto.Uint32(0) }, "image size"},
		// The GUI test fixture's: identity rotation, so the camera sits
		// 4 m under the field.
		"under the field": {func(c *vision.SSL_GeometryCameraCalibration) {
			c.Q0, c.Q1, c.Q2, c.Q3 = proto.Float32(0), proto.Float32(0), proto.Float32(0), proto.Float32(1)
			c.Tx, c.Ty, c.Tz = proto.Float32(0), proto.Float32(0), proto.Float32(4000)
		}, "-4000 mm"},
		"derived height under the field": {func(c *vision.SSL_GeometryCameraCalibration) {
			c.DerivedCameraWorldTz = proto.Float32(-4000)
		}, "-4000 mm"},
	} {
		t.Run(name, func(t *testing.T) {
			c := realCalib()
			tc.edit(c)

			err := CheckCalibration(c)
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("rejected: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("err = %v, want one mentioning %q", err, tc.want)
			}
		})
	}
}
