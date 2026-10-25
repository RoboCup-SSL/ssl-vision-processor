package frontend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// An in-memory stand-in for dist/, so these tests do not depend on whether
// anyone has run `make frontend`.
var testDist = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html>app")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

func TestServeSPAStatus(t *testing.T) {
	cases := map[string]int{
		"/": http.StatusOK,
		// FileServer canonicalises an explicit /index.html to ./
		"/index.html":                 http.StatusMovedPermanently,
		"/assets/app.js":              http.StatusOK,
		"/instances/cam0/calibration": http.StatusOK,       // a route: falls back
		"/assets/missing.js":          http.StatusNotFound, // an asset: must 404
	}

	handler := serveSPA(testDist, http.FileServerFS(testDist))

	for path, want := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
}

// A 200 alone would not prove the fallback works: the body has to be the app.
func TestServeSPAFallbackServesIndex(t *testing.T) {
	handler := serveSPA(testDist, http.FileServerFS(testDist))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/instances/cam0/calibration", nil))

	if got := rec.Body.String(); got != "<!doctype html>app" {
		t.Errorf("body = %q, want the index.html contents", got)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

// The asset guard is what keeps a stale bundle reference from being answered
// with HTML, which surfaces in the browser as "Unexpected token '<'".
func TestServeSPADoesNotFallBackForAssets(t *testing.T) {
	handler := serveSPA(testDist, http.FileServerFS(testDist))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))

	if strings.Contains(rec.Body.String(), "doctype") {
		t.Errorf("missing asset answered with HTML: %q", rec.Body.String())
	}
}
