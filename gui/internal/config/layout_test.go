package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCameraCountDefaultsToCamerasRoundedUp(t *testing.T) {
	for _, tc := range []struct{ cameras, want int }{{0, 1}, {1, 1}, {2, 2}, {3, 4}, {4, 4}} {
		doc := Document{Cameras: make([]Camera, tc.cameras)}
		if got := doc.CameraCount(); got != tc.want {
			t.Errorf("%d cameras: CameraCount = %d, want %d", tc.cameras, got, tc.want)
		}
	}

	doc := Document{Layout: &Layout{CameraCount: 4}, Cameras: make([]Camera, 1)}
	if got := doc.CameraCount(); got != 4 {
		t.Errorf("layout 4 with 1 camera: CameraCount = %d, want 4", got)
	}
}

func TestValidateChecksTheLayout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*Document)
		reason string
	}{
		{"unsupported count", func(d *Document) { d.Layout = &Layout{CameraCount: 3} }, "layout.camera_count"},
		{"count above 4", func(d *Document) { d.Layout = &Layout{CameraCount: 8} }, "layout.camera_count"},
		{"id past the count", func(d *Document) { d.Cameras[1].CameraID = 2 }, "must be below layout.camera_count"},
		{"same device twice", func(d *Document) {
			d.Cameras[1].Instance = d.Cameras[0].Instance
			d.Cameras[1].Config = map[string]any{"camera": map[string]any{"path": "/dev/video0"}}
		}, "same host and camera device"},
		{"odd rotation", func(d *Document) { d.Cameras[0].Display = &Display{Rotate: 45} }, "display.rotate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadFixture(t)
			tc.edit(&doc)

			if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Validate = %v, want an error mentioning %q", err, tc.reason)
			}
		})
	}
}

func TestValidateAllowsOneHostRunningSeveralCameras(t *testing.T) {
	doc := loadFixture(t)
	doc.Layout = &Layout{CameraCount: 4}
	doc.Cameras[1].Instance = doc.Cameras[0].Instance
	doc.Cameras[1].Config = map[string]any{"camera": map[string]any{"path": "/dev/video2"}}
	doc.Cameras[1].CameraID = 3
	doc.Cameras[0].Display = &Display{Rotate: 270, Mirror: true}

	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestRenderUsesTheLayoutCount(t *testing.T) {
	doc := loadFixture(t)
	doc.Layout = &Layout{CameraCount: 4}

	data, err := Render(doc, doc.Cameras[0], "vision.yml")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var got map[string]any
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if amount := got["geometry"].(map[string]any)["camera_amount"]; amount != 4 {
		t.Errorf("camera_amount = %v, want 4", amount)
	}

	if strings.Contains(string(data), "display") {
		t.Errorf("render should not include display settings:\n%s", data)
	}
}

func TestStatusFlagsASeedPickedForAnotherSlot(t *testing.T) {
	doc := loadFixture(t)
	c := doc.Cameras[0]

	if hasWarning(cameraStatus(doc, c, nil), "seed_stale") {
		t.Fatalf("seed without a slot should not be stale")
	}

	c.Seed.Slot = &Slot{CameraID: 0, CameraCount: 2}
	if hasWarning(cameraStatus(doc, c, nil), "seed_stale") {
		t.Fatalf("seed picked for the current slot should not be stale")
	}

	doc.Layout = &Layout{CameraCount: 4}
	if !hasWarning(cameraStatus(doc, c, nil), "seed_stale") {
		t.Errorf("seed picked as 0 of 2 should be stale once the count is 4")
	}

	doc.Layout = nil
	c.CameraID = 1
	if !hasWarning(cameraStatus(doc, c, nil), "seed_stale") {
		t.Errorf("seed picked as camera 0 should be stale on camera 1")
	}
}

func TestDiffClassifiesLayoutChanges(t *testing.T) {
	before := loadFixture(t)
	after := before.clone()
	after.Layout = &Layout{CameraCount: 4}
	after.Cameras[0].Instance = "host-c"
	after.Cameras[1].Display = &Display{Rotate: 90}

	changes := Diff(before, after)
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3: %+v", len(changes), changes)
	}

	for _, c := range changes {
		if c.Section != SectionLayout {
			t.Errorf("%s: section %s, want %s", c.Path, c.Section, SectionLayout)
		}
	}
}

func TestImportKeepsTheLegacyCameraAmount(t *testing.T) {
	doc, err := Import("testdata/legacy-geometry.yml", "testdata/legacy-config.yml")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if doc.CameraCount() != 2 {
		t.Errorf("CameraCount = %d, want the legacy camera_amount 2", doc.CameraCount())
	}
}

func hasWarning(s CameraStatus, code string) bool {
	for _, w := range s.Warnings {
		if w.Code == code {
			return true
		}
	}

	return false
}

