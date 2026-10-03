package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func loadFixture(t *testing.T) Document {
	t.Helper()

	data, err := os.ReadFile("testdata/vision.yml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return doc
}

func TestParseReadsTheFixture(t *testing.T) {
	doc := loadFixture(t)

	if doc.Field.FieldLength != 9000 || len(doc.Cameras) != 2 {
		t.Fatalf("field_length=%d cameras=%d, want 9000 and 2", doc.Field.FieldLength, len(doc.Cameras))
	}

	c := doc.Cameras[0]
	if c.Seed == nil || c.Seed.Resolution != [2]int{1920, 1080} || len(c.Seed.LineCorners) != 4 {
		t.Fatalf("seed = %+v, want 1920x1080 with 4 corners", c.Seed)
	}

	calib, err := c.Calibration.Proto()
	if err != nil {
		t.Fatalf("Calibration.Proto: %v", err)
	}

	if calib.GetPixelImageWidth() != 1920 || calib.GetTz() != 4000 {
		t.Fatalf("calibration = %v, want pixel width 1920 and tz 4000", calib)
	}
}

func TestMarshalParseRoundTrips(t *testing.T) {
	doc := loadFixture(t)

	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	again, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(Marshal(doc)): %v\n%s", err, data)
	}

	if changes := Diff(doc, again); len(changes) != 0 {
		t.Fatalf("round trip changed the document: %+v", changes)
	}
}

// The browser sends every number as a JSON float; a document that went
// through JSON must still compare equal to the one read from YAML, or every
// edit would show bogus "1920 -> 1920" changes.
func TestDiffIgnoresNumberRepresentation(t *testing.T) {
	doc := loadFixture(t)

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var fromJSON Document
	if err := json.Unmarshal(data, &fromJSON); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	fromJSON.normalize()

	if changes := Diff(doc, fromJSON); len(changes) != 0 {
		t.Fatalf("JSON round trip produced changes: %+v", changes)
	}
}

func TestParseRejectsUnknownKeys(t *testing.T) {
	data, err := os.ReadFile("testdata/vision.yml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	typo := strings.Replace(string(data), "optional_field_lines:", "optional_feild_lines:", 1)

	if _, err := Parse([]byte(typo)); err == nil || !IsValidation(err) {
		t.Fatalf("Parse(typo) = %v, want a validation error", err)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	doc := loadFixture(t)
	doc.Cameras[1].CameraID = 0
	doc.Cameras[0].Seed.LineCorners = doc.Cameras[0].Seed.LineCorners[:3]
	doc.Field.GoalWidth = 0

	err := doc.Validate()
	if err == nil {
		t.Fatal("Validate accepted a broken document")
	}

	for _, want := range []string{"duplicate", "want 0 or 4 corners", "goal_width"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestValidateRejectsAMismatchedCalibrationCameraID(t *testing.T) {
	doc := loadFixture(t)
	doc.Cameras[0].Calibration.Camera["camera_id"] = 3

	if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), "camera_id") {
		t.Fatalf("Validate = %v, want a camera_id mismatch", err)
	}
}

func TestValidateRejectsAnOutOfRangeColor(t *testing.T) {
	doc := loadFixture(t)
	doc.Cameras[0].Config["color"] = map[string]any{"reference_force": 0.9, "history_force": 0.5}

	if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), "must not exceed 1") {
		t.Fatalf("Validate = %v, want a color force error", err)
	}
}

func TestImportBuildsADocumentFromLegacyFiles(t *testing.T) {
	doc, err := Import("testdata/legacy-geometry.yml", "testdata/legacy-config.yml")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if doc.Field.FieldLength != 2160 || !doc.OptionalFieldLines.Halfway {
		t.Errorf("field = %+v, want the legacy geometry", doc.Field)
	}

	if _, ok := doc.Models["straight_two_phase"]; !ok {
		t.Errorf("models = %v, want straight_two_phase carried over", doc.Models)
	}

	if len(doc.Cameras) != 1 {
		t.Fatalf("cameras = %d, want 1", len(doc.Cameras))
	}

	c := doc.Cameras[0]
	if c.CameraID != 1 || c.ConfigPath != "testdata/legacy-config.yml" {
		t.Errorf("camera_id=%d config_path=%q, want 1 and the legacy file", c.CameraID, c.ConfigPath)
	}

	if c.Seed == nil || len(c.Seed.LineCorners) != 4 || c.Seed.GoalSideMarker != 2 || c.Seed.Resolution != [2]int{} {
		t.Errorf("seed = %+v, want 4 corners, marker 2, unknown resolution", c.Seed)
	}

	geom, _ := c.Config["geometry"].(map[string]any)
	for _, derived := range []string{"line_corners", "goal_side_marker", "camera_amount"} {
		if _, ok := geom[derived]; ok {
			t.Errorf("camera config still has geometry.%s", derived)
		}
	}

	if _, ok := c.Config["color"]; !ok {
		t.Error("color should be per-camera")
	}

	if _, ok := doc.Defaults["thresholds"]; !ok {
		t.Error("thresholds should be a shared default")
	}

	if _, ok := doc.Defaults["tracking"]; ok {
		t.Error("an empty legacy section should not be carried over")
	}
}

