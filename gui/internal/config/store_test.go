package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/proto"
)

// fakeLive records what a Store pushed and serves canned live calibrations.
type fakeLive struct {
	mu       sync.Mutex
	field    geometry.FieldConfig
	locked   map[uint32]*vision.SSL_GeometryCameraCalibration
	unlocked []uint32
	live     map[uint32]*vision.SSL_GeometryCameraCalibration
}

func (f *fakeLive) ApplyConfig(field geometry.FieldConfig, _ geometry.OptionalLinesConfig, _ *vision.SSL_GeometryModels) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.field = field

	return nil
}

func (f *fakeLive) SetLocked(locked map[uint32]*vision.SSL_GeometryCameraCalibration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.locked = locked

	return nil
}

func (f *fakeLive) Unlock(camID uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.unlocked = append(f.unlocked, camID)
	delete(f.live, camID)

	return nil
}

func (f *fakeLive) LiveCalibration(camID uint32) *vision.SSL_GeometryCameraCalibration {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.live[camID]
}

func liveCalib(camID uint32, width, height uint32) *vision.SSL_GeometryCameraCalibration {
	return &vision.SSL_GeometryCameraCalibration{
		CameraId:         proto.Uint32(camID),
		FocalLength:      proto.Float32(900),
		PrincipalPointX:  proto.Float32(float32(width) / 2),
		PrincipalPointY:  proto.Float32(float32(height) / 2),
		Distortion:       proto.Float32(0),
		Q0:               proto.Float32(0),
		Q1:               proto.Float32(0),
		Q2:               proto.Float32(0),
		Q3:               proto.Float32(1),
		Tx:               proto.Float32(0),
		Ty:               proto.Float32(0),
		Tz:               proto.Float32(4000),
		PixelImageWidth:  proto.Uint32(width),
		PixelImageHeight: proto.Uint32(height),
	}
}

