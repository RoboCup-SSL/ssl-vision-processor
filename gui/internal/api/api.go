// Package api registers the host's HTTP routes with their request and
// response types, and builds the OpenAPI 3.1 spec from those same types.
//
// A handler takes one input struct and returns one output value. Input fields
// tagged path:, query: or header: are parameters; one field tagged
// body:"<content types>" is the request body. Output fields tagged header:
// (and json:"-") are response headers; the rest is the JSON body. Because the
// spec is reflected from the handler's own types, it can't describe types the
// handler doesn't use. Field and type descriptions come from Go doc
// comments, via `go generate` (comments_gen.go).
package api

//go:generate go run ./gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
	"github.com/swaggest/jsonschema-go"
	"github.com/swaggest/openapi-go"
	"github.com/swaggest/openapi-go/openapi31"
)

// API is a ServeMux plus the spec of what's registered on it.
type API struct {
	mux      *http.ServeMux
	r        *openapi31.Reflector
	onError  func(http.ResponseWriter, error)
	declared map[string][]string
}

// Info is the spec's title, version and introduction.
type Info struct {
	Title       string
	Version     string
	Description string
}

// New returns an API registering on mux. onError writes a handler's error as
// an httpjson error body, mapping domain errors to codes.
func New(mux *http.ServeMux, info Info, onError func(http.ResponseWriter, error)) *API {
	r := openapi31.NewReflector()
	r.Spec.Info.WithTitle(info.Title).WithVersion(info.Version).WithDescription(info.Description)
	r.JSONSchemaReflector().DefaultOptions = append(r.JSONSchemaReflector().DefaultOptions,
		jsonschema.InterceptSchema(describe),
		// A slice or map the host sends is [] or {}, never null: Go's nil
		// isn't part of the API (the cmd tests check responses for it).
		jsonschema.InterceptNullability(func(p jsonschema.InterceptNullabilityParams) {
			if p.NullAdded && (p.Type.Kind() == reflect.Slice || p.Type.Kind() == reflect.Map) {
				*p.Schema = p.OrigSchema
			}
		}),
		// Types in package main get their bare name, not the module path's.
		jsonschema.InterceptDefName(func(t reflect.Type, name string) string {
			if canonical(t.PkgPath()) == mainPackage && t.Name() != "" {
				return strings.ToUpper(t.Name()[:1]) + t.Name()[1:]
			}

			return name
		}),
	)

	return &API{mux: mux, r: r, onError: onError, declared: map[string][]string{}}
}

// Doc is a route's documentation. The summary is its first sentence; the
// rest of Description follows it.
type Doc struct {
	Summary     string
	Description string
	Tags        []string
	// Errors are the httpjson codes the route can return, besides internal.
	Errors []string

	id string
}

// withHandlerComment fills an empty Summary and Description from h's Go doc
// comment, so the prose lives in one place: the first sentence is the
// summary, the rest the description.
func (d Doc) withHandlerComment(h any) Doc {
	name := canonical(strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name(), "-fm"))
	d.id = name[strings.LastIndex(name, ".")+1:]

	summary, rest, _ := strings.Cut(comments[name], ". ")

	if d.Summary == "" {
		d.Summary = strings.TrimSuffix(summary, ".")
	}

	if d.Description == "" {
		d.Description = rest
	}

	return d
}

// SchemaFor documents values of type t with schema instead of reflecting
// them, for types whose shape lives elsewhere (config.Settings, from the
// settings schema).
func (a *API) SchemaFor(t reflect.Type, schema jsonschema.Schema) {
	a.r.JSONSchemaReflector().DefaultOptions = append(a.r.JSONSchemaReflector().DefaultOptions,
		jsonschema.InterceptSchema(func(p jsonschema.InterceptSchemaParams) (bool, error) {
			if !p.Value.IsValid() {
				return false, nil
			}

			vt := p.Value.Type()
			for vt.Kind() == reflect.Pointer {
				vt = vt.Elem()
			}

			if vt == t {
				*p.Schema = schema

				return true, nil
			}

			return false, nil
		}),
	)
}

// Route registers h for pattern ("METHOD /path/{param}") and documents it.
// It panics on a pattern or type it can't document, which is a programming
// error found at startup.
func Route[In, Out any](a *API, pattern string, doc Doc, h func(*http.Request, In) (Out, error)) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		panic("api: pattern needs a method: " + pattern)
	}

	doc = doc.withHandlerComment(h)

	in, err := inputOf(reflect.TypeFor[In]())
	if err != nil {
		panic(fmt.Sprintf("api: %s: %v", pattern, err))
	}

	if err := a.document(method, path, doc, in, reflect.TypeFor[Out]()); err != nil {
		panic(fmt.Sprintf("api: %s: %v", pattern, err))
	}

	a.declared[pattern] = doc.Errors

	a.mux.Handle(pattern, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if CheckDeclared != nil {
			sniff := &sniffer{ResponseWriter: w}
			w = sniff

			defer func() { sniff.check(pattern, doc.Errors) }()
		}

		var req In

		if !in.decode(w, r, reflect.ValueOf(&req).Elem()) {
			return
		}

		resp, err := h(r, req)
		if err != nil {
			a.onError(w, err)

			return
		}

		writeOutput(w, resp)
	}))
}

