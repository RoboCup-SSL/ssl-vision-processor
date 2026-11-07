package config

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestEffectiveConfigResolvesEverySource(t *testing.T) {
	doc := loadFixture(t)
	doc.Cameras[0].Config["thresholds"] = map[string]any{"made_up": 3}

	eff, err := doc.EffectiveConfig(0)
	if err != nil {
		t.Fatalf("EffectiveConfig: %v", err)
	}

	for path, want := range map[string]EffectiveSetting{
		"camera.path":                  {Value: "/dev/video0", Source: SourceCamera, Known: true},
		"camera.driver":                {Value: "OPENCV", Source: SourceDefaults, Known: true},
		"color.orange":                 {Value: []any{200, 120, 60}, Source: SourceCamera, Known: true},
		"color.reference_force":        {Value: 0.1, Source: SourceDefaults, Known: true},
		"thresholds.blobs":             {Value: 2000, Source: SourceFallback, Known: true},
		"cam_id":                       {Value: 0, Source: SourceDerived, Known: true},
		"geometry.camera_amount":       {Value: 2, Source: SourceDerived, Known: true},
		"thresholds.made_up":           {Value: 3, Source: SourceCamera},
		"debug.wait_for_geometry":      {Value: false, Source: SourceFallback, Known: true},
		"network.vision_port":          {Value: 10006, Source: SourceFallback, Known: true},
		"tracking.min_tracking_radius": {Value: 20, Source: SourceFallback, Known: true},
	} {
		if got := eff.Settings[path]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %#v, want %#v", path, got, want)
		}
	}

	if corners, _ := eff.Settings["geometry.line_corners"].Value.([][2]int); len(corners) != 4 {
		t.Errorf("geometry.line_corners = %v, want the seed's 4 corners", eff.Settings["geometry.line_corners"].Value)
	}
}

func TestEffectiveConfigPathFallbackFollowsTheID(t *testing.T) {
	doc := loadFixture(t)
	doc.Cameras[1].Config = map[string]any{"camera": map[string]any{"id": 3}}

	eff, err := doc.EffectiveConfig(1)
	if err != nil {
		t.Fatal(err)
	}

	if got := eff.Settings["camera.path"]; got.Value != "/dev/video3" || got.Source != SourceFallback {
		t.Errorf("camera.path = %+v, want the /dev/video3 fallback", got)
	}
}

func TestEffectiveConfigUnknownCamera(t *testing.T) {
	if _, err := loadFixture(t).EffectiveConfig(9); !errors.Is(err, ErrUnknownCamera) {
		t.Errorf("err = %v, want ErrUnknownCamera", err)
	}
}

func TestCameraStatusWarnsAboutUnknownSettings(t *testing.T) {
	doc := loadFixture(t)
	doc.Defaults["thresholds"] = map[string]any{"circularty": 20}

	status := cameraStatus(doc, doc.Cameras[1], nil)

	var codes []string
	for _, w := range status.Warnings {
		codes = append(codes, w.Code)
	}

	if !slices.Contains(codes, "unknown_setting") {
		t.Errorf("warnings = %v, want unknown_setting for thresholds.circularty", status.Warnings)
	}
}
