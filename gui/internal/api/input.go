package api

import (
	"encoding"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/httpjson"
)

// input is what Route learned about a handler's input struct.
type input struct {
	fields []param
	// params is the input struct without its body field, for the spec.
	params reflect.Type
	// body is the body field's index, or -1.
	body      int
	bodyType  reflect.Type
	bodyTypes []string
	// bodyDoc is the body field's doc comment.
	bodyDoc string
}

type param struct {
	index    int
	in       string // path, query or header
	name     string
	required bool
}

var textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()

// inputOf reads the tags on t, which must be a struct (struct{} for none).
func inputOf(t reflect.Type) (input, error) {
	if t.Kind() != reflect.Struct {
		return input{}, fmt.Errorf("input %s: want a struct", t)
	}

	in := input{body: -1}

	var paramFields []reflect.StructField

	for i := range t.NumField() {
		f := t.Field(i)

		if types, ok := f.Tag.Lookup("body"); ok {
			if in.body >= 0 {
				return input{}, fmt.Errorf("input %s: more than one body field", t)
			}

			if types == "" {
				return input{}, errNoBody
			}

			in.body, in.bodyType, in.bodyTypes = i, f.Type, strings.Split(types, ",")
			in.bodyDoc = comments[canonical(t.PkgPath())+"."+t.Name()+"."+f.Name]

			continue
		}

		for _, where := range []string{"path", "query", "header"} {
			name, ok := f.Tag.Lookup(where)
			if !ok {
				continue
			}

			if f.Type.Kind() != reflect.String && f.Type.Kind() != reflect.Int && f.Type.Kind() != reflect.Int64 &&
				!reflect.PointerTo(f.Type).Implements(textUnmarshaler) {
				return input{}, fmt.Errorf("input %s.%s: want a string, an int, or an encoding.TextUnmarshaler", t, f.Name)
			}

			// The field's doc comment becomes the parameter's description.
			if doc := comments[canonical(t.PkgPath())+"."+t.Name()+"."+f.Name]; doc != "" && f.Tag.Get("description") == "" {
				f.Tag = reflect.StructTag(fmt.Sprintf("%s description:%q", f.Tag, doc))
			}

			in.fields = append(in.fields, param{
				index:    i,
				in:       where,
				name:     name,
				required: where == "path" || f.Tag.Get("required") == "true",
			})
			paramFields = append(paramFields, f)
		}
	}

	if len(paramFields) > 0 {
		in.params = reflect.StructOf(paramFields)
	}

	return in, nil
}

// conditionalHeaders are answered 428 Precondition Required when missing
// (RFC 6585), rather than 400.
var conditionalHeaders = []string{"If-Match", "If-None-Match", "If-Unmodified-Since"}

// decode fills v from r, or answers r itself and returns false.
func (in input) decode(w http.ResponseWriter, r *http.Request, v reflect.Value) bool {
	for _, p := range in.fields {
		var raw string

		switch p.in {
		case "path":
			raw = r.PathValue(p.name)
		case "query":
			raw = r.URL.Query().Get(p.name)
		case "header":
			raw = r.Header.Get(p.name)
		}

		if raw == "" {
			if !p.required {
				continue
			}

			if p.in == "header" && slices.Contains(conditionalHeaders, p.name) {
				httpjson.Error(w, http.StatusPreconditionRequired, httpjson.CodePrecondition, p.name+": required")
			} else {
				httpjson.Error(w, http.StatusBadRequest, httpjson.CodeBadRequest, p.name+": required")
			}

			return false
		}

		if err := set(v.Field(p.index), raw); err != nil {
			httpjson.Error(w, http.StatusBadRequest, httpjson.CodeBadRequest, fmt.Sprintf("%s: %v", p.name, err))

			return false
		}
	}

	if in.body >= 0 {
		return httpjson.Decode(w, r, v.Field(in.body).Addr().Interface(), in.bodyTypes...)
	}

	return true
}

func set(f reflect.Value, raw string) error {
	if u, ok := f.Addr().Interface().(encoding.TextUnmarshaler); ok {
		return u.UnmarshalText([]byte(raw))
	}

	switch f.Kind() {
	case reflect.String:
		f.SetString(raw)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("want a whole number, got %q", raw)
		}

		f.SetInt(n)
	}

	return nil
}
