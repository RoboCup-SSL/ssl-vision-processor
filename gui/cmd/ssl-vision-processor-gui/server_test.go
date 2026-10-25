package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/origins"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
)

// testServer serves a *copy* of testdata/vision.yml -- saves write back to
// it, and would otherwise rewrite a tracked file on every run.
func testServer(t *testing.T) (http.Handler, string) {
	t.Helper()

	src, err := os.ReadFile(visionFixture)
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

	// httptest requests name the host example.com.
	policy, err := origins.New(origins.Options{TrustedOrigins: []string{"http://trusted.example"}, AllowedHosts: []string{"example.com"}})
	if err != nil {
		t.Fatal(err)
	}

	return NewVisionServer(geom, store, stubStatus{}, policy, hub.New("config.state"), video.NewManager(false), nil, t.TempDir()), path
}

// stubStatus is a host whose sockets have heard nothing yet.
type stubStatus struct{}

func (stubStatus) Network() NetworkState {
	return NetworkState{
		Vision:            multicast.Status{Address: "224.5.23.2:10006", Mode: "multicast"},
		GC:                multicast.Status{Address: "224.5.23.1:10003", Mode: "multicast"},
		Cameras:           []detections.Source{},
		Interfaces:        []multicast.Interface{},
		LoopbackMulticast: true,
	}
}

func (stubStatus) Alerts() []alerts.Alert {
	return []alerts.Alert{{ID: "silent-vision", Code: alerts.CodeSocketSilent, Severity: alerts.Warning}}
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

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return send(h, req)
}

func send(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func errorBody(t *testing.T, rec *httptest.ResponseRecorder) httpjson.ErrorBody {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("error Content-Type = %q, want application/json (body %q)", got, rec.Body)
	}

	var body httpjson.ErrorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	return body
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

	return errorBody(t, rec).Error
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

	rec := do(t, h, http.MethodPut, "/api/config", putConfigRequest{Revision: resp.State.Revision, Document: resp.Document})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid PUT = %d, want 400", rec.Code)
	}

	if body := errorBody(t, rec); body.Error != httpjson.CodeInvalid || len(body.Issues) != 1 || !strings.HasPrefix(body.Issues[0], "field: ") {
		t.Errorf("invalid PUT body = %+v, want code invalid and one field: issue", body)
	}

	rec = do(t, h, http.MethodPut, "/api/config", "{nope")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed PUT = %d, want 400", rec.Code)
	}

	if code := errorBody(t, rec).Error; code != httpjson.CodeBadRequest {
		t.Errorf("malformed PUT code = %q, want bad_request", code)
	}
}

// A misspelled key must fail, not be dropped: dropping it would replace the
// value it named with the zero value.
func TestPutConfigRejectsUnknownFields(t *testing.T) {
	h, _ := testServer(t)
	resp := getConfig(t, h)

	doc, err := json.Marshal(resp.Document)
	if err != nil {
		t.Fatal(err)
	}

	misspelled := strings.Replace(string(doc), `"fieldWidth"`, `"fieldWdith"`, 1)
	body := `{"revision":` + strconv.FormatInt(resp.State.Revision, 10) + `,"document":` + misspelled + `}`

	if rec := do(t, h, http.MethodPut, "/api/config", body); rec.Code != http.StatusBadRequest {
		t.Errorf("PUT with unknown field = %d, want 400", rec.Code)
	}
}

func TestJSONBodiesNeedAJSONContentType(t *testing.T) {
	h, _ := testServer(t)
	rev := getConfig(t, h).State.Revision

	req := httptest.NewRequest(http.MethodPost, "/api/config/save", strings.NewReader(`{"revision":`+strconv.FormatInt(rev, 10)+`}`))
	req.Header.Set("Content-Type", "text/plain")

	rec := send(h, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain save = %d, want 415", rec.Code)
	}

	if code := errorBody(t, rec).Error; code != httpjson.CodeUnsupportedMedia {
		t.Errorf("code = %q, want unsupported_media", code)
	}
}

