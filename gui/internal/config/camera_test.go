package config

import (
	"strings"
	"testing"
)

// Serial numbers are usually written bare, which YAML reads as an integer.
func TestDeviceKeyDevice(t *testing.T) {
	key := func(block map[string]any) string { return deviceKey("host", block) }

	first := key(map[string]any{"driver": "SPINNAKER", "device": "15000001"})

	if second := key(map[string]any{"driver": "SPINNAKER", "device": "15000002"}); first == second {
		t.Errorf("different serials share the key %q", first)
	}

	if bare := key(map[string]any{"driver": "SPINNAKER", "device": 15000001}); bare != first {
		t.Errorf("bare serial = %q, want %q", bare, first)
	}

	if former := key(map[string]any{"driver": "SPINNAKER", "path": "15000001"}); former != first {
		t.Errorf("former name path = %q, want %q", former, first)
	}
}

func TestValidateCamera(t *testing.T) {
	cases := map[string]struct {
		block map[string]any
		want  string // "" for valid
	}{
		"fixture":         {map[string]any{"driver": "OPENCV", "path": "/dev/video0"}, ""},
		"dc1394":          {map[string]any{"driver": "DC1394", "id": 1}, ""},
		"serial":          {map[string]any{"driver": "SPINNAKER", "device": "15000001"}, ""},
		"manual wb":       {map[string]any{"white_balance": map[string]any{"red": 1.2, "blue": 0.9}}, ""},
		"indoor":          {map[string]any{"white_balance": "INDOOR"}, ""},
		"unknown driver":  {map[string]any{"driver": "opencv"}, "camera.driver"},
		"lowercase wb":    {map[string]any{"white_balance": "outdoor"}, "camera.white_balance"},
		"negative wb":     {map[string]any{"white_balance": map[string]any{"red": -1.0}}, "camera.white_balance"},
		"half resolution": {map[string]any{"width": 1920}, "camera.width/height"},
		"negative gain":   {map[string]any{"gain": -2.0}, "camera.gain"},
		"zero gamma":      {map[string]any{"gamma": 0.0}, "camera.gamma"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doc := loadFixture(t)
			doc.Cameras[1].Config = map[string]any{"camera": tc.block}

			err := doc.Validate()

			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("Validate = %v, want valid", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), "cameras[1]: "+tc.want)):
				t.Fatalf("Validate = %v, want an error about %s", err, tc.want)
			}
		})
	}
}
