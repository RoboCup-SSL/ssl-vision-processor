package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
)

// testServer loads a *copy* of the fixture, never testdata/geometry.yml
// directly -- the /api/geometry/field PUT handler writes back to whatever
// path Geometry was loaded from, and would otherwise rewrite a tracked file
// on every test run.
func testServer(t *testing.T) http.Handler {
	t.Helper()

	src, err := os.ReadFile("testdata/geometry.yml")
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "geometry.yml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	geom, err := geometry.New(path)
	if err != nil {
		t.Fatalf("geometry.New: %v", err)
	}

	return NewVisionServer(geom, hub.New(), t.TempDir(), filepath.Join(t.TempDir(), "config.yml"))
}

// testServerWithConfig is testServer plus a real, writable copy of
// testdata/config.yml -- for the line-corners tests, which need to read back
// what was written.
func testServerWithConfig(t *testing.T) (http.Handler, string) {
	t.Helper()

	geomSrc, err := os.ReadFile("testdata/geometry.yml")
	if err != nil {
		t.Fatalf("ReadFile geometry fixture: %v", err)
	}

	geomPath := filepath.Join(t.TempDir(), "geometry.yml")
	if err := os.WriteFile(geomPath, geomSrc, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	geom, err := geometry.New(geomPath)
	if err != nil {
		t.Fatalf("geometry.New: %v", err)
	}

	configSrc, err := os.ReadFile("testdata/config.yml")
	if err != nil {
		t.Fatalf("ReadFile config fixture: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, configSrc, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return NewVisionServer(geom, hub.New(), t.TempDir(), configPath), configPath
}

func TestHealthReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

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

func TestGeometryReturnsProtojson(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/geometry", nil)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	if !strings.Contains(rec.Body.String(), `"fieldLength"`) {
		t.Errorf("body = %q, want it to contain fieldLength", rec.Body.String())
	}
}

func TestGetFieldConfigReturnsCurrentValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/geometry/field", nil)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !strings.Contains(rec.Body.String(), `"fieldLength"`) || !strings.Contains(rec.Body.String(), `"optionalFieldLines"`) {
		t.Errorf("body = %q, want fieldLength and optionalFieldLines", rec.Body.String())
	}
}

func TestPutFieldConfigUpdatesAndPersists(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{
		"field": {"fieldLength": 2160, "fieldWidth": 1680, "goalWidth": 280, "goalDepth": 50, "boundaryWidth": 100, "lineThickness": 10},
		"optionalFieldLines": {"halfway": true, "penalty": true}
	}`)

	putReq := httptest.NewRequest(http.MethodPut, "/api/geometry/field", body)
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want %d, body: %s", putRec.Code, http.StatusNoContent, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/geometry/field", nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	if !strings.Contains(getRec.Body.String(), `"fieldLength":2160`) {
		t.Errorf("after PUT, GET returned %q, want it to reflect the update", getRec.Body.String())
	}
}

// A zero/negative dimension is the caller's mistake -- 400, not a 500, and
// must not silently write anything to disk.
func TestPutFieldConfigRejectsInvalidDimensions(t *testing.T) {
	srv := testServer(t)

	body := strings.NewReader(`{"field": {"fieldLength": 0}, "optionalFieldLines": {}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/geometry/field", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPutFieldConfigRejectsMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/geometry/field", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// The preset endpoint reads the real repo-root files -- this test runs from
// cmd/ssl-vision-processor-gui, so it doesn't see them and both are skipped.
// It's still worth asserting the endpoint degrades to an empty list rather
// than failing outright when a preset can't be read.
func TestGetFieldPresetsDegradesGracefullyWhenFilesAreMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/geometry/presets", nil)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("body = %q, want an empty array (no presets reachable from this cwd)", rec.Body.String())
	}
}

func TestSaveAsWritesANewFileAndGetReflectsIt(t *testing.T) {
	srv := testServer(t)

	newPath := filepath.Join(t.TempDir(), "renamed.yml")
	body := strings.NewReader(`{
		"path": "` + newPath + `",
		"field": {"fieldLength": 2160, "fieldWidth": 1680, "goalWidth": 280, "goalDepth": 50, "boundaryWidth": 100, "lineThickness": 10},
		"optionalFieldLines": {"halfway": true, "penalty": true}
	}`)

	postReq := httptest.NewRequest(http.MethodPost, "/api/geometry/field/save-as", body)
	postRec := httptest.NewRecorder()
	srv.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusNoContent {
		t.Fatalf("save-as status = %d, want %d, body: %s", postRec.Code, http.StatusNoContent, postRec.Body.String())
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("Stat(newPath): %v", err)
	}

	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/geometry/field", nil))

	if !strings.Contains(getRec.Body.String(), `"fieldLength":2160`) {
		t.Errorf("GET after save-as returned %q, want it to reflect the new values", getRec.Body.String())
	}

	if !strings.Contains(getRec.Body.String(), newPath) {
		t.Errorf("GET after save-as path = %q, want it to contain %q", getRec.Body.String(), newPath)
	}
}

