package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
	"github.com/swaggest/jsonschema-go"
)

type version int

func (v *version) UnmarshalText(text []byte) error {
	n, err := strconv.Atoi(strings.Trim(string(text), `"`))
	*v = version(n)

	return err
}

type thingIn struct {
	ID      int            `path:"id"`
	Mode    string         `query:"mode"`
	IfMatch version        `header:"If-Match" required:"true"`
	Body    map[string]any `body:"application/merge-patch+json,application/json"`
}

type thingOut struct {
	ETag string `header:"ETag" json:"-"`
	Got  string `json:"got"`
}

var errTaken = errors.New("taken")

func testAPI(t *testing.T) (*API, *http.ServeMux) {
	t.Helper()

	mux := http.NewServeMux()
	a := New(mux, Info{Title: "test", Version: "1"}, func(w http.ResponseWriter, err error) {
		httpjson.Error(w, http.StatusConflict, httpjson.CodeStale, err.Error())
	})

	Route(a, "PATCH /things/{id}", Doc{Summary: "Patch a thing", Errors: []string{httpjson.CodeStale, httpjson.CodeInvalid}},
		func(_ *http.Request, in thingIn) (thingOut, error) {
			if in.ID == 9 {
				return thingOut{}, errTaken
			}

			return thingOut{ETag: `"7"`, Got: strconv.Itoa(in.ID) + "/" + in.Mode + "/" + strconv.Itoa(int(in.IfMatch)) + "/" + in.Body["k"].(string)}, nil
		})

	return a, mux
}

func call(mux *http.ServeMux, method, target, contentType, ifMatch, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func TestRouteDecodesAndEncodes(t *testing.T) {
	_, mux := testAPI(t)

	rec := call(mux, http.MethodPatch, "/things/3?mode=fast", "application/merge-patch+json", `"5"`, `{"k":"v"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	if got := rec.Header().Get("ETag"); got != `"7"` {
		t.Errorf("ETag = %q, want \"7\" from the output's header field", got)
	}

	var out map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}

	if out["got"] != "3/fast/5/v" || len(out) != 1 {
		t.Errorf("body = %v, want only got = 3/fast/5/v (no ETag in the body)", out)
	}
}

func TestRouteRejectsBadInput(t *testing.T) {
	_, mux := testAPI(t)

	for name, tc := range map[string]struct {
		target, contentType, ifMatch, body string
		status                             int
		code                               string
	}{
		"missing If-Match is 428": {"/things/3", "application/json", "", `{"k":"v"}`, http.StatusPreconditionRequired, httpjson.CodePrecondition},
		"bad If-Match":            {"/things/3", "application/json", `"x"`, `{"k":"v"}`, http.StatusBadRequest, httpjson.CodeBadRequest},
		"non-numeric path":        {"/things/x", "application/json", `"5"`, `{"k":"v"}`, http.StatusBadRequest, httpjson.CodeBadRequest},
		"wrong content type":      {"/things/3", "text/plain", `"5"`, `{"k":"v"}`, http.StatusUnsupportedMediaType, httpjson.CodeUnsupportedMedia},
		"malformed body":          {"/things/3", "application/json", `"5"`, `{nope`, http.StatusBadRequest, httpjson.CodeBadRequest},
		"handler error":           {"/things/9", "application/json", `"5"`, `{"k":"v"}`, http.StatusConflict, httpjson.CodeStale},
	} {
		rec := call(mux, http.MethodPatch, tc.target, tc.contentType, tc.ifMatch, tc.body)

		var body httpjson.ErrorBody
		_ = json.NewDecoder(rec.Body).Decode(&body)

		if rec.Code != tc.status || body.Error != tc.code {
			t.Errorf("%s: %d %q, want %d %q", name, rec.Code, body.Error, tc.status, tc.code)
		}
	}
}

func TestSpecDescribesTheRoute(t *testing.T) {
	a, _ := testAPI(t)

	data, err := a.Spec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Paths map[string]map[string]struct {
			Summary     string
			Parameters  []struct{ Name, In string }
			RequestBody struct {
				Content map[string]any
			}
			Responses map[string]struct {
				Description string
				Headers     map[string]any
			}
		}
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	op, ok := spec.Paths["/things/{id}"]["patch"]
	if !ok {
		t.Fatalf("paths = %v, want PATCH /things/{id}", spec.Paths)
	}

	if op.Summary != "Patch a thing" {
		t.Errorf("summary = %q", op.Summary)
	}

	params := map[string]string{}
	for _, p := range op.Parameters {
		params[p.Name] = p.In
	}

	if params["id"] != "path" || params["mode"] != "query" || params["If-Match"] != "header" || len(params) != 3 {
		t.Errorf("parameters = %v, want id in path, mode in query, If-Match in header", params)
	}

	if _, ok := op.RequestBody.Content["application/merge-patch+json"]; !ok {
		t.Errorf("request body content = %v, want application/merge-patch+json", op.RequestBody.Content)
	}

	if _, ok := op.Responses["200"].Headers["ETag"]; !ok {
		t.Errorf("200 headers = %v, want ETag", op.Responses["200"].Headers)
	}

	for status, want := range map[string]string{"409": "stale", "400": "invalid", "500": "internal"} {
		if !strings.Contains(op.Responses[status].Description, want) {
			t.Errorf("response %s = %q, want it to list %s", status, op.Responses[status].Description, want)
		}
	}
}

type custom map[string]any

func TestSchemaForReplacesAType(t *testing.T) {
	mux := http.NewServeMux()
	a := New(mux, Info{Title: "test", Version: "1"}, nil)

	var s jsonschema.Schema
	s.WithType(jsonschema.Object.Type()).WithDescription("from elsewhere")
	a.SchemaFor(reflect.TypeFor[custom](), s)

	Route(a, "GET /custom", Doc{}, func(*http.Request, struct{}) (custom, error) { return custom{}, nil })

	data, err := a.Spec()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "from elsewhere") {
		t.Errorf("spec doesn't use the replacement schema:\n%s", data)
	}
}

func TestRoutePanicsOnInputItCantDocument(t *testing.T) {
	type twoBodies struct {
		A map[string]any `body:"application/json"`
		B map[string]any `body:"application/json"`
	}

	defer func() {
		if recover() == nil {
			t.Error("Route accepted an input with two bodies")
		}
	}()

	Route(New(http.NewServeMux(), Info{}, nil), "POST /x", Doc{}, func(*http.Request, twoBodies) (struct{}, error) { return struct{}{}, nil })
}