// testStore opens a copy of the fixture in a temp dir, with camera 0's
// config.yml rendered alongside it.
func testStore(t *testing.T) (*Store, *fakeLive, string) {
	t.Helper()

	dir := t.TempDir()
	doc := loadFixture(t)
	doc.Cameras[0].ConfigPath = "config-0.yml"

	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	path := filepath.Join(dir, "vision.yml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	live := &fakeLive{live: map[uint32]*vision.SSL_GeometryCameraCalibration{}}

	s, err := Open(path, live)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	return s, live, path
}

func TestOpenAppliesAndRenders(t *testing.T) {
	s, live, path := testStore(t)

	if live.field.FieldLength != 9000 {
		t.Errorf("applied field_length = %d, want 9000", live.field.FieldLength)
	}

	if _, ok := live.locked[0]; !ok || len(live.locked) != 1 {
		t.Errorf("locked = %v, want camera 0's locked calibration", live.locked)
	}

	rendered, err := os.ReadFile(filepath.Join(filepath.Dir(path), "config-0.yml"))
	if err != nil || !strings.Contains(string(rendered), "cam_id: 0") {
		t.Errorf("rendered config.yml = %q (%v), want cam_id 0", rendered, err)
	}

	if state := s.State(); len(state.Changes) != 0 || state.External != nil {
		t.Errorf("fresh store state = %+v, want clean", state)
	}
}

func TestUpdateAppliesLiveAndSaveClearsTheChanges(t *testing.T) {
	s, live, path := testStore(t)

	doc, rev := s.Working()
	doc.Field.FieldWidth = 5000

	rev, err := s.Update(rev, doc)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if live.field.FieldWidth != 5000 {
		t.Errorf("live field_width = %d, want 5000 applied immediately", live.field.FieldWidth)
	}

	if changes := s.State().Changes; len(changes) != 1 || changes[0].Path != "field.field_width" {
		t.Fatalf("changes = %+v, want just field.field_width", changes)
	}

	if err := s.Save(rev, false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if changes := s.State().Changes; len(changes) != 0 {
		t.Fatalf("changes after save = %+v, want none", changes)
	}

	data, _ := os.ReadFile(path)

	saved, err := Parse(data)
	if err != nil || saved.Field.FieldWidth != 5000 {
		t.Fatalf("saved file field_width = %d (%v), want 5000", saved.Field.FieldWidth, err)
	}
}

func TestUpdateRejectsAStaleRevision(t *testing.T) {
	s, _, _ := testStore(t)
	doc, rev := s.Working()

	if _, err := s.Update(rev, doc); err != nil {
		t.Fatalf("first Update: %v", err)
	}

	if _, err := s.Update(rev, doc); !errors.Is(err, ErrStale) {
		t.Fatalf("second Update with the old revision = %v, want ErrStale", err)
	}
}

func TestUpdateRejectsAnInvalidDocumentAndChangesNothing(t *testing.T) {
	s, live, _ := testStore(t)
	doc, rev := s.Working()
	doc.Field.FieldLength = -1

	if _, err := s.Update(rev, doc); !IsValidation(err) {
		t.Fatalf("Update = %v, want a validation error", err)
	}

	if live.field.FieldLength != 9000 {
		t.Fatalf("live field_length = %d, want unchanged 9000", live.field.FieldLength)
	}

	if _, after := s.Working(); after != rev {
		t.Fatalf("revision moved on a rejected update")
	}
}

func TestSaveRefusesToOverwriteAnExternalChangeUnlessForced(t *testing.T) {
	s, _, path := testStore(t)
	_, rev := s.Working()

	writeExternal(t, path, "field_width: 6000", "field_width: 6100")

	if err := s.Save(rev, false); !errors.Is(err, ErrDiskChanged) {
		t.Fatalf("Save = %v, want ErrDiskChanged", err)
	}

	if ext := s.State().External; ext == nil {
		t.Fatal("refused save should record the external change")
	}

	if err := s.Save(rev, true); err != nil {
		t.Fatalf("forced Save: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "field_width: 6000") {
		t.Fatalf("forced save did not overwrite the external edit:\n%s", data)
	}

	if s.State().External != nil {
		t.Fatal("forced save should clear the external change")
	}
}

func TestWatcherReportsExternalEditsButNotOwnSaves(t *testing.T) {
	s, _, path := testStore(t)
	doc, rev := s.Working()
	doc.Field.FieldWidth = 5000

	rev, _ = s.Update(rev, doc)
	if err := s.Save(rev, false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if s.checkDisk() {
		t.Fatal("watcher fired on the store's own save")
	}

	writeExternal(t, path, "tz: 4000", "tz: 4200")

	if !s.checkDisk() {
		t.Fatal("watcher missed an external edit")
	}

	ext := s.State().External
	if ext == nil || ext.Error != "" || len(ext.Changes) != 1 || ext.Changes[0].Path != "cameras[0].calibration.camera.tz" {
		t.Fatalf("external = %+v, want one calibration tz change", ext)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	working, _ := s.Working()
	if tz := working.Cameras[0].Calibration.Camera["tz"]; tz != 4200 {
		t.Errorf("tz after accepting the disk version = %v, want 4200", tz)
	}

	if s.State().External != nil {
		t.Error("Reload should clear the external change")
	}
}

func TestWatcherReportsAnUnparseableFile(t *testing.T) {
	s, _, path := testStore(t)

	writeExternal(t, path, "version: 2", "version: 2\nbogus_key: true")

	if !s.checkDisk() {
		t.Fatal("watcher missed an external edit")
	}

	if ext := s.State().External; ext == nil || ext.Error == "" {
		t.Fatalf("external = %+v, want a parse error", ext)
	}
}

func TestLockAndUnlockCalibration(t *testing.T) {
	s, live, _ := testStore(t)
	live.live[1] = liveCalib(1, 1280, 720)

	if _, err := s.LockCalibration(1); err != nil {
		t.Fatalf("LockCalibration: %v", err)
	}

	working, _ := s.Working()
	if c := working.camera(1); c.Calibration == nil || c.Calibration.FieldHash != fieldHash(working.Field) {
		t.Fatalf("camera 1 calibration = %+v, want locked with the current field hash", c.Calibration)
	}

	if _, ok := live.locked[1]; !ok {
		t.Fatal("locking should publish the calibration as locked")
	}

	if status := s.State().Cameras[1]; status.Calibration != CalibrationLocked {
		t.Fatalf("status = %+v, want locked", status)
	}

	if _, err := s.UnlockCalibration(1); err != nil {
		t.Fatalf("UnlockCalibration: %v", err)
	}

	working, _ = s.Working()
	if working.camera(1).Calibration != nil {
		t.Fatal("unlock should remove the calibration from the document")
	}

	if _, ok := live.locked[1]; ok || len(live.unlocked) != 1 || live.unlocked[0] != 1 {
		t.Fatalf("locked=%v unlocked=%v, want camera 1 withheld", live.locked, live.unlocked)
	}
}

func TestLockWithoutALiveCalibrationFails(t *testing.T) {
	s, _, _ := testStore(t)

	if _, err := s.LockCalibration(1); !errors.Is(err, ErrNoLiveCalibration) {
		t.Fatalf("LockCalibration = %v, want ErrNoLiveCalibration", err)
	}

	if _, err := s.LockCalibration(9); !errors.Is(err, ErrUnknownCamera) {
		t.Fatalf("LockCalibration(9) = %v, want ErrUnknownCamera", err)
	}
}

func TestStatusWarnings(t *testing.T) {
	s, live, _ := testStore(t)

	codes := func(camID int) string {
		for _, c := range s.State().Cameras {
			if c.CameraID == camID {
				var out []string
				for _, w := range c.Warnings {
					out = append(out, w.Code)
				}

				return strings.Join(out, ",")
			}
		}

		return "missing"
	}

	// The fixture's field_hash is a placeholder, so camera 0's lock is stale.
	if got := codes(0); got != "field_changed" {
		t.Errorf("camera 0 warnings = %q, want field_changed", got)
	}

	live.live[0] = liveCalib(0, 1280, 720) // same 16:9 as the 1920x1080 seed
	if got := codes(0); got != "field_changed,seed_rescalable" {
		t.Errorf("camera 0 warnings = %q, want field_changed,seed_rescalable", got)
	}

	live.live[0] = liveCalib(0, 640, 480) // 4:3, like the camera that broke the picker
	if got := codes(0); got != "field_changed,calibration_aspect_mismatch,seed_aspect_mismatch" {
		t.Errorf("camera 0 warnings = %q, want aspect mismatches", got)
	}
}

func TestSaveAsSwitchesThePath(t *testing.T) {
	s, _, path := testStore(t)
	_, rev := s.Working()

	if err := s.SaveAs(rev, "other.yml"); err != nil {
		t.Fatalf("SaveAs: %v", err)
	}

	newPath := filepath.Join(filepath.Dir(path), "other.yml")
	if state := s.State(); state.Path != newPath || state.File != "other.yml" {
		t.Fatalf("path = %q, file = %q, want %q", state.Path, state.File, newPath)
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("SaveAs did not write: %v", err)
	}
}

// Everything the API can reach has to stay in the one flat folder.
func TestSaveAsAndLoadStayInTheConfigFolder(t *testing.T) {
	s, _, path := testStore(t)
	dir := filepath.Dir(path)
	_, rev := s.Working()

	outside := filepath.Join(t.TempDir(), "outside.yml")

	if err := os.WriteFile(outside, []byte("not a config"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(outside, filepath.Join(dir, "link.yml")); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{outside, "../outside.yml", "sub/outside.yml", ".hidden.yml", "outside.txt", "link.yml"} {
		if err := s.SaveAs(rev, name); !IsValidation(err) {
			t.Errorf("SaveAs(%q) = %v, want a validation error", name, err)
		}

		if err := s.Load(name); !IsValidation(err) {
			t.Errorf("Load(%q) = %v, want a validation error", name, err)
		}
	}

	if got := s.State().Path; got != path {
		t.Errorf("path = %q, want it unchanged", got)
	}

	if data, _ := os.ReadFile(outside); string(data) != "not a config" {
		t.Errorf("the file outside was changed: %q", data)
	}
}

func TestSaveAsWontOverwriteFilesTheConfigUses(t *testing.T) {
	s, _, _ := testStore(t)
	_, rev := s.Working()

	for _, name := range []string{"config-0.yml", "robot-heights.yml"} {
		if err := s.SaveAs(rev, name); !IsValidation(err) {
			t.Errorf("SaveAs(%q) = %v, want a validation error", name, err)
		}
	}
}

func TestAConfigPathCantBeTheConfigItself(t *testing.T) {
	s, _, path := testStore(t)
	doc, rev := s.Working()
	doc.Cameras[0].ConfigPath = filepath.Base(path)

	if _, err := s.Update(rev, doc); !IsValidation(err) {
		t.Fatalf("Update = %v, want a validation error", err)
	}

	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "field_length") {
		t.Errorf("the config was overwritten:\n%s", data)
	}
}

func TestOnChangeFiresAfterEdits(t *testing.T) {
	s, _, _ := testStore(t)

	calls := 0
	s.OnChange(func() {
		_ = s.State() // must not deadlock: called without the lock held
		calls++
	})

	doc, rev := s.Working()
	if _, err := s.Update(rev, doc); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if calls != 1 {
		t.Fatalf("OnChange calls = %d, want 1", calls)
	}
}

// writeExternal edits path the way a person would, replacing one substring.
// It also changes the size, so the watcher's mtime+size check can't miss it
// within one filesystem timestamp tick.
func writeExternal(t *testing.T, path, old, replacement string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	edited := strings.Replace(string(data), old, replacement, 1)
	if edited == string(data) {
		t.Fatalf("%q not found in %s", old, path)
	}

	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// firstRunStore is testStore on a config the host has just created.
func firstRunStore(t *testing.T) (*Store, string) {
	t.Helper()

	s, _, path := testStore(t)
	doc, _ := s.Working()
	doc.FirstRun = true

	data, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}

	return s, path
}

func TestAnEditCantChangeFirstRun(t *testing.T) {
	s, _ := firstRunStore(t)

	doc, rev := s.Working()
	doc.FirstRun = false

	if _, err := s.Update(rev, doc); err != nil {
		t.Fatal(err)
	}

	if !s.FirstRun() {
		t.Error("an edit cleared first run")
	}

	s2, _, _ := testStore(t)
	doc, rev = s2.Working()
	doc.FirstRun = true

	if _, err := s2.Update(rev, doc); err != nil {
		t.Fatal(err)
	}

	if s2.FirstRun() {
		t.Error("an edit set first run")
	}
}

func TestSavingEndsFirstRunAndDropsItFromTheFile(t *testing.T) {
	s, path := firstRunStore(t)

	if changes := s.State().Changes; len(changes) != 0 {
		t.Fatalf("changes = %+v, want none: first run isn't an edit", changes)
	}

	_, rev := s.Working()
	if err := s.Save(rev, false); err != nil {
		t.Fatal(err)
	}

	if s.FirstRun() || s.State().FirstRun {
		t.Error("still a first run after saving")
	}

	if changes := s.State().Changes; len(changes) != 0 {
		t.Errorf("changes = %+v, want none after saving", changes)
	}

	if data, _ := os.ReadFile(path); strings.Contains(string(data), "first_run") {
		t.Errorf("first_run is still in the file:\n%s", data)
	}
}

func TestSaveAsEndsFirstRun(t *testing.T) {
	s, _ := firstRunStore(t)

	_, rev := s.Working()
	if err := s.SaveAs(rev, "other.yml"); err != nil {
		t.Fatal(err)
	}

	if s.FirstRun() {
		t.Error("still a first run after Save As")
	}
}

// Publishing starts only once the confirmation has actually reached the disk.
func TestAFailedSaveKeepsFirstRun(t *testing.T) {
	s, path := firstRunStore(t)

	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}

	_, rev := s.Working()
	if err := s.Save(rev, false); err == nil {
		t.Fatal("Save over a read-only file: want an error")
	}

	if !s.FirstRun() {
		t.Error("a failed save ended first run")
	}
}

func TestLocalCamerasGetConfigPaths(t *testing.T) {
	_, _, path := testStore(t)

	s, err := OpenWith(path, &fakeLive{live: map[uint32]*vision.SSL_GeometryCameraCalibration{}}, Options{LocalCameras: true})
	if err != nil {
		t.Fatal(err)
	}

	doc, rev := s.Working()
	if got := doc.camera(1).ConfigPath; got != "cam1.yml" {
		t.Fatalf("camera 1 config_path = %q, want cam1.yml", got)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "cam1.yml")); err != nil {
		t.Errorf("cam1.yml not rendered: %v", err)
	}

	// Assigned, not yet saved: it shows as a change to save.
	if changes := s.State().Changes; len(changes) == 0 {
		t.Error("no unsaved changes after assigning a config_path")
	}

	// An edit that leaves one out gets it back, and this host as its instance.
	doc.camera(1).ConfigPath = ""
	doc.camera(1).Instance = ""
	if _, err := s.Update(rev, doc); err != nil {
		t.Fatal(err)
	}

	host, _ := os.Hostname()
	doc, rev = s.Working()
	if doc.camera(1).ConfigPath != "cam1.yml" || doc.camera(1).Instance != host {
		t.Errorf("after edit, camera 1 config_path = %q, instance = %q, want cam1.yml, %q", doc.camera(1).ConfigPath, doc.camera(1).Instance, host)
	}

	if !s.State().LocalCameras {
		t.Error("state.localCameras = false, want true")
	}

	// More regions each get a camera, on its own device.
	doc.Layout = &Layout{CameraCount: 4}
	if _, err := s.Update(rev, doc); err != nil {
		t.Fatal(err)
	}

	doc, _ = s.Working()
	for id := 2; id < 4; id++ {
		c := doc.camera(id)
		if c == nil {
			t.Fatalf("camera %d not added for the 4-camera layout", id)
		}

		if c.Instance != host || c.ConfigPath != fmt.Sprintf("cam%d.yml", id) {
			t.Errorf("camera %d = %+v, want instance %q, cam%d.yml", id, *c, host, id)
		}
	}

	devices := map[string]int{}
	for _, c := range doc.Cameras {
		driver, device := doc.CaptureDevice(c)
		if other, ok := devices[driver+" "+device]; ok && c.CameraID >= 2 { // 0 and 1 are the fixture's, on two hosts
			t.Errorf("cameras %d and %d both open %s %s", other, c.CameraID, driver, device)
		}

		devices[driver+" "+device] = c.CameraID
	}
}

func TestRenderNamesTheHeightsTableBesideTheSource(t *testing.T) {
	doc := loadFixture(t)

	data, err := Render(doc, doc.Cameras[0], "/srv/rcssl/vision.yml")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "bot_heights_file: /srv/rcssl/robot-heights.yml\n") {
		t.Errorf("render = %s\nwant an absolute bot_heights_file", data)
	}
}