func TestSaveAsRejectsAPresetTarget(t *testing.T) {
	body := strings.NewReader(`{
		"path": "geometry-divB.yml",
		"field": {"fieldLength": 2160, "fieldWidth": 1680, "goalWidth": 280, "goalDepth": 50, "boundaryWidth": 100},
		"optionalFieldLines": {}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/geometry/field/save-as", body)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoadReplacesTheActiveFieldAndReturnsIt(t *testing.T) {
	otherPath := filepath.Join(t.TempDir(), "other.yml")
	if err := os.WriteFile(otherPath, []byte(
		"optional_field_lines:\n  goal2goal: false\n  halfway: false\n  centercircle: false\n  penalty: false\n"+
			"field:\n  field_length: 3000\n  field_width: 2000\n  goal_width: 300\n  goal_depth: 60\n  boundary_width: 120\n",
	), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/geometry/field/load", strings.NewReader(`{"path":"`+otherPath+`"}`))
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"fieldLength":3000`) {
		t.Errorf("body = %q, want it to reflect the loaded file", rec.Body.String())
	}
}

func TestLoadOfAMissingFileReturns400(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/geometry/field/load", strings.NewReader(`{"path":"does-not-exist.yml"}`))
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetLineCornersReturnsWhatWasWritten(t *testing.T) {
	srv, _ := testServerWithConfig(t)

	putBody := strings.NewReader(`{"corners": [{"x":1,"y":2},{"x":3,"y":4},{"x":5,"y":6},{"x":7,"y":8}], "goalSideMarker": 2}`)
	putReq := httptest.NewRequest(http.MethodPut, "/api/config/line-corners", putBody)
	putRec := httptest.NewRecorder()
	srv.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusNoContent {
		t.Fatalf("PUT status = %d, want %d, body: %s", putRec.Code, http.StatusNoContent, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/config/line-corners", nil)
	getRec := httptest.NewRecorder()
	srv.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d, body: %s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	body := getRec.Body.String()
	if !strings.Contains(body, `"x":1`) || !strings.Contains(body, `"goalSideMarker":2`) {
		t.Errorf("GET body = %q, want it to reflect the PUT", body)
	}
}

func TestGetLineCornersDegradesGracefullyWithNothingSavedYet(t *testing.T) {
	// testServer's configFile path doesn't exist at all.
	srv := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/config/line-corners", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"goalSideMarker":0`) {
		t.Errorf("body = %q, want a zero-value response", rec.Body.String())
	}
}

func TestPutLineCornersWritesToConfigFile(t *testing.T) {
	srv, configPath := testServerWithConfig(t)

	body := strings.NewReader(`{"corners": [{"x":1,"y":2},{"x":3,"y":4},{"x":5,"y":6},{"x":7,"y":8}], "goalSideMarker": 3}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config/line-corners", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if !strings.Contains(string(got), "- [1, 2]") {
		t.Errorf("config.yml was not updated: %s", got)
	}

	if !strings.Contains(string(got), "goal_side_marker: 3") {
		t.Errorf("goal_side_marker was not recorded: %s", got)
	}
}

func TestPutLineCornersRejectsAnOutOfRangeGoalSideMarker(t *testing.T) {
	srv, _ := testServerWithConfig(t)

	body := strings.NewReader(`{"corners": [{"x":1,"y":2},{"x":3,"y":4},{"x":5,"y":6},{"x":7,"y":8}], "goalSideMarker": 5}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config/line-corners", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPutLineCornersRejectsTheWrongCornerCount(t *testing.T) {
	srv, _ := testServerWithConfig(t)

	body := strings.NewReader(`{"corners": [{"x":1,"y":2}]}`)
	req := httptest.NewRequest(http.MethodPut, "/api/config/line-corners", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPutLineCornersRejectsMalformedJSON(t *testing.T) {
	srv, _ := testServerWithConfig(t)

	req := httptest.NewRequest(http.MethodPut, "/api/config/line-corners", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// A wrong method is a 404 rather than a 405: the "/" catch-all matches every
// method, so ServeMux never reaches its method-not-allowed path. What matters is
// that the "/api/" subtree answers, instead of the request falling through to
// the frontend and returning HTML with status 200.
func TestHealthRejectsNonGET(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/health", nil)
		rec := httptest.NewRecorder()

		testServer(t).ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("%s /api/health = %d, want %d", method, rec.Code, http.StatusNotFound)
		}
	}
}

// An unrouted API path must not be answered by the SPA fallback. Returning
// index.html here would give fetch() HTML where it expects JSON, surfacing as a
// parse error rather than as the 404 it really is.
func TestUnknownAPIPathReturns404(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

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
	req := httptest.NewRequest(http.MethodGet, "/instances/cam0/calibration", nil)
	rec := httptest.NewRecorder()

	testServer(t).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
