package main

import (
	"encoding/json"
	"log/slog"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/api"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/discovery"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/livefield"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/supervisor"
)

// The WebSocket topics, the only list of them: the hub, GET /api/topics, the
// API reference and the frontend's TopicMap (through the spec's Topics schema)
// all come from here.
var (
	registeredTopics []api.Topic

	alertsTopic    = newTopic[[]alerts.Alert](api.Topic{Name: "alerts.state", Route: "/api/alerts", Description: everySecond})
	configTopic    = newTopic[config.State](api.Topic{Name: "config.state", Route: "/api/config", Field: "state", Description: everySecond})
	geometryTopic  = newTopic[wrapperPacket](api.Topic{Name: "geometry.state", Route: "/api/geometry", Description: "Once a second."})
	instancesTopic = newTopic[[]discovery.Instance](api.Topic{Name: "instances.state", Route: "/api/instances", Description: everySecond})
	liveFieldTopic = newTopic[livefield.Snapshot](api.Topic{Name: "live-field.state", Route: "/api/live-field", Description: "Up to 60 times a second while something new arrives, else once a second; only while subscribed."})
	networkTopic   = newTopic[NetworkState](api.Topic{Name: "network.state", Route: "/api/network", Description: everySecond})
	processesTopic = newTopic[[]supervisor.Status](api.Topic{Name: "processes.state", Route: "/api/processes", Description: "Once a second, and on every state change."})
)

const everySecond = "Once a second, and on every configuration change."

// topic is a WebSocket topic carrying T.
type topic[T any] struct{ name string }

func newTopic[T any](t api.Topic) topic[T] {
	registeredTopics = append(registeredTopics, t)

	return topic[T]{t.Name}
}

// publish sends v to the topic's subscribers.
func (t topic[T]) publish(wsHub *hub.Hub, v T) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("marshalling websocket topic", "topic", t.name, "err", err)

		return
	}

	wsHub.Publish(t.name, data)
}

// newHub is a hub with every topic.
func newHub() *hub.Hub {
	names := make([]string, len(registeredTopics))
	for i, t := range registeredTopics {
		names[i] = t.Name
	}

	return hub.New(names...)
}
