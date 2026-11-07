package config

import (
	"strings"
	"testing"
)

func TestValidateCamera(t *testing.T) {
	cases := map[string]struct {
		block map[string]any
		want  string // "" for valid
	}{
		"fixture":         {map[string]any{"driver": "OPENCV", "path": "/dev/video0"}, ""},
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