// CheckDeclared, when set (by tests), is called with each error code a
// Route answers that its Doc doesn't declare.
var CheckDeclared func(pattern, code string)

// sniffer keeps an error response's body for CheckDeclared.
type sniffer struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (s *sniffer) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *sniffer) Write(b []byte) (int, error) {
	if s.status >= 400 {
		s.body = append(s.body, b...)
	}

	return s.ResponseWriter.Write(b)
}

func (s *sniffer) check(pattern string, declared []string) {
	var body httpjson.ErrorBody
	if s.status < 400 || json.Unmarshal(s.body, &body) != nil {
		return
	}

	if body.Error != httpjson.CodeInternal && !slices.Contains(declared, body.Error) {
		CheckDeclared(pattern, body.Error)
	}
}

// RawResponse is a response a Raw route writes itself.
type RawResponse struct {
	Status      int
	ContentType string // empty for none
	Description string
}

// Raw registers h for a route whose responses aren't one JSON value (an
// image, a WebSocket upgrade) and documents it: In's parameters as for Route,
// which h reads itself, and responses. Errors are JSON as usual.
func Raw[In any](a *API, pattern string, doc Doc, h http.Handler, responses ...RawResponse) {
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		panic("api: pattern needs a method: " + pattern)
	}

	in, err := inputOf(reflect.TypeFor[In]())
	if err != nil {
		panic(fmt.Sprintf("api: %s: %v", pattern, err))
	}

	if err := a.documentRaw(method, path, doc, in, responses); err != nil {
		panic(fmt.Sprintf("api: %s: %v", pattern, err))
	}

	a.declared[pattern] = doc.Errors
	a.mux.Handle(pattern, h)
}

func (a *API) documentRaw(method, path string, doc Doc, in input, responses []RawResponse) error {
	oc, err := a.r.NewOperationContext(method, path)
	if err != nil {
		return err
	}

	oc.SetSummary(doc.Summary)
	oc.SetDescription(doc.Description)
	oc.SetTags(doc.Tags...)

	if in.params != nil {
		oc.AddReqStructure(reflect.New(in.params).Interface())
	}

	for _, resp := range responses {
		oc.AddRespStructure(nil, openapi.WithHTTPStatus(resp.Status), openapi.WithCustomize(func(cor openapi.ContentOrReference) {
			r, ok := cor.(*openapi31.ResponseOrReference)
			if !ok {
				return
			}

			if r.Response == nil {
				r.Response = &openapi31.Response{}
			}

			r.Response.Description = resp.Description
			if resp.ContentType != "" {
				r.Response.Content = map[string]openapi31.MediaType{
					resp.ContentType: {Schema: map[string]any{"type": "string", "format": "binary"}},
				}
			}
		}))
	}

	for status, codes := range errorsByStatus(doc.Errors) {
		oc.AddRespStructure(new(httpjson.ErrorBody),
			openapi.WithHTTPStatus(status),
			func(cu *openapi.ContentUnit) { cu.Description = "error: " + strings.Join(codes, ", ") },
		)
	}

	return a.r.AddOperation(oc)
}

// Declared is the error codes each route documents, by pattern, for tests
// that check a route only returns those.
func (a *API) Declared() map[string][]string { return a.declared }

// Spec is the OpenAPI document as JSON.
func (a *API) Spec() ([]byte, error) {
	return json.MarshalIndent(a.r.Spec, "", "  ")
}

func (a *API) document(method, path string, doc Doc, in input, out reflect.Type) error {
	oc, err := a.r.NewOperationContext(method, path)
	if err != nil {
		return err
	}

	oc.SetSummary(doc.Summary)
	oc.SetDescription(doc.Description)
	oc.SetID(doc.id)
	oc.SetTags(doc.Tags...)

	if in.params != nil {
		oc.AddReqStructure(reflect.New(in.params).Interface())
	}

	if in.body >= 0 || in.whole {
		types, bodyDoc := in.bodyTypes, in.bodyDoc
		// Reflected as JSON, then filed under the body's own media types.
		oc.AddReqStructure(reflect.New(in.bodyType).Interface(),
			openapi.WithContentType("application/json"),
			openapi.WithCustomize(func(cor openapi.ContentOrReference) {
				rb, ok := cor.(*openapi31.RequestBodyOrReference)
				if !ok || rb.RequestBody == nil {
					return
				}

				media := rb.RequestBody.Content["application/json"]
				notNull(media.Schema)

				rb.RequestBody.Content = map[string]openapi31.MediaType{}

				for _, t := range types {
					rb.RequestBody.Content[t] = media
				}

				rb.RequestBody.WithRequired(true)

				if bodyDoc != "" {
					rb.RequestBody.WithDescription(bodyDoc)
				}
			}),
		)
	}

	oc.AddRespStructure(reflect.New(out).Interface(), openapi.WithHTTPStatus(http.StatusOK),
		openapi.WithCustomize(func(cor openapi.ContentOrReference) {
			if r, ok := cor.(*openapi31.ResponseOrReference); ok && r.Response != nil {
				for _, media := range r.Response.Content {
					notNull(media.Schema)
				}
			}
		}),
	)

	for status, codes := range errorsByStatus(doc.Errors) {
		oc.AddRespStructure(new(httpjson.ErrorBody),
			openapi.WithHTTPStatus(status),
			func(cu *openapi.ContentUnit) { cu.Description = "error: " + strings.Join(codes, ", ") },
		)
	}

	return a.r.AddOperation(oc)
}

