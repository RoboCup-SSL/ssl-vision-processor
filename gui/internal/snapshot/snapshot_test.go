package snapshot

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func writeFile(t *testing.T, dir, name string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestListReturnsEmptyForMissingDir(t *testing.T) {
	entries, err := List(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("entries = %v, want empty", entries)
	}
}

func TestListFindsMatchingFilesAndSkipsOthers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.raw.jpg")
	writeFile(t, dir, "1.calib.png")
	writeFile(t, dir, "not-a-snapshot.txt")
	writeFile(t, dir, "0.calib.json") // legacy / non-image extension

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []Entry{{CameraID: 0, View: "raw"}, {CameraID: 1, View: "calib"}}
	if len(entries) != len(want) {
		t.Fatalf("entries = %v, want %v", entries, want)
	}

	for i, e := range want {
		if entries[i] != e {
			t.Errorf("entries[%d] = %v, want %v", i, entries[i], e)
		}
	}
}

// GeomModel.cpp writes debug overlays as "<camId>.pixels.corner.png" and
// "<camId>.pixels.refined.png" -- view itself contains a dot.
func TestListHandlesADottedViewName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.pixels.corner.png")

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := Entry{CameraID: 0, View: "pixels.corner"}
	if len(entries) != 1 || entries[0] != want {
		t.Fatalf("entries = %v, want [%v]", entries, want)
	}
}

func TestListDedupesSameCameraAndView(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.raw.jpg")
	writeFile(t, dir, "0.raw.png")

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("entries = %v, want a single deduped entry", entries)
	}
}

func TestListReadsEachCameraFolder(t *testing.T) {
	dir := t.TempDir()
	for _, cam := range []string{"cam0", "cam1", "other"} {
		if err := os.Mkdir(filepath.Join(dir, cam), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(t, filepath.Join(dir, "cam0"), "0.raw.jpg")
	writeFile(t, filepath.Join(dir, "cam1"), "1.lines.png")
	writeFile(t, filepath.Join(dir, "cam1"), "spinnaker.calib.json")
	writeFile(t, filepath.Join(dir, "other"), "2.raw.jpg") // not a camera's folder
	writeFile(t, dir, "3.raw.jpg")

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []Entry{{CameraID: 0, View: "raw"}, {CameraID: 1, View: "lines"}, {CameraID: 3, View: "raw"}}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
}

func snapshotMux(dir string) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/snapshot/{camID}/{view}", HandleGet(dir))

	return mux
}

func TestHandleGetServesTheFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.raw.jpg")

	rec := httptest.NewRecorder()
	snapshotMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot/0/raw", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "0.raw.jpg" {
		t.Errorf("body = %q, want file contents", rec.Body.String())
	}
}

func TestHandleGetServesTheMostRecentlyModifiedMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.raw.jpg")

	oldPath := filepath.Join(dir, "0.raw.jpg")
	if err := os.Chtimes(oldPath, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	writeFile(t, dir, "0.raw.png") // written just now, newer

	rec := httptest.NewRecorder()
	snapshotMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot/0/raw", nil))

	if rec.Body.String() != "0.raw.png" {
		t.Errorf("body = %q, want the newer file", rec.Body.String())
	}
}

func TestHandleGetServesADottedViewName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.pixels.corner.png")

	rec := httptest.NewRecorder()
	snapshotMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot/0/pixels.corner", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "0.pixels.corner.png" {
		t.Errorf("body = %q, want file contents", rec.Body.String())
	}
}

func TestHandleGetReturns404ForNoMatch(t *testing.T) {
	dir := t.TempDir()

	rec := httptest.NewRecorder()
	snapshotMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot/9/raw", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// A view name containing glob metacharacters must not turn into a directory
// listing, and must not escape dir via a crafted segment.
func TestHandleGetRejectsUnsafeSegments(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0.raw.jpg")

	for _, view := range []string{"*", "..", "raw/../../etc"} {
		req := httptest.NewRequest(http.MethodGet, "/api/snapshot/0/"+view, nil)
		rec := httptest.NewRecorder()
		snapshotMux(dir).ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			t.Errorf("view=%q: status = 200, want rejected", view)
		}
	}
}

func TestHandleGetServesFromTheCameraFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "cam1"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "1.raw.jpg")

	oldPath := filepath.Join(dir, "1.raw.jpg")
	if err := os.Chtimes(oldPath, time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	writeFile(t, filepath.Join(dir, "cam1"), "1.raw.png") // newer, in the camera's folder

	rec := httptest.NewRecorder()
	snapshotMux(dir).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/snapshot/1/raw", nil))

	if rec.Body.String() != "1.raw.png" {
		t.Errorf("body = %q, want the camera folder's newer file", rec.Body.String())
	}
}
