package geometry

import (
	"errors"
	"fmt"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// FieldConfig is the editable subset of SSL_GeometryFieldSize -- everything
// except FieldLines/FieldArcs (generated, never hand-edited) and the fields
// that live elsewhere (Calib is runtime state, not static config).
type FieldConfig struct {
	FieldLength               int32   `yaml:"field_length" json:"fieldLength"`
	FieldWidth                int32   `yaml:"field_width" json:"fieldWidth"`
	GoalWidth                 int32   `yaml:"goal_width" json:"goalWidth"`
	GoalDepth                 int32   `yaml:"goal_depth" json:"goalDepth"`
	GoalHeight                int32   `yaml:"goal_height" json:"goalHeight"`
	PenaltyAreaDepth          int32   `yaml:"penalty_area_depth" json:"penaltyAreaDepth"`
	PenaltyAreaWidth          int32   `yaml:"penalty_area_width" json:"penaltyAreaWidth"`
	GoalCenterToPenaltyMark   int32   `yaml:"goal_center_to_penalty_mark" json:"goalCenterToPenaltyMark"`
	BoundaryWidth             int32   `yaml:"boundary_width" json:"boundaryWidth"`
	BoundaryWidthGoalLine     int32   `yaml:"boundary_width_goal_line" json:"boundaryWidthGoalLine"`
	CenterCircleRadius        int32   `yaml:"center_circle_radius" json:"centerCircleRadius"`
	LineThickness             int32   `yaml:"line_thickness" json:"lineThickness"`
	BallRadius                float32 `yaml:"ball_radius" json:"ballRadius"`
	MaxRobotRadius            float32 `yaml:"max_robot_radius" json:"maxRobotRadius"`
	GoalSubstitutionAreaWidth int32   `yaml:"goal_substitution_area_width" json:"goalSubstitutionAreaWidth"`
}

// OptionalLinesConfig is optionalLines with concrete bools instead of
// pointers -- an API request/response always supplies all four, so there's no
// "missing" case to represent here the way there is when parsing a hand-edited
// YAML file.
type OptionalLinesConfig struct {
	Goal2Goal    bool `yaml:"goal2goal" json:"goal2Goal"`
	Halfway      bool `yaml:"halfway" json:"halfway"`
	CenterCircle bool `yaml:"centercircle" json:"centerCircle"`
	Penalty      bool `yaml:"penalty" json:"penalty"`
}

// ValidationError marks a FieldConfig rejected by Validate -- the caller's
// mistake, not ours. Callers such as the HTTP handler use errors.As to tell
// this apart from an internal failure (e.g. a disk write error) and respond
// 400 instead of 500.
type ValidationError struct{ err error }

func (v *ValidationError) Error() string { return v.err.Error() }
func (v *ValidationError) Unwrap() error { return v.err }

// Validate reports every dimension that can't be right, so a caller can
// surface all of them at once rather than one failed PUT per typo.
func (c FieldConfig) Validate() error {
	var errs error

	check := func(name string, value int32) {
		if value <= 0 {
			errs = errors.Join(errs, fmt.Errorf("%s: must be greater than 0, got %d", name, value))
		}
	}

	check("field_length", c.FieldLength)
	check("field_width", c.FieldWidth)
	check("goal_width", c.GoalWidth)
	check("goal_depth", c.GoalDepth)
	check("boundary_width", c.BoundaryWidth)

	if errs != nil {
		return &ValidationError{errs}
	}

	return nil
}

// fieldConfigFrom reads a FieldConfig/OptionalLinesConfig pair out of a
// decoded field size and its optional-line toggles. Shared by FieldConfig
// (the live, held Geometry) and LoadPreset (an arbitrary file read fresh,
// with no Geometry involved).
func fieldConfigFrom(field *vision.SSL_GeometryFieldSize, optional optionalLines) (FieldConfig, OptionalLinesConfig) {
	return FieldConfig{
			FieldLength:               field.GetFieldLength(),
			FieldWidth:                field.GetFieldWidth(),
			GoalWidth:                 field.GetGoalWidth(),
			GoalDepth:                 field.GetGoalDepth(),
			GoalHeight:                field.GetGoalHeight(),
			PenaltyAreaDepth:          field.GetPenaltyAreaDepth(),
			PenaltyAreaWidth:          field.GetPenaltyAreaWidth(),
			GoalCenterToPenaltyMark:   field.GetGoalCenterToPenaltyMark(),
			BoundaryWidth:             field.GetBoundaryWidth(),
			BoundaryWidthGoalLine:     field.GetBoundaryWidthGoalLine(),
			CenterCircleRadius:        field.GetCenterCircleRadius(),
			LineThickness:             field.GetLineThickness(),
			BallRadius:                field.GetBallRadius(),
			MaxRobotRadius:            field.GetMaxRobotRadius(),
			GoalSubstitutionAreaWidth: field.GetGoalSubstitutionAreaWidth(),
		}, OptionalLinesConfig{
			Goal2Goal:    optional.enabled(optional.Goal2Goal),
			Halfway:      optional.enabled(optional.Halfway),
			CenterCircle: optional.enabled(optional.CenterCircle),
			Penalty:      optional.enabled(optional.Penalty),
		}
}

// LoadFieldFile reads a legacy geometry YAML file (geometry-*.yml) fresh --
// no Geometry instance, no mutation of anything -- and returns its field
// dimensions, optional-line toggles, and ball models. Used for the rulebook
// presets and for importing a pre-vision.yml setup.
func LoadFieldFile(path string) (FieldConfig, OptionalLinesConfig, *vision.SSL_GeometryModels, error) {
	wrapper, optional, _, err := Load(path)
	if err != nil {
		return FieldConfig{}, OptionalLinesConfig{}, nil, err
	}

	field, opt := fieldConfigFrom(wrapper.GetGeometry().GetField(), optional)

	return field, opt, wrapper.GetGeometry().GetModels(), nil
}

// LoadPreset is LoadFieldFile without the models, for serving the rulebook
// presets (config/legacy/geometry-div{A,B}.yml) straight from the same files
// a human would open, rather than a copy that could drift from them.
func LoadPreset(path string) (FieldConfig, OptionalLinesConfig, error) {
	field, opt, _, err := LoadFieldFile(path)

	return field, opt, err
}

// applyFieldConfig builds a field message from cfg, regenerates its derived
// markings, and installs it onto g. Callers must hold mu and must have already
// validated cfg -- this never fails.
func (g *Geometry) applyFieldConfig(cfg FieldConfig, opt OptionalLinesConfig) {
	field := &vision.SSL_GeometryFieldSize{
		FieldLength:               proto.Int32(cfg.FieldLength),
		FieldWidth:                proto.Int32(cfg.FieldWidth),
		GoalWidth:                 proto.Int32(cfg.GoalWidth),
		GoalDepth:                 proto.Int32(cfg.GoalDepth),
		GoalHeight:                proto.Int32(cfg.GoalHeight),
		PenaltyAreaDepth:          proto.Int32(cfg.PenaltyAreaDepth),
		PenaltyAreaWidth:          proto.Int32(cfg.PenaltyAreaWidth),
		GoalCenterToPenaltyMark:   proto.Int32(cfg.GoalCenterToPenaltyMark),
		BoundaryWidth:             proto.Int32(cfg.BoundaryWidth),
		BoundaryWidthGoalLine:     proto.Int32(cfg.BoundaryWidthGoalLine),
		CenterCircleRadius:        proto.Int32(cfg.CenterCircleRadius),
		LineThickness:             proto.Int32(cfg.LineThickness),
		BallRadius:                proto.Float32(cfg.BallRadius),
		MaxRobotRadius:            proto.Float32(cfg.MaxRobotRadius),
		GoalSubstitutionAreaWidth: proto.Int32(cfg.GoalSubstitutionAreaWidth),
	}

	optional := optionalLines{
		Goal2Goal:    &opt.Goal2Goal,
		Halfway:      &opt.Halfway,
		CenterCircle: &opt.CenterCircle,
		Penalty:      &opt.Penalty,
	}

	generateFieldMarkings(field, optional)

	g.wrapper.Geometry.Field = field
}