func TestImportWithoutAConfigFileMakesCameraZero(t *testing.T) {
	doc, err := Import("testdata/legacy-geometry.yml", "")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(doc.Cameras) != 1 || doc.Cameras[0].CameraID != 0 || doc.Cameras[0].ConfigPath != "" {
		t.Fatalf("cameras = %+v, want a lone camera 0 with no config_path", doc.Cameras)
	}
}

func TestDiffClassifiesSections(t *testing.T) {
	before := loadFixture(t)
	after := before.clone()

	after.Field.FieldWidth = 5000
	after.Cameras[0].Seed.LineCorners[0] = [2]int{1, 2}
	after.Cameras[0].Config["color"].(map[string]any)["orange"] = []any{1, 2, 3}
	after.Defaults["color"].(map[string]any)["reference_force"] = 0.2
	after.Defaults["thresholds"].(map[string]any)["min_confidence"] = 0.4
	after.Cameras[0].Calibration.Camera["tz"] = 4100

	want := map[string]struct {
		section string
		camera  int // -1 for none
	}{
		"field.field_width":                  {SectionField, -1},
		"cameras[0].seed.line_corners":       {SectionGeometry, 0},
		"cameras[0].config.color.orange":     {SectionColor, 0},
		"defaults.color.reference_force":     {SectionColor, -1},
		"defaults.thresholds.min_confidence": {SectionOther, -1},
		"cameras[0].calibration.camera.tz":   {SectionGeometry, 0},
	}

	changes := Diff(before, after)
	if len(changes) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(changes), len(want), changes)
	}

	for _, c := range changes {
		w, ok := want[c.Path]
		if !ok {
			t.Errorf("unexpected change %q", c.Path)

			continue
		}

		gotCamera := -1
		if c.CameraID != nil {
			gotCamera = *c.CameraID
		}

		if c.Section != w.section || gotCamera != w.camera {
			t.Errorf("%s: section=%s camera=%d, want %s and %d", c.Path, c.Section, gotCamera, w.section, w.camera)
		}
	}
}

func TestDiffMatchesCamerasByIDNotPosition(t *testing.T) {
	before := loadFixture(t)
	after := before.clone()
	after.Cameras[0], after.Cameras[1] = after.Cameras[1], after.Cameras[0]

	if changes := Diff(before, after); len(changes) != 0 {
		t.Fatalf("reordering cameras produced changes: %+v", changes)
	}
}

func TestDiffReportsANewBlockAsOneChange(t *testing.T) {
	before := loadFixture(t)
	after := before.clone()
	after.Cameras = append(after.Cameras, Camera{CameraID: 2})
	after.Cameras[1].Calibration = before.Cameras[0].clone().Calibration

	paths := []string{}
	for _, c := range Diff(before, after) {
		paths = append(paths, c.Path)
	}

	want := []string{"cameras[1].calibration", "cameras[2]"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestRenderMergesDefaultsAndInjectsDerivedKeys(t *testing.T) {
	doc := loadFixture(t)

	data, err := Render(doc, doc.Cameras[0], "vision.yml")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var got map[string]any
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal render: %v\n%s", err, data)
	}

	camera := got["camera"].(map[string]any)
	if camera["driver"] != "OPENCV" || camera["width"] != 1920 {
		t.Errorf("camera = %v, want the default driver merged with the camera's own width", camera)
	}

	color := got["color"].(map[string]any)
	if color["reference_force"] != 0.1 || color["orange"].([]any)[0] != 200 {
		t.Errorf("color = %v, want default force with the camera's orange override", color)
	}

	geom := got["geometry"].(map[string]any)
	if got["cam_id"] != 0 || geom["camera_amount"] != 2 || len(geom["line_corners"].([]any)) != 4 {
		t.Errorf("cam_id=%v geometry=%v, want derived cam_id 0, camera_amount 2, 4 corners", got["cam_id"], geom)
	}

	if !strings.Contains(string(data), "- [1131, 949]") {
		t.Errorf("corners should render as flow pairs:\n%s", data)
	}
}

func TestRenderAllOnlyWritesChangedFiles(t *testing.T) {
	doc := loadFixture(t)
	path := filepath.Join(t.TempDir(), "config.yml")
	doc.Cameras[0].ConfigPath = path

	if failures := renderAll(doc, "vision.yml"); len(failures) != 0 {
		t.Fatalf("renderAll: %v", failures)
	}

	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	renderAll(doc, "vision.yml")

	if info, _ := os.Stat(path); !info.ModTime().Equal(old) {
		t.Fatalf("unchanged render rewrote the file")
	}

	doc.Cameras[0].Config["camera"].(map[string]any)["width"] = 1280
	renderAll(doc, "vision.yml")

	if info, _ := os.Stat(path); info.ModTime().Equal(old) {
		t.Fatalf("changed render did not rewrite the file")
	}
}

// clone of one camera, via the document clone, for tests.
func (c Camera) clone() Camera {
	return Document{Version: Version, Cameras: []Camera{c}}.clone().Cameras[0]
}
