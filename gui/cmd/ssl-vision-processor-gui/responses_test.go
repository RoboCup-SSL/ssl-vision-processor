package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// checkedGETs is each JSON GET route's spec path and a request for it that
// answers 200 from testServer.
var checkedGETs = map[string]string{
	"/api/health":                        "/api/health",
	"/api/topics":                        "/api/topics",
	"/api/network":                       "/api/network",
	"/api/alerts":                        "/api/alerts",
	"/api/schema":                        "/api/schema",
	"/api/config":                        "/api/config",
	"/api/config/files":                  "/api/config/files",
	"/api/config/cameras/{id}/effective": "/api/config/cameras/0/effective",
	"/api/geometry":                      "/api/geometry",
	"/api/geometry/presets":              "/api/geometry/presets",
	"/api/live-field":                    "/api/live-field",
	"/api/capture-devices":               "/api/capture-devices",
	"/api/snapshots":                     "/api/snapshots",
	"/api/processes":                     "/api/processes",
	"/api/instances":                     "/api/instances",
	"/api/system":                        "/api/system",
	"/api/logs":                          "/api/logs",
}

// notJSON are the GET routes whose 200 isn't a JSON body.
var notJSON = []string{"/api/snapshots/{id}/{view}", "/ws", "/ws/video/{id}"}

// A new GET route has to be added to checkedGETs (or notJSON), so its
// responses get checked against the spec too.
func TestEveryGETRouteIsChecked(t *testing.T) {
	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Paths map[string]map[string]json.RawMessage
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	for path, ops := range spec.Paths {
		if _, ok := ops["get"]; !ok {
			continue
		}

		if _, ok := checkedGETs[path]; !ok && !slices.Contains(notJSON, path) {
			t.Errorf("GET %s isn't in checkedGETs: add a request for it", path)
		}
	}

	for path := range checkedGETs {
		if _, ok := spec.Paths[path]["get"]; !ok {
			t.Errorf("checkedGETs has %s, which the spec has no GET for", path)
		}
	}
}

// The spec says which values can be null and which strings are enums; real
// responses must agree, or the generated frontend types would lie.
func TestResponsesMatchTheSpec(t *testing.T) {
	h, _ := testServer(t)

	for path, target := range checkedGETs {
		rec := do(t, h, http.MethodGet, target, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", target, rec.Code)

			continue
		}

		checkResponse(t, path, rec.Body.Bytes())
	}
}

// checkResponse checks body, a 200 from GET path, against the spec.
func checkResponse(t *testing.T, path string, body []byte) {
	t.Helper()

	checkResponseFor(t, http.MethodGet, path, http.StatusOK, body)
}

// checkResponseFor checks body, a status answer to method path, against the
// spec: the spec must list that status, and the body must match its schema.
func checkResponseFor(t *testing.T, method, path string, status int, body []byte) {
	t.Helper()

	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Content map[string]struct{ Schema map[string]any }
			}
		}
		Components struct{ Schemas map[string]map[string]any }
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	response, ok := spec.Paths[path][strings.ToLower(method)].Responses[strconv.Itoa(status)]
	if !ok {
		t.Errorf("%s %s answered %d, which the spec doesn't list", method, path, status)

		return
	}

	at := fmt.Sprintf("%s %s %d", method, path, status)
	for _, problem := range conforms(at, value, response.Content["application/json"].Schema, spec.Components.Schemas) {
		t.Error(problem)
	}
}

