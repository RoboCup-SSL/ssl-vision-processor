package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Network is the vision and game controller multicast addresses, from the
// config.yml-layout network: block. vision_processor reads the same block at
// startup, and the host uses it for its own sockets, so both always agree.
type Network struct {
	VisionIP   string
	VisionPort int
	GCIP       string
	GCPort     int
}

// The vision_processor's own fallbacks (Resources.cpp) for keys the network
// block leaves out.
var defaultNetwork = Network{
	VisionIP:   "224.5.23.2",
	VisionPort: 10006,
	GCIP:       "224.5.23.1",
	GCPort:     10003,
}

// networkBlock is the YAML shape of a network: block. Pointers tell a missing
// key (use the fallback) from a zero one (invalid).
type networkBlock struct {
	VisionIP   *string `yaml:"vision_ip"`
	VisionPort *int    `yaml:"vision_port"`
	GCIP       *string `yaml:"gc_ip"`
	GCPort     *int    `yaml:"gc_port"`
}

// Network is the shared network block (defaults.network) with fallbacks
// applied: what the host listens on, and what every camera gets unless it
// overrides it.
func (d Document) Network() (Network, error) {
	return networkFromBlock(d.Defaults["network"])
}

// VisionAddress is the vision group as host:port.
func (n Network) VisionAddress() string {
	return net.JoinHostPort(n.VisionIP, strconv.Itoa(n.VisionPort))
}

// GCAddress is the game controller group as host:port.
func (n Network) GCAddress() string {
	return net.JoinHostPort(n.GCIP, strconv.Itoa(n.GCPort))
}

func networkFromBlock(block any) (Network, error) {
	var parsed networkBlock

	if block != nil {
		data, err := yaml.Marshal(block)
		if err != nil {
			return Network{}, err
		}

		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return Network{}, &ValidationError{fmt.Errorf("network: %w", err)}
		}
	}

	n := defaultNetwork

	if parsed.VisionIP != nil {
		n.VisionIP = *parsed.VisionIP
	}

	if parsed.VisionPort != nil {
		n.VisionPort = *parsed.VisionPort
	}

	if parsed.GCIP != nil {
		n.GCIP = *parsed.GCIP
	}

	if parsed.GCPort != nil {
		n.GCPort = *parsed.GCPort
	}

	return n, nil
}

// Validate reports every problem at once. Any IPv4 address is accepted:
// multicast is the norm, broadcast works around switches whose IGMP snooping
// drops multicast, and the GUI warns about anything else rather than refusing
// it.
func (n Network) Validate() error {
	var errs error

	errs = errors.Join(errs, validateGroup("vision_ip", n.VisionIP))
	errs = errors.Join(errs, validatePort("vision_port", n.VisionPort))
	errs = errors.Join(errs, validateGroup("gc_ip", n.GCIP))
	errs = errors.Join(errs, validatePort("gc_port", n.GCPort))

	if n.VisionAddress() == n.GCAddress() {
		errs = errors.Join(errs, fmt.Errorf("network: vision and game controller can't share %s", n.VisionAddress()))
	}

	return errs
}

func validateGroup(key, ip string) error {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return fmt.Errorf("network.%s: want an IPv4 address, got %q", key, ip)
	}

	if addr.IsUnspecified() {
		return fmt.Errorf("network.%s: 0.0.0.0 isn't a destination", key)
	}

	return nil
}

func validatePort(key string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("network.%s: want 1 to 65535, got %d", key, port)
	}

	return nil
}
