package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

// Tracker is the group autorefs send TrackerWrapperPacket to. Only this host
// listens on it; no vision_processor does, so it lives at the top level
// rather than in defaults.network, and is never rendered into a camera's
// config.yml. Unset keys take the league's standard group.
type Tracker struct {
	IP   *string `yaml:"ip,omitempty" json:"ip,omitempty"`
	Port *int    `yaml:"port,omitempty" json:"port,omitempty"`
}

const (
	defaultTrackerIP   = "224.5.23.2"
	defaultTrackerPort = 10010
)

// TrackerIP and TrackerPort are the tracker group with fallbacks applied.
func (d Document) TrackerIP() string {
	if d.Tracker == nil || d.Tracker.IP == nil {
		return defaultTrackerIP
	}

	return *d.Tracker.IP
}

func (d Document) TrackerPort() int {
	if d.Tracker == nil || d.Tracker.Port == nil {
		return defaultTrackerPort
	}

	return *d.Tracker.Port
}

// TrackerAddress is the tracker group as host:port.
func (d Document) TrackerAddress() string {
	return net.JoinHostPort(d.TrackerIP(), strconv.Itoa(d.TrackerPort()))
}

// validateTracker checks the tracker group's format and, given a valid
// network block n, that it doesn't collide with the vision or game controller
// group. Sharing the vision group's IP on another port is the standard setup.
func (d Document) validateTracker(n *Network) error {
	errs := errors.Join(
		validateGroup("tracker.ip", d.TrackerIP()),
		validatePort("tracker.port", d.TrackerPort()),
	)

	if n == nil {
		return errs
	}

	switch d.TrackerAddress() {
	case n.VisionAddress():
		errs = errors.Join(errs, fmt.Errorf("tracker: can't share %s with vision", n.VisionAddress()))
	case n.GCAddress():
		errs = errors.Join(errs, fmt.Errorf("tracker: can't share %s with the game controller", n.GCAddress()))
	}

	return errs
}
