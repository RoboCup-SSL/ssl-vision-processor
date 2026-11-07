package main

import (
	"os"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/referee"
)

// StatusSource is the live host state the API serves beside the store's:
// what the sockets hear, and the alerts derived from it all.
type StatusSource interface {
	Network() NetworkState
	Alerts() []alerts.Alert
}

// NetworkState is each socket's address and what it has heard, plus the host
// facts the Network page's presets and notes need. Served as network.state
// and GET /api/network.
type NetworkState struct {
	Vision                multicast.Status      `json:"vision"`
	GC                    multicast.Status      `json:"gc"`
	Cameras               []detections.Source   `json:"cameras"`
	Referee               referee.State         `json:"referee"`
	AutoInterfaces        bool                  `json:"autoInterfaces"`
	Interfaces            []multicast.Interface `json:"interfaces"`
	Loopback              string                `json:"loopback"`
	LoopbackMulticast     bool                  `json:"loopbackMulticast"`
	UnprivilegedPortStart int                   `json:"unprivilegedPortStart"`
	Root                  bool                  `json:"root"`
}

// hostStatus is the running host's StatusSource.
type hostStatus struct {
	store   *config.Store
	sockets *networkSockets
	match   *referee.Match
}

func (h *hostStatus) Network() NetworkState {
	now := time.Now()

	h.sockets.mu.Lock()
	auto, ifaces := h.sockets.auto, h.sockets.ifaces
	h.sockets.mu.Unlock()

	loopback, loopbackMulticast, _ := multicast.LoopbackMulticast()

	return NetworkState{
		Vision:                h.sockets.vision.Status(now),
		GC:                    h.sockets.gc.Status(now),
		Cameras:               nonNil(h.sockets.detections.Sources(now)),
		Referee:               h.match.State(),
		AutoInterfaces:        auto,
		Interfaces:            nonNil(ifaces),
		Loopback:              loopback,
		LoopbackMulticast:     loopbackMulticast,
		UnprivilegedPortStart: multicast.UnprivilegedPortStart(),
		Root:                  os.Geteuid() == 0,
	}
}

func (h *hostStatus) Alerts() []alerts.Alert {
	doc, _ := h.store.Working()
	n := h.Network()

	return alerts.Collect(doc, h.store.State(), alerts.Network{
		Vision:            n.Vision,
		GC:                n.GC,
		Sources:           n.Cameras,
		Referee:           n.Referee,
		Loopback:          n.Loopback,
		LoopbackMulticast: n.LoopbackMulticast,
	})
}

// nonNil keeps an empty list a JSON [] rather than null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
