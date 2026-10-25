package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/api"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/origins"
	"github.com/swaggest/jsonschema-go"
)

// specVersion is the API's version in the spec. Fields and endpoints may be
// added freely; changing or removing one means /api/v2.
const specVersion = "1.0.0"

const specIntro = `The GUI host's control API: the shared vision.yml document, its state, and what the host's
sockets hear. Every error is JSON, ` + "`{\"error\": code, \"message\": text, \"issues\": [...]}`" + `; clients branch on
` + "`error`" + `. Request bodies must be JSON and unknown keys are rejected.

## Live updates

` + "`/ws`" + ` is a WebSocket carrying topics. Send ` + "`{\"action\": \"subscribe\", \"topic\": \"...\"}`" + ` (or
` + "`unsubscribe`" + `); each message is ` + "`{\"topic\": \"...\", \"data\": ...}`" + `. A new subscriber gets the latest value at
once. A message the host can't act on is answered with an error and the connection stays open. Each topic's data
is the body of a REST endpoint documented here:

| Topic | Same as |
| --- | --- |
| ` + "`config.state`" + ` | ` + "`state`" + ` in GET /api/config |
| ` + "`network.state`" + ` | GET /api/network |
| ` + "`alerts.state`" + ` | GET /api/alerts |
| ` + "`wrapper_packet.out`" + ` | GET /api/geometry |
`

// openapiSpec is the host's OpenAPI spec as JSON. It depends only on the
// code, so it's built from routes registered on a server with nothing behind
// it; no handler runs.
func openapiSpec() ([]byte, error) {
	mux := http.NewServeMux()
	a := newAPI(mux)
	(&VisionServer{origins: &origins.Policy{}}).addRoutes(mux, a)

	return a.Spec()
}

// newAPI is the host's typed route registry and spec.
func newAPI(mux *http.ServeMux) *api.API {
	a := api.New(mux, api.Info{Title: "vision_processor GUI host", Version: specVersion, Description: specIntro},
		func(w http.ResponseWriter, err error) { respondConfigErr(w, err, "handling request") })

	a.SchemaFor(reflect.TypeFor[config.Settings](), settingsSchema())
	var rev jsonschema.Schema
	rev.WithType(jsonschema.String.Type()).
		WithDescription(`A document revision, quoted as an ETag: "12".`).
		WithExamples(`"12"`)
	a.SchemaFor(reflect.TypeFor[revision](), rev)

	packet := mustSchema(map[string]any{
		"type": "object",
		"description": "An SSL_WrapperPacket from the league's ssl_vision_wrapper.proto, as canonical protojson " +
			"(camelCase field names, enums as strings).",
		"properties": map[string]any{
			"geometry": map[string]any{
				"type":        "object",
				"description": "SSL_GeometryData.",
				"properties": map[string]any{
					"field": map[string]any{"type": "object", "description": "SSL_GeometryFieldSize: the field template, its lines and arcs."},
					"calib": map[string]any{"type": "array", "description": "SSL_GeometryCameraCalibration, one per camera.", "items": map[string]any{"type": "object"}},
				},
			},
		},
	})
	a.SchemaFor(reflect.TypeFor[wrapperPacket](), packet)

	return a
}

// settingsSchema is config.CurrentSchema() as JSON Schema: one object per
// config.yml block, one property per key, with vision_processor's fallback,
// bounds, and when a change takes effect.
func settingsSchema() jsonschema.Schema {
	applies := map[string]string{
		config.ApplyLive:    "Reloaded live, within a second.",
		config.ApplyRestart: "Takes effect when vision_processor restarts.",
		config.ApplyDerived: "Written by the host from the document; don't set it.",
		config.ApplyHost:    "Read only by the GUI host.",
	}

	blocks := map[string]map[string]any{}
	top := map[string]any{}

	for path, s := range config.CurrentSchema().Settings {
		var prop map[string]any

		switch s.Type {
		case config.TypeRGB:
			prop = map[string]any{"type": "array", "minItems": 3, "maxItems": 3, "items": map[string]any{"type": "integer", "minimum": 0, "maximum": 255}}
		case config.TypeWhiteBalance:
			prop = map[string]any{"oneOf": []any{
				map[string]any{"type": "string", "enum": s.Enum},
				map[string]any{"type": "object", "properties": map[string]any{"red": map[string]any{"type": "number"}, "blue": map[string]any{"type": "number"}}},
			}}
		case config.TypePoints:
			prop = map[string]any{"type": "array", "items": map[string]any{"type": "array", "minItems": 2, "maxItems": 2, "items": map[string]any{"type": "integer"}}}
		default:
			prop = map[string]any{"type": s.Type}
			if len(s.Enum) > 0 {
				prop["enum"] = s.Enum
			}
		}

		if s.Default != nil {
			prop["default"] = s.Default
		}

		if s.Minimum != nil {
			if s.ExclusiveMinimum {
				prop["exclusiveMinimum"] = *s.Minimum
			} else {
				prop["minimum"] = *s.Minimum
			}
		}

		if s.Maximum != nil {
			prop["maximum"] = *s.Maximum
		}

		desc := applies[s.Apply]
		if s.Unit != "" {
			desc += " Unit: " + s.Unit + "."
		}

		prop["description"] = desc

		if s.Apply == config.ApplyDerived {
			prop["readOnly"] = true
		}

		block, key, nested := strings.Cut(path, ".")
		if !nested {
			top[path] = prop

			continue
		}

		if blocks[block] == nil {
			blocks[block] = map[string]any{}
		}

		blocks[block][key] = prop
	}

	for name, props := range blocks {
		top[name] = map[string]any{"type": "object", "properties": props}
	}

	return mustSchema(map[string]any{
		"type":        "object",
		"description": "vision_processor's config.yml layout, as GET /api/schema describes it. A key it doesn't read is kept but raises an unknown_setting warning.",
		"properties":  top,
	})
}

// mustSchema is a JSON Schema written as plain values.
func mustSchema(v map[string]any) jsonschema.Schema {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("schema: %v", err))
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		panic(fmt.Sprintf("schema: %v", err))
	}

	return schema
}
