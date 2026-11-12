package httpjson

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestStatusOfEveryCodeAndAnUnknownOne(t *testing.T) {
	for code, want := range statuses {
		if got := StatusOf(code); got != want {
			t.Errorf("StatusOf(%q) = %d, want %d", code, got, want)
		}
	}

	if got := StatusOf("no_such_code"); got != http.StatusInternalServerError {
		t.Errorf("StatusOf(unknown) = %d, want 500", got)
	}
}

// The enum tag is what the spec, and so the frontend's generated types, offer
// as error codes; it has to list exactly the codes the host can answer with.
func TestErrorBodyEnumMatchesTheCodes(t *testing.T) {
	field, _ := reflect.TypeFor[ErrorBody]().FieldByName("Error")
	enum := strings.Split(field.Tag.Get("enum"), ",")

	codes := make([]string, 0, len(statuses))
	for code := range statuses {
		codes = append(codes, code)
	}

	slices.Sort(enum)
	slices.Sort(codes)

	if !slices.Equal(enum, codes) {
		t.Errorf("ErrorBody enum = %v, codes with a status = %v", enum, codes)
	}
}

func TestErrorfFormatsAProblem(t *testing.T) {
	p := Errorf(CodeInvalid, "cameras[%d]: %s", 2, "missing")

	if p.Code != CodeInvalid || p.Message != "cameras[2]: missing" || p.Error() != p.Message {
		t.Errorf("Errorf = %+v", p)
	}
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}

	return body
}

func TestErrorWritesTheEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, http.StatusBadRequest, CodeInvalid, "rejected", "a: bad", "b: bad")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d", rec.Code)
	}

	body := decodeBody(t, rec)
	if body["error"] != CodeInvalid || body["message"] != "rejected" {
		t.Errorf("body = %v", body)
	}

	if issues, _ := body["issues"].([]any); len(issues) != 2 {
		t.Errorf("issues = %v", body["issues"])
	}
}

func TestErrorWithoutIssuesOmitsThem(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, http.StatusNotFound, CodeNotFound, "gone")

	if _, ok := decodeBody(t, rec)["issues"]; ok {
		t.Errorf("issues present: %s", rec.Body.String())
	}
}

func TestNotFoundNamesTheRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/nope", nil))

	body := decodeBody(t, rec)
	if rec.Code != http.StatusNotFound || body["error"] != CodeNotFound || body["message"] != "no such endpoint: DELETE /api/nope" {
		t.Errorf("%d %v", rec.Code, body)
	}
}

type target struct {
	Name string `json:"name"`
}

func decode(contentType, body string, types ...string) (*httptest.ResponseRecorder, target, bool) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}

	rec := httptest.NewRecorder()

	var v target
	ok := Decode(rec, r, &v, types...)

	return rec, v, ok
}

func TestDecodeAcceptsJSONWithParameters(t *testing.T) {
	rec, v, ok := decode("application/json; charset=utf-8", `{"name":"cam0"}`)

	if !ok || v.Name != "cam0" || rec.Body.Len() != 0 {
		t.Errorf("ok=%v v=%+v body=%q", ok, v, rec.Body.String())
	}
}

func TestDecodeRejects(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		types                   []string
		status                  int
		code                    string
	}{
		{"no content type", "", `{"name":"x"}`, nil, http.StatusUnsupportedMediaType, CodeUnsupportedMedia},
		{"form", "application/x-www-form-urlencoded", `name=x`, nil, http.StatusUnsupportedMediaType, CodeUnsupportedMedia},
		{"not one of the given types", "application/json", `{}`, []string{"application/merge-patch+json"}, http.StatusUnsupportedMediaType, CodeUnsupportedMedia},
		{"malformed", "application/json", `{"name":`, nil, http.StatusBadRequest, CodeBadRequest},
		{"unknown field", "application/json", `{"nmae":"x"}`, nil, http.StatusBadRequest, CodeBadRequest},
		{"two values", "application/json", `{"name":"a"} {"name":"b"}`, nil, http.StatusBadRequest, CodeBadRequest},
		{"empty", "application/json", ``, nil, http.StatusBadRequest, CodeBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, _, ok := decode(tc.contentType, tc.body, tc.types...)
			if ok {
				t.Fatal("accepted")
			}

			if body := decodeBody(t, rec); rec.Code != tc.status || body["error"] != tc.code {
				t.Errorf("%d %v, want %d %s", rec.Code, body, tc.status, tc.code)
			}
		})
	}
}

func TestDecodeAcceptsAGivenType(t *testing.T) {
	_, v, ok := decode("application/merge-patch+json", `{"name":"x"}`, "application/merge-patch+json")

	if !ok || v.Name != "x" {
		t.Errorf("ok=%v v=%+v", ok, v)
	}
}
