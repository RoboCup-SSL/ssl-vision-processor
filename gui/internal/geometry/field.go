package geometry

import (
	"math"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// generateFieldMarkings appends the standard SSL lines and arcs derived from the
// field dimensions. Markings the physical field lacks are skipped: publishing a
// line the camera cannot see disturbs refinement calibration.
//
// Coordinates are in mm, with the origin at the field centre, +x towards the
// right goal and +y towards the top touch line.
func generateFieldMarkings(field *vision.SSL_GeometryFieldSize, optional optionalLines) {
	var (
		thickness     = float32(field.GetLineThickness())
		halfLength    = float32(field.GetFieldLength()) / 2
		halfWidth     = float32(field.GetFieldWidth()) / 2
		penaltyLength = halfLength - float32(field.GetPenaltyAreaDepth())
		halfPenalty   = float32(field.GetPenaltyAreaWidth()) / 2
	)

	addLine := func(shape vision.SSL_FieldShapeType, x1, y1, x2, y2 float32) {
		field.FieldLines = append(field.FieldLines, &vision.SSL_FieldLineSegment{
			// The name is the enum value's own spelling, which is what the rest
			// of the league keys off.
			Name:      proto.String(shape.String()),
			P1:        &vision.Vector2F{X: proto.Float32(x1), Y: proto.Float32(y1)},
			P2:        &vision.Vector2F{X: proto.Float32(x2), Y: proto.Float32(y2)},
			Thickness: proto.Float32(thickness),
			Type:      shape.Enum(),
		})
	}

	// Touchlines and goal lines are always present: every field has them, and
	// they bound the area the detector considers in play.
	addLine(vision.SSL_FieldShapeType_TopTouchLine, -halfLength, halfWidth, halfLength, halfWidth)
	addLine(vision.SSL_FieldShapeType_BottomTouchLine, -halfLength, -halfWidth, halfLength, -halfWidth)
	addLine(vision.SSL_FieldShapeType_LeftGoalLine, -halfLength, -halfWidth, -halfLength, halfWidth)
	addLine(vision.SSL_FieldShapeType_RightGoalLine, halfLength, -halfWidth, halfLength, halfWidth)

	if optional.enabled(optional.Halfway) {
		addLine(vision.SSL_FieldShapeType_HalfwayLine, 0, -halfWidth, 0, halfWidth)
	}

	if optional.enabled(optional.Goal2Goal) {
		addLine(vision.SSL_FieldShapeType_CenterLine, -halfLength, 0, halfLength, 0)
	}

	if optional.enabled(optional.Penalty) {
		addLine(vision.SSL_FieldShapeType_LeftPenaltyStretch,
			-penaltyLength, -halfPenalty, -penaltyLength, halfPenalty)
		addLine(vision.SSL_FieldShapeType_RightPenaltyStretch,
			penaltyLength, -halfPenalty, penaltyLength, halfPenalty)
		addLine(vision.SSL_FieldShapeType_LeftFieldLeftPenaltyStretch,
			-halfLength, -halfPenalty, -penaltyLength, -halfPenalty)
		addLine(vision.SSL_FieldShapeType_LeftFieldRightPenaltyStretch,
			-halfLength, halfPenalty, -penaltyLength, halfPenalty)
		addLine(vision.SSL_FieldShapeType_RightFieldLeftPenaltyStretch,
			penaltyLength, halfPenalty, halfLength, halfPenalty)
		addLine(vision.SSL_FieldShapeType_RightFieldRightPenaltyStretch,
			penaltyLength, -halfPenalty, halfLength, -halfPenalty)
	}

	if optional.enabled(optional.CenterCircle) {
		shape := vision.SSL_FieldShapeType_CenterCircle

		field.FieldArcs = append(field.FieldArcs, &vision.SSL_FieldCircularArc{
			Name:      proto.String(shape.String()),
			Center:    &vision.Vector2F{X: proto.Float32(0), Y: proto.Float32(0)},
			Radius:    proto.Float32(float32(field.GetCenterCircleRadius())),
			A1:        proto.Float32(0),
			A2:        proto.Float32(math.Pi * 2),
			Thickness: proto.Float32(thickness),
			Type:      shape.Enum(),
		})
	}
}

// enabled reads one of the optionalLines pointers. A nil pointer cannot occur
// after validate, but treating it as off keeps a programming error from
// publishing a marking the field does not have.
func (o optionalLines) enabled(flag *bool) bool {
	return flag != nil && *flag
}