// Write routes, and the error body every route shares, against the spec:
// each request in turn on one host, so one's result sets up the next.
func TestWriteResponsesMatchTheSpec(t *testing.T) {
	h, _ := testServer(t)

	current := func() (int64, map[string]any) {
		var got struct {
			Document map[string]any `json:"document"`
			State    struct {
				Revision int64 `json:"revision"`
			} `json:"state"`
		}
		if err := json.Unmarshal(do(t, h, http.MethodGet, "/api/config", nil).Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}

		return got.State.Revision, got.Document
	}

	seen := map[string]bool{}
	check := func(method, path string, rec *httptest.ResponseRecorder, want ...int) {
		t.Helper()

		seen[method+" "+path] = true

		if !slices.Contains(want, rec.Code) {
			t.Errorf("%s %s = %d %s, want one of %v", method, path, rec.Code, rec.Body, want)
		}

		checkResponseFor(t, method, path, rec.Code, rec.Body.Bytes())
	}

	rev, doc := current()
	check("PUT", "/api/config", do(t, h, http.MethodPut, "/api/config", map[string]any{"revision": rev, "document": doc}), http.StatusOK)
	check("PUT", "/api/config", do(t, h, http.MethodPut, "/api/config", map[string]any{"revision": rev, "document": doc}), http.StatusConflict)
	check("PUT", "/api/config", do(t, h, http.MethodPut, "/api/config", "{not json"), http.StatusBadRequest)

	rev, _ = current()
	check("PATCH", "/api/config", patchConfig(t, h, strconv.Quote(strconv.FormatInt(rev, 10)), `{"field":{"fieldWidth":5000}}`), http.StatusOK)
	check("PATCH", "/api/config", patchConfig(t, h, "", `{}`), http.StatusPreconditionRequired)

	const calibration = "/api/config/cameras/{id}/calibration"
	check("POST", calibration, do(t, h, http.MethodPost, "/api/config/cameras/0/calibration", nil), http.StatusOK, http.StatusConflict)
	check("DELETE", calibration, do(t, h, http.MethodDelete, "/api/config/cameras/0/calibration", nil), http.StatusOK)
	check("DELETE", calibration, do(t, h, http.MethodDelete, "/api/config/cameras/99/calibration", nil), http.StatusNotFound)

	rev, _ = current()
	check("POST", "/api/config/save", do(t, h, http.MethodPost, "/api/config/save", map[string]any{"revision": rev, "force": true}), http.StatusOK)
	check("POST", "/api/config/save-as", do(t, h, http.MethodPost, "/api/config/save-as", map[string]any{"revision": rev, "name": "other.yml"}), http.StatusOK)
	check("POST", "/api/config/load", do(t, h, http.MethodPost, "/api/config/load", map[string]any{"name": "other.yml"}), http.StatusOK)
	check("POST", "/api/config/load", do(t, h, http.MethodPost, "/api/config/load", map[string]any{"name": "../outside.yml"}), http.StatusBadRequest)
	check("POST", "/api/config/reload", do(t, h, http.MethodPost, "/api/config/reload", nil), http.StatusOK)

	check("POST", "/api/processes/{id}/restart", do(t, h, http.MethodPost, "/api/processes/0/restart", nil), http.StatusNotFound)

	// A new write route needs a request above.
	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Paths map[string]map[string]json.RawMessage
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	for path, ops := range spec.Paths {
		for method := range ops {
			if method == "get" {
				continue
			}

			if key := strings.ToUpper(method) + " " + path; !seen[key] {
				t.Errorf("%s isn't exercised by TestWriteResponsesMatchTheSpec", key)
			}
		}
	}
}

// conforms lists where value breaks schema's nullability, enums or required
// fields. It only
// checks what the generated types rely on.
func conforms(at string, value any, schema map[string]any, defs map[string]map[string]any) []string {
	if ref, ok := schema["$ref"].(string); ok {
		return conforms(at, value, defs[strings.TrimPrefix(ref, "#/components/schemas/")], defs)
	}

	if _, ok := schema["oneOf"]; ok {
		return nil
	}

	types := schemaTypes(schema)

	if value == nil {
		if len(types) == 0 || slices.Contains(types, "null") {
			return nil
		}

		return []string{fmt.Sprintf("%s: null, but the spec says %v", at, types)}
	}

	if enum, ok := schema["enum"].([]any); ok && !slices.ContainsFunc(enum, func(e any) bool { return fmt.Sprint(e) == fmt.Sprint(value) }) {
		return []string{fmt.Sprintf("%s: %v isn't one of %v", at, value, enum)}
	}

	var problems []string

	switch v := value.(type) {
	case map[string]any:
		props, _ := schema["properties"].(map[string]any)
		extra, _ := schema["additionalProperties"].(map[string]any)

		required, _ := schema["required"].([]any)
		for _, r := range required {
			if _, ok := v[fmt.Sprint(r)]; !ok {
				problems = append(problems, fmt.Sprintf("%s: missing required %v", at, r))
			}
		}

		for k, item := range v {
			if p, ok := props[k].(map[string]any); ok {
				problems = append(problems, conforms(at+"."+k, item, p, defs)...)
			} else if extra != nil {
				problems = append(problems, conforms(at+"."+k, item, extra, defs)...)
			}
		}
	case []any:
		if items, ok := schema["items"].(map[string]any); ok {
			for i, item := range v {
				problems = append(problems, conforms(fmt.Sprintf("%s[%d]", at, i), item, items, defs)...)
			}
		}
	}

	return problems
}

func schemaTypes(schema map[string]any) []string {
	switch t := schema["type"].(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, s := range t {
			out = append(out, fmt.Sprint(s))
		}

		return out
	}

	return nil
}
