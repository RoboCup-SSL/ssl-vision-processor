package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// The spec says which values can be null and which strings are enums; real
// responses must agree, or the generated frontend types would lie.
func TestResponsesMatchTheSpec(t *testing.T) {
	h, _ := testServer(t)

	for path, target := range map[string]string{
		"/api/health":                        "/api/health",
		"/api/topics":                        "/api/topics",
		"/api/network":                       "/api/network",
		"/api/alerts":                        "/api/alerts",
		"/api/schema":                        "/api/schema",
		"/api/config":                        "/api/config",
		"/api/config/files":                  "/api/config/files",
		"/api/config/cameras/{id}/effective": "/api/config/cameras/0/effective",
		"/api/geometry/presets":              "/api/geometry/presets",
		"/api/camera/devices":                "/api/camera/devices",
		"/api/snapshots":                     "/api/snapshots",
	} {
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
