package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
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
	"/api/live_field":                    "/api/live_field",
	"/api/camera/devices":                "/api/camera/devices",
	"/api/snapshots":                     "/api/snapshots",
	"/api/processes":                     "/api/processes",
	"/api/instances":                     "/api/instances",
	"/api/system":                        "/api/system",
	"/api/logs":                          "/api/logs",
}

// notJSON are the GET routes whose 200 isn't a JSON body.
var notJSON = []string{"/api/snapshot/{camID}/{view}", "/ws", "/ws/video/{id}"}

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
		t.Fatalf("GET %s: %v", path, err)
	}

	schema := spec.Paths[path]["get"].Responses["200"].Content["application/json"].Schema
	for _, problem := range conforms(path, value, schema, spec.Components.Schemas) {
		t.Error(problem)
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
