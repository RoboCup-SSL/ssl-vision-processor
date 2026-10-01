package geometry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// ReadOnlyFiles are geometry files UpdateField refuses to overwrite: the
// rulebook presets (see gui/frontend/src/lib/fieldPresets.ts, or rather --
// once presets are read from these directly -- the sole source of truth for
// them). Matched by base filename, wherever -geometryFile happens to point.
//
// This exists because it's an easy mistake to make once, not a hypothetical:
// the default -geometryFile briefly was "geometry-divB.yml" itself, so the
// very first Save in the Virtual Field editor would have silently overwritten
// the tracked competition preset.
var ReadOnlyFiles = map[string]bool{
	"geometry-divA.yml": true,
	"geometry-divB.yml": true,
}

// ReadOnlyError marks a save rejected because the target file is a protected
// preset, not the caller's working config.
type ReadOnlyError struct{ path string }

func (e *ReadOnlyError) Error() string {
	return fmt.Sprintf("%s is a read-only rulebook preset; point -geometryFile at a working copy instead", e.path)
}

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
	Goal2Goal    bool `json:"goal2Goal"`
	Halfway      bool `json:"halfway"`
	CenterCircle bool `json:"centerCircle"`
	Penalty      bool `json:"penalty"`
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

// FieldConfig returns the current editable field dimensions and optional-line
// toggles.
func (g *Geometry) FieldConfig() (FieldConfig, OptionalLinesConfig) {
	g.mu.Lock()
	defer g.mu.Unlock()

	return fieldConfigFrom(g.wrapper.GetGeometry().GetField(), g.optional)
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

// LoadPreset reads a geometry YAML file fresh -- no Geometry instance, no
// mutation of anything -- and returns its field dimensions and optional-line
// toggles. Used to serve the rulebook presets (geometry-divA.yml,
// geometry-divB.yml) straight from the same files a human would open, rather
// than a copy that could drift from them.
func LoadPreset(path string) (FieldConfig, OptionalLinesConfig, error) {
	wrapper, optional, _, err := Load(path)
	if err != nil {
		return FieldConfig{}, OptionalLinesConfig{}, err
	}

	field, opt := fieldConfigFrom(wrapper.GetGeometry().GetField(), optional)

	return field, opt, nil
}

// applyFieldConfig builds a field message from cfg, regenerates its derived
// markings, and installs both it and opt onto g. Callers must hold mu and
// must have already validated cfg and checked ReadOnlyFiles for whichever
// path they're about to write to -- this never fails, so it's only safe to
// call once nothing left can reject the operation.
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
	g.optional = optional
}

// applyAndPersist mutates g to reflect cfg/opt, re-encodes, and calls persist
// to write the change to disk -- rolling the in-memory mutation back if
// either step fails, so a rejected save never leaves g holding (and
// broadcasting, over multicast and the WS hub) field data that was never
// actually written anywhere. Callers must hold mu, must have already
// validated cfg, and must have already checked ReadOnlyFiles for whichever
// path persist is about to write to.
func (g *Geometry) applyAndPersist(cfg FieldConfig, opt OptionalLinesConfig, persist func() error) error {
	previousField := g.wrapper.Geometry.Field
	previousOptional := g.optional
	previousEncoded := g.encoded

	rollback := func() {
		g.wrapper.Geometry.Field = previousField
		g.optional = previousOptional
		g.encoded = previousEncoded
	}

	g.applyFieldConfig(cfg, opt)

	if err := g.reencode(); err != nil {
		rollback()

		return fmt.Errorf("encode updated geometry: %w", err)
	}

	if err := persist(); err != nil {
		rollback()

		return fmt.Errorf("save %s: %w", g.path, err)
	}

	return nil
}

// UpdateField replaces the field dimensions and optional-line toggles,
// regenerates the derived field lines/arcs, and persists the change to the
// file Geometry was loaded from. Existing calibrations are kept.
//
// Persisting rewrites the whole file, so hand-added comments in it do not
// survive a save made through this path.
func (g *Geometry) UpdateField(cfg FieldConfig, opt OptionalLinesConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Checked before anything below mutates g.wrapper/g.optional: a rejected
	// save must not leave the in-memory (and multicast-broadcast) state
	// holding values that were never actually persisted.
	if ReadOnlyFiles[filepath.Base(g.path)] {
		return &ReadOnlyError{path: g.path}
	}

	return g.applyAndPersist(cfg, opt, func() error { return g.saveYAML(cfg, opt) })
}

// SaveAs is UpdateField plus switching which file Geometry edits: it writes
// cfg/opt to path rather than the current file, and -- once that write
// succeeds -- every subsequent Save (UpdateField) goes to path too. Whatever
// non-field content the current file carried (e.g. "models") comes along
// unchanged; existing calibrations are kept, same as UpdateField.
func (g *Geometry) SaveAs(path string, cfg FieldConfig, opt OptionalLinesConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if ReadOnlyFiles[filepath.Base(path)] {
		return &ReadOnlyError{path: path}
	}

	// g.path changes before persisting so applyAndPersist's error message (and
	// saveYAML's ReadOnlyFiles recheck) refer to the new path -- rolled back
	// on any failure, same as the field/optional/encoded state, so a rejected
	// SaveAs never leaves Geometry claiming to be a file nothing was actually
	// written to.
	previousPath := g.path
	g.path = path

	if err := g.applyAndPersist(cfg, opt, func() error { return g.saveYAML(cfg, opt) }); err != nil {
		g.path = previousPath

		return err
	}

	return nil
}

// LoadFrom replaces this Geometry's entire state -- field config, optional
// lines, and whatever else the file carries (e.g. "models") -- with what's in
// path, and starts editing that file: subsequent Save calls go there.
//
// Existing calibrations are discarded, not carried over: they were computed
// against the field this Geometry used to hold, and are meaningless against
// whatever was just loaded (which may not even be the same size).
func (g *Geometry) LoadFrom(path string) error {
	wrapper, optional, extra, err := Load(path)
	if err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.wrapper = wrapper
	g.path = path
	g.optional = optional
	g.extra = extra

	return g.reencode()
}

// Path reports the file Geometry currently reads from and saves to.
func (g *Geometry) Path() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.path
}

// saveYAML writes the current field config and optional-line toggles back to
// g.path, merged with whatever other top-level keys (e.g. "models") were
// present when it was loaded. Callers must hold mu.
func (g *Geometry) saveYAML(cfg FieldConfig, opt OptionalLinesConfig) error {
	if ReadOnlyFiles[filepath.Base(g.path)] {
		return &ReadOnlyError{path: g.path}
	}

	out := map[string]any{
		"optional_field_lines": map[string]bool{
			"goal2goal":    opt.Goal2Goal,
			"halfway":      opt.Halfway,
			"centercircle": opt.CenterCircle,
			"penalty":      opt.Penalty,
		},
		"field": cfg,
	}

	for k, v := range g.extra {
		out[k] = v
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return err
	}

	return os.WriteFile(g.path, data, 0o644)
}