func TestCrossSiteWritesAreRejected(t *testing.T) {
	h, _ := testServer(t)

	for site, want := range map[string]int{"cross-site": http.StatusForbidden, "same-origin": http.StatusOK} {
		req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
		req.Header.Set("Sec-Fetch-Site", site)

		if rec := send(h, req); rec.Code != want {
			t.Errorf("reload from %s = %d, want %d", site, rec.Code, want)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	req.Header.Set("Origin", "http://evil.example")

	rec := send(h, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reload with a foreign Origin = %d, want 403", rec.Code)
	}

	if code := errorBody(t, rec).Error; code != httpjson.CodeCrossOrigin {
		t.Errorf("code = %q, want cross_origin", code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/config/reload", nil)
	req.Header.Set("Origin", "http://trusted.example")

	if rec := send(h, req); rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "http://trusted.example" {
		t.Errorf("reload from a trusted origin = %d %v, want 204 with CORS", rec.Code, rec.Header())
	}
}

func patchConfig(t *testing.T, h http.Handler, ifMatch, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPatch, "/api/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/merge-patch+json")

	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}

	return send(h, req)
}

func TestPatchConfigMergesIntoTheWorkingDocument(t *testing.T) {
	h, _ := testServer(t)

	etag := do(t, h, http.MethodGet, "/api/config", nil).Header().Get("ETag")
	if etag == "" {
		t.Fatal("GET /api/config: no ETag")
	}

	rec := patchConfig(t, h, etag, `{"field":{"fieldWidth":5000}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body)
	}

	if next := rec.Header().Get("ETag"); next == etag || next == "" {
		t.Errorf("ETag after PATCH = %q, want a new revision after %q", next, etag)
	}

	resp := getConfig(t, h)
	if resp.Document.Field.FieldWidth != 5000 || resp.Document.Field.FieldLength != 9000 || len(resp.Document.Cameras) != 2 {
		t.Errorf("document = %+v, want only fieldWidth changed", resp.Document)
	}

	if code := conflictCode(t, patchConfig(t, h, etag, `{"field":{"fieldWidth":5100}}`)); code != httpjson.CodeStale {
		t.Errorf("PATCH on the old ETag: conflict = %q, want stale", code)
	}
}

func TestPatchConfigErrors(t *testing.T) {
	h, _ := testServer(t)
	etag := do(t, h, http.MethodGet, "/api/config", nil).Header().Get("ETag")

	if rec := patchConfig(t, h, "", `{}`); rec.Code != http.StatusPreconditionRequired {
		t.Errorf("PATCH without If-Match = %d, want 428", rec.Code)
	}

	if rec := patchConfig(t, h, etag, `{"feild":{}}`); rec.Code != http.StatusBadRequest || errorBody(t, rec).Error != httpjson.CodeInvalid {
		t.Errorf("PATCH with an unknown key = %d, want 400 invalid", rec.Code)
	}

	if rec := patchConfig(t, h, etag, `{"field":{"fieldWidth":0}}`); rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH to an invalid document = %d, want 400", rec.Code)
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

	if rec := do(t, h, http.MethodPost, "/api/config/save", saveRequest{Revision: put.Revision}); rec.Code != http.StatusOK {
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

	if rec := do(t, h, http.MethodPost, "/api/config/save", saveRequest{Revision: rev, Force: true}); rec.Code != http.StatusOK {
		t.Fatalf("forced save = %d: %s", rec.Code, rec.Body)
	}
}

func TestReloadAcceptsTheFileOnDisk(t *testing.T) {
	h, path := testServer(t)

	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, bytes.Replace(data, []byte("field_width: 6000"), []byte("field_width: 6200"), 1), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	rec := do(t, h, http.MethodPost, "/api/config/reload", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reload = %d: %s", rec.Code, rec.Body)
	}

	var reloaded revisionResponse
	if err := json.NewDecoder(rec.Body).Decode(&reloaded); err != nil || rec.Header().Get("ETag") != revision(reloaded.Revision).String() {
		t.Errorf("reload body %+v (%v), ETag %q: want the new revision in both", reloaded, err, rec.Header().Get("ETag"))
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

	if code := errorBody(t, rec).Error; code != httpjson.CodeNotFound {
		t.Errorf("code = %q, want not_found", code)
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

func TestGetSchemaDescribesSettings(t *testing.T) {
	h, _ := testServer(t)
	rec := do(t, h, http.MethodGet, "/api/schema", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var schema config.Schema
	if err := json.NewDecoder(rec.Body).Decode(&schema); err != nil {
		t.Fatalf("decode: %v", err)
	}

	blobs := schema.Settings["thresholds.blobs"]
	if blobs.Default != 2000.0 || blobs.Apply != config.ApplyRestart {
		t.Errorf("thresholds.blobs = %+v, want default 2000, restart", blobs)
	}

	if len(schema.Drivers) == 0 || len(schema.CameraCounts) == 0 {
		t.Errorf("schema = %+v, want drivers and camera counts", schema)
	}
}

func TestEffectiveConfig(t *testing.T) {
	h, _ := testServer(t)

	rec := do(t, h, http.MethodGet, "/api/config/cameras/0/effective", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	var eff config.EffectiveConfig
	if err := json.NewDecoder(rec.Body).Decode(&eff); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got := eff.Settings["thresholds.blobs"]; got.Value != 2000.0 || got.Source != config.SourceFallback {
		t.Errorf("thresholds.blobs = %+v, want the 2000 fallback", got)
	}

	if rec := do(t, h, http.MethodGet, "/api/config/cameras/9/effective", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown camera = %d, want 404", rec.Code)
	}
}

func TestStatusEndpoints(t *testing.T) {
	h, _ := testServer(t)

	for _, path := range []string{"/api/network", "/api/alerts", "/api/topics"} {
		rec := do(t, h, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("GET %s = %d %q: %s", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body)
		}
	}

	var got []alerts.Alert
	if err := json.NewDecoder(do(t, h, http.MethodGet, "/api/alerts", nil).Body).Decode(&got); err != nil || len(got) != 1 {
		t.Errorf("alerts = %+v (%v), want the stub's one", got, err)
	}
}

func TestConfigFilesListsTheCurrentFile(t *testing.T) {
	h, path := testServer(t)

	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var files []config.File
	if err := json.NewDecoder(do(t, h, http.MethodGet, "/api/config/files", nil).Body).Decode(&files); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(files) != 1 || files[0].Path != path || !files[0].Current {
		t.Errorf("files = %+v, want just %s as current", files, path)
	}
}

// DNS rebinding: an attacker's domain pointed at this host looks same-origin
// to the browser, so a request naming an unknown host is refused outright.
func TestUnknownHostIsRefused(t *testing.T) {
	h, _ := testServer(t)

	req := httptest.NewRequest(http.MethodGet, "http://evil.example/api/config", nil)

	rec := send(h, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("GET via evil.example = %d, want 421", rec.Code)
	}

	if code := errorBody(t, rec).Error; code != httpjson.CodeUnknownHost {
		t.Errorf("code = %q, want unknown_host", code)
	}

	if rec := send(h, httptest.NewRequest(http.MethodGet, "http://192.168.1.50:8085/api/health", nil)); rec.Code != http.StatusOK {
		t.Errorf("GET by IP = %d, want 200", rec.Code)
	}
}
