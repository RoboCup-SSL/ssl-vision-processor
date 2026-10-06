package config

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

// advancedBlocks is the YAML shape of the thresholds: and tracking: blocks,
// as src/Resources.cpp reads them.
type advancedBlocks struct {
	Thresholds struct {
		Circularity        *float64 `yaml:"circularity"`
		Score              *float64 `yaml:"score"`
		Blobs              *int     `yaml:"blobs"`
		MinConfidence      *float64 `yaml:"min_confidence"`
		MinCamEdgeDistance *float64 `yaml:"min_cam_edge_distance"`
		ResamplingFactor   *float64 `yaml:"resampling_factor"`
		ClippingTolerance  *float64 `yaml:"clipping_tolerance"`
		GeometryTolerance  *float64 `yaml:"geometry_tolerance"`
	} `yaml:"thresholds"`
	Tracking struct {
		MinTrackingRadius  *float64 `yaml:"min_tracking_radius"`
		MaxBotAcceleration *float64 `yaml:"max_bot_acceleration"`
	} `yaml:"tracking"`
}

// validateAdvanced checks the thresholds and tracking values in an effective
// config for what vision_processor would accept but misbehave on: a zero or
// negative blob budget or resampling factor, a confidence outside 0-1, or a
// negative distance.
func validateAdvanced(effective map[string]any) error {
	data, err := yaml.Marshal(map[string]any{
		"thresholds": effective["thresholds"],
		"tracking":   effective["tracking"],
	})
	if err != nil {
		return err
	}

	var b advancedBlocks
	if err := yaml.Unmarshal(data, &b); err != nil {
		return fmt.Errorf("thresholds/tracking: %w", err)
	}

	var errs error

	add := func(format string, args ...any) { errs = errors.Join(errs, fmt.Errorf(format, args...)) }

	t := b.Thresholds
	if t.Blobs != nil && *t.Blobs < 1 {
		add("thresholds.blobs: must be at least 1, got %d", *t.Blobs)
	}

	if t.MinConfidence != nil && (*t.MinConfidence < 0 || *t.MinConfidence > 1) {
		add("thresholds.min_confidence: want 0-1, got %g", *t.MinConfidence)
	}

	if t.ResamplingFactor != nil && *t.ResamplingFactor <= 0 {
		add("thresholds.resampling_factor: must be positive, got %g", *t.ResamplingFactor)
	}

	for _, f := range []struct {
		name  string
		value *float64
	}{
		{"thresholds.circularity", t.Circularity},
		{"thresholds.score", t.Score},
		{"thresholds.min_cam_edge_distance", t.MinCamEdgeDistance},
		{"thresholds.clipping_tolerance", t.ClippingTolerance},
		{"thresholds.geometry_tolerance", t.GeometryTolerance},
		{"tracking.min_tracking_radius", b.Tracking.MinTrackingRadius},
		{"tracking.max_bot_acceleration", b.Tracking.MaxBotAcceleration},
	} {
		if f.value != nil && *f.value < 0 {
			add("%s: must not be negative, got %g", f.name, *f.value)
		}
	}

	return errs
}