func TestUpdateWithholdsACalibrationTheEditRemoved(t *testing.T) {
	s, live, _ := testStore(t)

	doc, rev := s.Working()
	if doc.camera(0).Calibration == nil {
		t.Fatal("fixture camera 0 should have a locked calibration")
	}

	// Swap the two cameras' slots, dropping the calibration that no longer
	// matches its region.
	doc.Cameras[0].CameraID, doc.Cameras[1].CameraID = 1, 0
	doc.Cameras[0].Calibration = nil

	if _, err := s.Update(rev, doc); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if len(live.unlocked) != 1 || live.unlocked[0] != 0 {
		t.Fatalf("unlocked = %v, want camera 0 withheld", live.unlocked)
	}
}

func TestDiffReportsASwapAsCameraIDChanges(t *testing.T) {
	before := loadFixture(t)
	before.Layout = &Layout{CameraCount: 4}
	after := before.clone()

	// Swap the two cameras' slots; camera 0 also loses its calibration.
	after.Cameras[0].CameraID, after.Cameras[1].CameraID = 1, 0
	after.Cameras[0].Calibration = nil

	got := map[string]Change{}
	for _, c := range Diff(before, after) {
		got[c.Path] = c
	}

	if len(got) != 3 {
		t.Fatalf("got %d changes, want 2 camera_id changes and the calibration: %v", len(got), got)
	}

	// Camera 0 (host-a) is now filed under 1, its new slot.
	if c := got["cameras[1].camera_id"]; c.Before != 0 || c.After != 1 || c.Section != SectionLayout {
		t.Errorf("cameras[1].camera_id = %+v, want 0 -> 1 in layout", c)
	}

	if c := got["cameras[0].camera_id"]; c.Before != 1 || c.After != 0 {
		t.Errorf("cameras[0].camera_id = %+v, want 1 -> 0", c)
	}

	if c, ok := got["cameras[1].calibration"]; !ok || c.After != nil {
		t.Errorf("cameras[1].calibration = %+v, want removed", c)
	}
}

func TestDiffFallsBackToIDsWhenAMoveCantBeMatched(t *testing.T) {
	before := loadFixture(t)
	after := before.clone()

	// Camera 1 changes device as well as slot, so only camera 0 matches.
	after.Cameras[0].CameraID, after.Cameras[1].CameraID = 1, 0
	after.Cameras[0].Calibration = nil
	after.Cameras[1].Config = map[string]any{"camera": map[string]any{"path": "/dev/video4"}}

	for _, c := range Diff(before, after) {
		if strings.HasSuffix(c.Path, ".camera_id") {
			t.Errorf("unexpected %s: a partial match would file two cameras under one id", c.Path)
		}
	}
}

func TestValidateChecksTeamOverrides(t *testing.T) {
	tall, both := 900.0, 150.0

	for _, tc := range []struct {
		name      string
		overrides map[string]TeamOverride
		reason    string
	}{
		{"custom height too tall", map[string]TeamOverride{"Custom FC": {Height: &tall}}, `teams.overrides["Custom FC"].height`},
		{"name and height", map[string]TeamOverride{"Custom FC": {Name: "ER-Force", Height: &both}}, "set name or height"},
		{"empty team name", map[string]TeamOverride{"": {Height: &both}}, "can't be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := loadFixture(t)
			doc.Teams = &Teams{Overrides: tc.overrides}

			if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("Validate = %v, want an error mentioning %q", err, tc.reason)
			}
		})
	}

	// A custom height not entered yet is allowed; the GUI warns about it.
	doc := loadFixture(t)
	doc.Teams = &Teams{Overrides: map[string]TeamOverride{"Custom FC": {Height: &both}, "ER Force": {Name: "ER-Force"}, "New FC": {}}}

	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	after := doc.clone()
	after.Teams.Overrides["Custom FC"] = TeamOverride{Height: &tall}
	for _, c := range Diff(doc, after) {
		if c.Section != SectionOverview {
			t.Errorf("%s: section %s, want %s", c.Path, c.Section, SectionOverview)
		}
	}
}

func TestValidateChecksColorOverridesAndClassifiesCompetition(t *testing.T) {
	tall, ok := 900.0, 150.0

	doc := loadFixture(t)
	doc.Teams = &Teams{ByColor: &ColorOverrides{Blue: &TeamOverride{Height: &tall}}}

	if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), "teams.by_color.blue.height") {
		t.Fatalf("Validate = %v, want a by_color height error", err)
	}

	doc.Teams.ByColor.Blue.Height = &ok
	if err := doc.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	after := doc.clone()
	after.CompetitionField = true
	changes := Diff(doc, after)
	if len(changes) != 1 || changes[0].Path != "competition_field" || changes[0].Section != SectionOverview {
		t.Fatalf("changes = %+v, want competition_field in overview", changes)
	}
}
