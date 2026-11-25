package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/api"
)

// Every topic is published, with data matching its documented schema: what
// the frontend's TopicMap is generated from.
func TestEveryTopicMatchesItsSchema(t *testing.T) {
	h, _ := testHost(t)

	data, err := openapiSpec()
	if err != nil {
		t.Fatal(err)
	}

	var spec struct {
		Components struct{ Schemas map[string]map[string]any }
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	documented, _ := spec.Components.Schemas["Topics"]["properties"].(map[string]any)

	if len(documented) != len(h.hub.Topics()) {
		t.Errorf("%d topics documented, %d served", len(documented), len(h.hub.Topics()))
	}

	received := map[string]<-chan []byte{}

	for _, name := range h.hub.Topics() {
		ch, unsubscribe := h.hub.Subscribe(name)
		t.Cleanup(unsubscribe)

		received[name] = ch
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = publishLiveField(ctx, h.hub, h.status.live) }()

	configTopic.publish(h.hub, h.store.State())
	publishStatus(h.hub, h.status)
	processesTopic.publish(h.hub, h.procs.Statuses())
	publishGeometryToHub(h.hub, h.geom)

	for name, ch := range received {
		schema, ok := documented[name].(map[string]any)
		if !ok {
			t.Errorf("topic %s isn't documented", name)

			continue
		}

		select {
		case raw := <-ch:
			var value any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Errorf("%s: %v", name, err)
			}

			for _, problem := range conforms(name, value, schema, spec.Components.Schemas) {
				t.Error(problem)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("nothing published on %s", name)
		}
	}

	if !slices.IsSortedFunc(registeredTopics, func(a, b api.Topic) int { return strings.Compare(a.Name, b.Name) }) {
		t.Error("registeredTopics isn't sorted by name, so neither is the reference's table")
	}
}
