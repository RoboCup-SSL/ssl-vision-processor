package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
)

// testServer serves a *copy* of testdata/vision.yml -- saves write back to
// it, and would otherwise rewrite a tracked file on every run.
func testServer(t *testing.T) (http.Handler, string) {
	t.Helper()

	src, err := os.ReadFile("testdata/vision.yml")
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "vision.yml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	geom, store, err := openConfig(path)
	if err != nil {
		t.Fatalf("openConfig: %v", err)
	}

	return NewVisionServer(geom, store, hub.New(), video.NewManager(false), t.TempDir()), path
}

func do(t *testing.T, h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader

	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		data, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("Marshal body: %v", err)
		}

		reader = bytes.NewReader(data)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, reader))

	return rec
}

func getConfig(t *testing.T, h http.Handler) configResponse {
	t.Helper()

	rec := do(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config = %d: %s", rec.Code, rec.Body)
	}

	var resp configResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return resp
}

func conflictCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d (%s), want 409", rec.Code, rec.Body)
	}

	var resp conflictResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode conflict: %v", err)
	}

	return resp.Error
}

func TestHealthReturnsOK(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodGet, "/api/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	// json.Encoder.Encode terminates every value with a newline.
	if got, want := rec.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestGeometryReturnsProtojsonWithTheLockedCalibration(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodGet, "/api/geometry", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"fieldLength"`) || !strings.Contains(body, `"pixelImageWidth":1920`) {
		t.Errorf("body = %q, want field and the fixture's locked calibration", body)
	}
}

func TestGetConfigReturnsTheDocumentAndACleanState(t *testing.T) {
	h, path := testServer(t)
	resp := getConfig(t, h)

	if resp.Document.Field.FieldLength != 9000 || len(resp.Document.Cameras) != 2 {
		t.Errorf("document = %+v, want the fixture", resp.Document)
	}

	if resp.State.Path != path || len(resp.State.Changes) != 0 || resp.State.External != nil {
		t.Errorf("state = %+v, want a clean store at %s", resp.State, path)
	}
}

func TestPutConfigAppliesLiveWithoutSaving(t *testing.T) {
	h, path := testServer(t)
	before, _ := os.ReadFile(path)

	resp := getConfig(t, h)
	resp.Document.Field.FieldWidth = 5000

	rec := do(t, h, http.MethodPut, "/api/config", putConfigRequest{Revision: resp.State.Revision, Document: resp.Document})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d: %s", rec.Code, rec.Body)
	}

	if geo := do(t, h, http.MethodGet, "/api/geometry", nil).Body.String(); !strings.Contains(geo, `"fieldWidth":5000`) {
		t.Errorf("live geometry = %q, want field_width 5000 applied immediately", geo)
	}

	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Error("PUT wrote to disk; only save should")
	}

	if changes := getConfig(t, h).State.Changes; len(changes) != 1 || changes[0].Path != "field.field_width" {
		t.Errorf("changes = %+v, want field.field_width", changes)
	}
}

func TestPutConfigRejectsAStaleRevision(t *testing.T) {
	h, _ := testServer(t)
	resp := getConfig(t, h)
	req := putConfigRequest{Revision: resp.State.Revision, Document: resp.Document}

	if rec := do(t, h, http.MethodPut, "/api/config", req); rec.Code != http.StatusOK {
		t.Fatalf("first PUT = %d: %s", rec.Code, rec.Body)
	}

	if code := conflictCode(t, do(t, h, http.MethodPut, "/api/config", req)); code != "stale" {
		t.Errorf("conflict = %q, want stale", code)
	}
}

func TestPutConfigRejectsInvalidAndMalformedBodies(t *testing.T) {
	h, _ := testServer(t)
	resp := getConfig(t, h)
	resp.Document.Field.FieldLength = 0

	if rec := do(t, h, http.MethodPut, "/api/config", putConfigRequest{Revision: resp.State.Revision, Document: resp.Document}); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid PUT = %d, want 400", rec.Code)
	}

	if rec := do(t, h, http.MethodPut, "/api/config", "{nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed PUT = %d, want 400", rec.Code)
	}
}

func TestSaveWritesTheWorkingDocument(t *testing.T) {
	h, path := testServer(t)
	resp := getConfig(t, h)
	resp.Document.Field.FieldWidth = 5000

	var put revisionResponse
	rec := do(t, h, http.MethodPut, "/api/config", putConfigRequest{Revision: resp.State.Revision, Document: resp.Document})
	if err := json.NewDecoder(rec.Body).Decode(&put); err != nil {
		t.Fatalf("decode PUT: %v", err)
	}

	if rec := do(t, h, http.MethodPost, "/api/config/save", saveRequest{Revision: put.Revision}); rec.Code != http.StatusNoContent {
		t.Fatalf("save = %d: %s", rec.Code, rec.Body)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "field_width: 5000") {
		t.Errorf("saved file missing the edit:\n%s", data)
	}

	if changes := getConfig(t, h).State.Changes; len(changes) != 0 {
		t.Errorf("changes after save = %+v, want none", changes)
	}
}

func TestSaveRefusesAnExternalEditUnlessForced(t *testing.T) {
	h, path := testServer(t)
	rev := getConfig(t, h).State.Revision

	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.Replace(data, []byte("tz: 4000"), []byte("tz: 4100"), 1), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if code := conflictCode(t, do(t, h, http.MethodPost, "/api/config/save", saveRequest{Revision: rev})); code != "disk_changed" {
		t.Errorf("conflict = %q, want disk_changed", code)
	}

	if ext := getConfig(t, h).State.External; ext == nil || len(ext.Changes) != 1 {
		t.Errorf("external = %+v, want the tz edit reported", ext)
	}

	if rec := do(t, h, http.MethodPost, "/api/config/save", saveRequest{Revision: rev, Force: true}); rec.Code != http.StatusNoContent {
		t.Fatalf("forced save = %d: %s", rec.Code, rec.Body)
	}
}

func TestReloadAcceptsTheFileOnDisk(t *testing.T) {
	h, path := testServer(t)

	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.Replace(data, []byte("field_width: 6000"), []byte("field_width: 6200"), 1), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if rec := do(t, h, http.MethodPost, "/api/config/reload", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("reload = %d: %s", rec.Code, rec.Body)
	}

	if got := getConfig(t, h).Document.Field.FieldWidth; got != 6200 {
		t.Errorf("field_width = %d, want 6200 from disk", got)
	}
}

func TestLoadOfAMissingFileReturns400(t *testing.T) {
	h, _ := testServer(t)

	if rec := do(t, h, http.MethodPost, "/api/config/load", loadRequest{Path: "/nonexistent/vision.yml"}); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestSaveAsRejectsAPresetTarget(t *testing.T) {
	h, path := testServer(t)
	rev := getConfig(t, h).State.Revision
	target := filepath.Join(filepath.Dir(path), "geometry-divA.yml")

	if rec := do(t, h, http.MethodPost, "/api/config/save-as", saveAsRequest{Revision: rev, Path: target}); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestLockCalibrationErrors(t *testing.T) {
	h, _ := testServer(t)

	if code := conflictCode(t, do(t, h, http.MethodPost, "/api/config/cameras/1/calibration", nil)); code != "no_live_calibration" {
		t.Errorf("conflict = %q, want no_live_calibration", code)
	}

	if rec := do(t, h, http.MethodPost, "/api/config/cameras/9/calibration", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown camera = %d, want 404", rec.Code)
	}

	if rec := do(t, h, http.MethodPost, "/api/config/cameras/x/calibration", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("bad id = %d, want 400", rec.Code)
	}
}

func TestUnlockCalibrationRemovesItLiveAndFromTheDocument(t *testing.T) {
	h, _ := testServer(t)

	if rec := do(t, h, http.MethodDelete, "/api/config/cameras/0/calibration", nil); rec.Code != http.StatusOK {
		t.Fatalf("unlock = %d: %s", rec.Code, rec.Body)
	}

	resp := getConfig(t, h)
	if resp.Document.Cameras[0].Calibration != nil {
		t.Error("document still holds camera 0's calibration")
	}

	if geo := do(t, h, http.MethodGet, "/api/geometry", nil).Body.String(); strings.Contains(geo, "pixelImageWidth") {
		t.Errorf("live geometry still publishes the calibration: %s", geo)
	}

	var sawCalibrationChange bool
	for _, c := range resp.State.Changes {
		sawCalibrationChange = sawCalibrationChange || c.Path == "cameras[0].calibration" && c.Section == config.SectionGeometry
	}

	if !sawCalibrationChange {
		t.Errorf("changes = %+v, want the removed calibration listed for saving", resp.State.Changes)
	}
}

func TestGetFieldPresetsDegradesGracefullyWhenFilesAreMissing(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodGet, "/api/geometry/presets", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("body = %q, want an empty array (no presets reachable from this cwd)", rec.Body.String())
	}
}

func TestHealthRejectsNonGET(t *testing.T) {
	h, _ := testServer(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := do(t, h, method, "/api/health", nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s /api/health = %d, want %d", method, rec.Code, http.StatusNotFound)
		}
	}
}

// An unrouted API path must not be answered by the SPA fallback. Returning
// index.html here would give fetch() HTML where it expects JSON, surfacing as a
// parse error rather than as the 404 it really is.
func TestUnknownAPIPathReturns404(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodGet, "/api/does-not-exist", nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	if strings.Contains(rec.Body.String(), "doctype") {
		t.Errorf("API 404 answered with HTML: %q", rec.Body.String())
	}
}

// The counterpart: a path that looks like a client-side route does get the app,
// so a deep link survives a reload.
func TestUnknownPathServesApp(t *testing.T) {
	h, _ := testServer(t)

	if rec := do(t, h, http.MethodGet, "/instances/cam0/calibration", nil); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
