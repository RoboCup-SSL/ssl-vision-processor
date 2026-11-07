package geometry

import (
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

func testConfig() FieldConfig {
	return FieldConfig{
		FieldLength:   2160,
		FieldWidth:    1680,
		GoalWidth:     280,
		GoalDepth:     50,
		BoundaryWidth: 100,
		LineThickness: 10,
	}
}

func testOptional() OptionalLinesConfig {
	return OptionalLinesConfig{Halfway: true, Penalty: true}
}

func TestApplyConfigRegeneratesMarkings(t *testing.T) {
	g := testGeometry(t)

	if err := g.ApplyConfig(testConfig(), testOptional(), nil); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}

	field := g.Snapshot().GetGeometry().GetField()
	if field.GetFieldLength() != 2160 {
		t.Errorf("field_length = %d, want 2160", field.GetFieldLength())
	}

	// Halfway is on, goal2goal/centercircle are off: expect the 4 mandatory
	// lines + HalfwayLine + the 6 penalty stretches, nothing else.
	if lines := field.GetFieldLines(); len(lines) != 11 {
		t.Fatalf("field lines = %d, want 11 (got %v)", len(lines), lines)
	}
}

func TestApplyConfigRejectsNonsenseAndChangesNothing(t *testing.T) {
	g := testGeometry(t)
	before := g.Encoded()

	cfg := testConfig()
	cfg.FieldLength = 0

	if err := g.ApplyConfig(cfg, testOptional(), nil); err == nil {
		t.Fatal("ApplyConfig accepted a zero field_length")
	}

	if string(g.Encoded()) != string(before) {
		t.Fatal("a rejected ApplyConfig changed the published packet")
	}
}

func TestApplyConfigKeepsExistingCalibrations(t *testing.T) {
	g := testGeometry(t)
	g.Absorb(&vision.SSL_GeometryData{Calib: []*vision.SSL_GeometryCameraCalibration{calib(0, 400)}})

	if err := g.ApplyConfig(testConfig(), testOptional(), nil); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}

	if got := g.Snapshot().GetGeometry().GetCalib(); len(got) != 1 {
		t.Fatalf("calib = %v, want the pre-existing entry kept", got)
	}
}

func TestApplyConfigReplacesModels(t *testing.T) {
	g := testGeometry(t)

	models := &vision.SSL_GeometryModels{
		StraightTwoPhase: &vision.SSL_BallModelStraightTwoPhase{
			AccSlide: proto.Float64(-3),
			AccRoll:  proto.Float64(-0.3),
			KSwitch:  proto.Float64(0.6),
		},
	}

	if err := g.ApplyConfig(testConfig(), testOptional(), models); err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}

	if got := g.Snapshot().GetGeometry().GetModels(); !proto.Equal(got, models) {
		t.Fatalf("models = %v, want %v", got, models)
	}
}

func TestLoadPresetReadsWithoutAGeometryInstance(t *testing.T) {
	cfg, opt, err := LoadPreset("testdata/geometry.yml")
	if err != nil {
		t.Fatalf("LoadPreset: %v", err)
	}

	if cfg.FieldLength != 9000 {
		t.Errorf("field_length = %d, want 9000", cfg.FieldLength)
	}

	if !opt.Halfway {
		t.Error("halfway = false, want true (per the fixture)")
	}
}

func TestNewRejectsAnInvalidField(t *testing.T) {
	cfg := testConfig()
	cfg.FieldWidth = -1

	if _, err := New(cfg, testOptional(), nil); err == nil {
		t.Fatal("New accepted a negative field_width")
	}
}
