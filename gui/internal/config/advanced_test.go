package config

import (
	"strings"
	"testing"
)

func TestValidateChecksThresholdsAndTracking(t *testing.T) {
	for _, tc := range []struct {
		name   string
		block  string
		values map[string]any
		reason string
	}{
		{"no blobs", "thresholds", map[string]any{"blobs": 0}, "thresholds.blobs"},
		{"confidence above 1", "thresholds", map[string]any{"min_confidence": 1.5}, "thresholds.min_confidence"},
		{"zero resampling", "thresholds", map[string]any{"resampling_factor": 0}, "thresholds.resampling_factor"},
		{"negative tolerance", "thresholds", map[string]any{"geometry_tolerance": -1}, "thresholds.geometry_tolerance"},
		{"negative acceleration", "tracking", map[string]any{"max_bot_acceleration": -2}, "tracking.max_bot_acceleration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadFixture(t)
			doc.Defaults[tc.block] = tc.values

			if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Validate = %v, want an error mentioning %q", err, tc.reason)
			}
		})
	}
}

func TestValidateAcceptsSensibleThresholds(t *testing.T) {
	doc := loadFixture(t)
	doc.Defaults["thresholds"] = map[string]any{"circularity": 20.0, "blobs": 3000, "min_confidence": 0.3, "resampling_factor": 0.5}
	doc.Defaults["tracking"] = map[string]any{"min_tracking_radius": 30.0, "max_bot_acceleration": 8.0}

	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