// notNull drops "null" from a top-level body schema's types: the host never
// sends or accepts a null body, though Go reflects a slice, map or pointer as
// nullable.
func notNull(schema map[string]any) {
	t, ok := schema["type"].([]any)
	if !ok || !slices.Contains(t, any("null")) {
		return
	}

	t = slices.DeleteFunc(slices.Clone(t), func(v any) bool { return v == "null" })
	if len(t) == 1 {
		schema["type"] = t[0]
	} else {
		schema["type"] = t
	}
}

// errorsByStatus groups codes by the status httpjson answers them with, plus
// internal under 500, which any route can return.
func errorsByStatus(codes []string) map[int][]string {
	out := map[int][]string{http.StatusInternalServerError: {httpjson.CodeInternal}}

	for _, c := range codes {
		s := httpjson.StatusOf(c)
		if !slices.Contains(out[s], c) {
			out[s] = append(out[s], c)
		}
	}

	return out
}

func writeOutput(w http.ResponseWriter, out any) {
	v := reflect.ValueOf(out)

	if v.Kind() == reflect.Struct {
		for i := range v.NumField() {
			if name := v.Type().Field(i).Tag.Get("header"); name != "" {
				w.Header().Set(name, fmt.Sprint(v.Field(i).Interface()))
			}
		}
	}

	httpjson.Write(w, http.StatusOK, out)
}

// describe sets a struct type's and its fields' descriptions from their Go
// doc comments, once the schema has been reflected.
func describe(p jsonschema.InterceptSchemaParams) (bool, error) {
	if !p.Processed || !p.Value.IsValid() {
		return false, nil
	}

	t := p.Value.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct || t.Name() == "" {
		return false, nil
	}

	key := canonical(t.PkgPath()) + "." + t.Name()

	if doc := comments[key]; doc != "" && p.Schema.Description == nil {
		p.Schema.WithDescription(doc)
	}

	for i := range t.NumField() {
		f := t.Field(i)

		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}

		// Without omitempty a field is always sent: required.
		if !strings.Contains(opts, "omitempty") && !slices.Contains(p.Schema.Required, name) {
			p.Schema.Required = append(p.Schema.Required, name)
		}

		prop, ok := p.Schema.Properties[name]
		if !ok || prop.TypeObject == nil {
			continue
		}

		exact(prop.TypeObject, f.Type)

		// An omitempty field is left out rather than sent as null.
		if strings.Contains(opts, "omitempty") {
			stripNull(prop.TypeObject)
		}

		if doc := comments[key+"."+f.Name]; doc != "" && prop.TypeObject.Description == nil {
			prop.TypeObject.WithDescription(doc)
		}

		p.Schema.Properties[name] = prop
	}

	return false, nil
}

// stripNull removes null from s's types.
func stripNull(s *jsonschema.Schema) {
	if s.Type == nil || len(s.Type.SliceOfSimpleTypeValues) == 0 {
		return
	}

	types := slices.DeleteFunc(slices.Clone(s.Type.SliceOfSimpleTypeValues), func(v jsonschema.SimpleType) bool { return v == jsonschema.Null })
	if len(types) == 1 {
		s.Type = &jsonschema.Type{SimpleTypes: &types[0]}
	} else {
		s.Type.SliceOfSimpleTypeValues = types
	}
}

// exact tightens a value's schema to what the host sends for Go type t: a
// slice, array or map is never null (nil ones are normalized away), and an
// array has its fixed length, so it becomes a tuple in generated types.
func exact(s *jsonschema.Schema, t reflect.Type) {
	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
	default:
		return
	}

	stripNull(s)

	if t.Kind() == reflect.Array {
		s.WithMinItems(int64(t.Len())).WithMaxItems(int64(t.Len()))
	}

	if t.Kind() != reflect.Map && s.Items != nil && s.Items.SchemaOrBool != nil && s.Items.SchemaOrBool.TypeObject != nil {
		exact(s.Items.SchemaOrBool.TypeObject, t.Elem())
	}
}

// canonical spells a package path, or a name starting with one, the way
// comments keys it: package main by its import path, which only test
// binaries report.
func canonical(name string) string {
	if name == "main" || strings.HasPrefix(name, "main.") {
		return mainPackage + strings.TrimPrefix(name, "main")
	}

	return name
}

// errNoBody is a body field without content types.
var errNoBody = errors.New(`body field needs content types, e.g. body:"application/json"`)
