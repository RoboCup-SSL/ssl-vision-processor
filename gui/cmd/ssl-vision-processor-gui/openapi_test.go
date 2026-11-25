package main

import (
	"encoding/json"
	"net/http"
	"regexp"
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

// Redoc shows tags in the spec's own order, with their descriptions; a tag
// missing from specTags would be appended undescribed.
func TestEveryTagIsDescribed(t *testing.T) {
	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Tags []struct {
			Name        string
			Description string
		}
		Paths map[string]map[string]struct {
			Tags []string
		}
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	described := map[string]bool{}
	for _, tag := range spec.Tags {
		described[tag.Name] = tag.Description != ""
	}

	for path, ops := range spec.Paths {
		for method, op := range ops {
			for _, tag := range op.Tags {
				if !described[tag] {
					t.Errorf("%s %s: tag %q isn't in specTags", method, path, tag)
				}
			}
		}
	}
}

// The API's naming rules, so a new route or topic keeps to them: kebab-case
// paths, {id} for a camera, verbNoun operationIds, <noun>.state topics.
func TestNamesFollowTheRules(t *testing.T) {
	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string
		}
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	path := regexp.MustCompile(`^(/([a-z]+(-[a-z]+)*|\{[a-z]+\}))+$`)
	id := regexp.MustCompile(`^(get|list|put|patch|lock|unlock|load|reload|save|restart|open)[A-Z][A-Za-z]*$`)
	topic := regexp.MustCompile(`^[a-z]+(-[a-z]+)*\.state$`)
	seen := map[string]string{}

	for p, ops := range spec.Paths {
		if !path.MatchString(p) {
			t.Errorf("path %s isn't kebab-case", p)
		}

		for method, op := range ops {
			at := method + " " + p
			if !id.MatchString(op.OperationID) {
				t.Errorf("%s: operationId %q isn't verbNoun", at, op.OperationID)
			}

			if other, ok := seen[op.OperationID]; ok {
				t.Errorf("%s and %s share operationId %q", at, other, op.OperationID)
			}

			seen[op.OperationID] = at
		}
	}

	h, _ := testHost(t)
	for _, name := range h.hub.Topics() {
		if !topic.MatchString(name) {
			t.Errorf("topic %s isn't <noun>.state", name)
		}
	}
}
