package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDocsViewerIsServed(t *testing.T) {
	h, _ := testServer(t)

	for path, contentType := range map[string]string{
		"/api/docs":                     "text/html; charset=utf-8",
		"/api/docs/redoc.standalone.js": "text/javascript",
	} {
		rec := do(t, h, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != contentType || rec.Body.Len() == 0 {
			t.Errorf("GET %s = %d %q (%d bytes)", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
	}
}

// The schema library drops a response body it thinks is empty (a bare
// {"type": "object"}), so every documented 200 must still have one.
func TestEveryOKResponseHasABody(t *testing.T) {
	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]any
			}
		}
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	for path, ops := range spec.Paths {
		for method, op := range ops {
			if ok, has := op.Responses["200"]; has && len(ok.Content) == 0 {
				t.Errorf("%s %s: 200 has no body in the spec", method, path)
			}
		}
	}
}
