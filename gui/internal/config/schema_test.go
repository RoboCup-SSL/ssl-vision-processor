package config

import (
	"strings"
	"testing"
)

// withSetting is the fixture with path set in camera 0's own config.
func withSetting(t *testing.T, path string, value any) Document {
	t.Helper()

	doc := loadFixture(t)
	block, key, _ := strings.Cut(path, ".")

	cam := &doc.Cameras[0]
	if cam.Config == nil {
		cam.Config = map[string]any{}
	}

	values, _ := cam.Config[block].(map[string]any)
	if values == nil {
		values = map[string]any{}
	}

	values[key] = value
	cam.Config[block] = values

	return doc
}

// Every bound the schema advertises must be one Validate enforces, whichever
// validator owns the block.
func TestSchemaBoundsAreEnforced(t *testing.T) {
	for path, s := range CurrentSchema().Settings {
		if !strings.Contains(path, ".") || s.Apply == ApplyDerived {
			continue
		}

		_, key, _ := strings.Cut(path, ".")

		var outside []float64

		if s.Minimum != nil {
			if s.ExclusiveMinimum {
				outside = append(outside, *s.Minimum)
			} else {
				outside = append(outside, *s.Minimum-1)
			}
		}

		if s.Maximum != nil {
			outside = append(outside, *s.Maximum+1)
		}

		for _, v := range outside {
			var value any = v
			if s.Type == TypeInteger {
				value = int(v)
			}

			err := withSetting(t, path, value).Validate()
			if err == nil || !strings.Contains(err.Error(), key) {
				t.Errorf("%s = %v: Validate = %v, want an error naming %s", path, value, err, key)
			}
		}
	}
}

func TestSchemaDefaultsAreValid(t *testing.T) {
	for path, s := range CurrentSchema().Settings {
		if s.Default == nil || !strings.Contains(path, ".") {
			continue
		}

		doc := withSetting(t, path, s.Default)

		// Width and height are set both or neither.
		if pair, ok := map[string]string{"width": "height", "height": "width"}[strings.TrimPrefix(path, "camera.")]; ok {
			doc.Cameras[0].Config["camera"].(map[string]any)[pair] = s.Default
		}

		if err := doc.Validate(); err != nil {
			t.Errorf("%s = %v (its default): Validate = %v", path, s.Default, err)
		}
	}
}

func TestSchemaChecksTypes(t *testing.T) {
	for path, value := range map[string]any{
		"thresholds.blobs":             1.5,
		"tracking.min_tracking_radius": "far",
		"stream.active":                "yes",
		"debug.ground_truth":           3,
	} {
		if err := withSetting(t, path, value).Validate(); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%s = %v: Validate = %v, want an error naming it", path, value, err)
		}
	}
}

func TestSchemaColorDefaultsMatchTheFallbacks(t *testing.T) {
	color, err := colorFromBlock(nil)
	if err != nil {
		t.Fatal(err)
	}

	s := CurrentSchema().Settings
	if s["color.reference_force"].Default != color.ReferenceForce || s["color.history_force"].Default != color.HistoryForce {
		t.Errorf("schema color forces = %v/%v, colorFromBlock = %v/%v",
			s["color.reference_force"].Default, s["color.history_force"].Default, color.ReferenceForce, color.HistoryForce)
	}
}
