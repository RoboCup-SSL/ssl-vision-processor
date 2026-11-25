package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Topic is one WebSocket topic: its data is what a GET route answers, or one
// field of it.
type Topic struct {
	Name string
	// Route is the GET route's path.
	Route string
	// Field is the field of Route's answer the topic carries, "" for all of it.
	Field       string
	Description string
}

// topicsSchema is the component schema with one property per topic, so
// generated clients get a map from topic name to its data.
const topicsSchema = "Topics"

const refPrefix = "#/components/schemas/"

// Topics documents the WebSocket topics as the Topics schema. Call it once
// the routes they name are registered.
func (a *API) Topics(topics []Topic) error {
	data, err := json.Marshal(a.r.Spec)
	if err != nil {
		return err
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
		return err
	}

	properties := map[string]any{}
	required := []any{}

	for _, t := range topics {
		schema := spec.Paths[t.Route]["get"].Responses["200"].Content["application/json"].Schema
		if schema == nil {
			return fmt.Errorf("topic %s: GET %s has no JSON answer", t.Name, t.Route)
		}

		if t.Field != "" {
			ref, _ := schema["$ref"].(string)
			fields, _ := spec.Components.Schemas[strings.TrimPrefix(ref, refPrefix)]["properties"].(map[string]any)

			field, ok := fields[t.Field].(map[string]any)
			if !ok {
				return fmt.Errorf("topic %s: GET %s has no field %s", t.Name, t.Route, t.Field)
			}

			schema = field
		}

		properties[t.Name] = schema
		required = append(required, t.Name)
	}

	a.r.Spec.ComponentsEns().WithSchemasItem(topicsSchema, map[string]any{
		"type":        "object",
		"description": "Each WebSocket topic's data, by topic name. Not itself sent: see Live updates.",
		"properties":  properties,
		"required":    required,
	})

	return nil
}

// TopicTable is topics as a Markdown table, for the spec's introduction.
func TopicTable(topics []Topic) string {
	var b strings.Builder

	b.WriteString("| Topic | Data | Published |\n| --- | --- | --- |\n")

	for _, t := range topics {
		same := "GET " + t.Route
		if t.Field != "" {
			same = fmt.Sprintf("`%s` in %s", t.Field, same)
		}

		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", t.Name, same, t.Description)
	}

	return b.String()
}
