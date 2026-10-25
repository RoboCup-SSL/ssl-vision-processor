package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cameraBlockOf(t *testing.T, s Settings) map[string]any {
	t.Helper()

	block, ok := s["camera"].(map[string]any)
	if !ok {
		t.Fatalf("no camera block in %v", s)
	}

	return block
}

func TestRenderConvertsExposureForTheDriver(t *testing.T) {
	for driver, want := range map[string]string{
		"OPENCV":    "exposure: 0.066",
		"SPINNAKER": "exposure: 6.6",
		"MVIMPACT":  "exposure: 6.6",
	} {
		doc := Document{Version: Version, Cameras: []Camera{{
			Config: Settings{"camera": map[string]any{"driver": driver, "exposure": 6600}},
		}}}

		data, err := Render(doc, doc.Cameras[0], "vision.yml")
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(data), want) {
			t.Errorf("%s: want %q in\n%s", driver, want, data)
		}

		// Auto stays 0.
		doc.Cameras[0].Config["camera"].(map[string]any)["exposure"] = 0
		if data, _ := Render(doc, doc.Cameras[0], "vision.yml"); !strings.Contains(string(data), "exposure: 0\n") {
			t.Errorf("%s auto: got\n%s", driver, data)
		}
	}
}

func TestImportConvertsExposureToMicros(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	if err := os.WriteFile(path, []byte("camera:\n  driver: OPENCV\n  exposure: 0.066\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	camera, _, _, err := importConfig(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := cameraBlockOf(t, camera.Config)["exposure"]; got != 6600 {
		t.Errorf("exposure = %v, want 6600 µs", got)
	}
}

func TestVersion1ExposureIsMigrated(t *testing.T) {
	doc, err := Parse([]byte(`version: 1
field: {field_length: 9000, field_width: 6000, goal_width: 1000, goal_depth: 180, goal_height: 160,
  penalty_area_depth: 1000, penalty_area_width: 2000, goal_center_to_penalty_mark: 6000,
  boundary_width: 300, boundary_width_goal_line: 300, center_circle_radius: 500, line_thickness: 10,
  ball_radius: 21.5, max_robot_radius: 90, goal_substitution_area_width: 0}
optional_field_lines: {}
defaults:
  camera: {driver: SPINNAKER, exposure: 5}
layout: {camera_count: 4}
cameras:
  - camera_id: 0
  - camera_id: 1
    config: {camera: {driver: OPENCV, path: /dev/video1}}
  - camera_id: 2
    config: {camera: {driver: OPENCV, path: /dev/video2, exposure: 0.066}}
`))
	if err != nil {
		t.Fatal(err)
	}

	if doc.Version != Version {
		t.Errorf("version = %d, want %d", doc.Version, Version)
	}

	if got := cameraBlockOf(t, doc.Defaults)["exposure"]; got != 5000 {
		t.Errorf("defaults exposure = %v, want 5000", got)
	}

	// Inherited 5 under OPENCV was 500 ms; it keeps running with that.
	if got := cameraBlockOf(t, doc.Cameras[1].Config)["exposure"]; got != 500000 {
		t.Errorf("camera 1 exposure = %v, want 500000", got)
	}

	if got := cameraBlockOf(t, doc.Cameras[2].Config)["exposure"]; got != 6600 {
		t.Errorf("camera 2 exposure = %v, want 6600", got)
	}

	// Every camera renders what it rendered before.
	for id, want := range map[int]string{0: "exposure: 5\n", 1: "exposure: 5\n", 2: "exposure: 0.066\n"} {
		if data, _ := Render(doc, doc.Cameras[id], "v"); !strings.Contains(string(data), want) {
			t.Errorf("camera %d renders\n%s\nwant %q", id, data, want)
		}
	}
}
