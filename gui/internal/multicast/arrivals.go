package multicast

import (
	"log/slog"
	"net"
	"slices"
	"strings"
	"time"
)

// Duplicate is one sender whose packets arrive on several interfaces at once,
// so each is heard more than once: two interfaces on one network, or a switch
// that joins two networks.
type Duplicate struct {
	// Address is the sender's "ip:port".
	Address    string   `json:"address"`
	Interfaces []string `json:"interfaces"`
}

const (
	// arrivalCount is how many packets in a row (none ReceivingWindow apart)
	// an interface needs to count, so one stray isn't a duplicate.
	arrivalCount = 3
	// maxArrivals is how many senders are kept before the silent ones are
	// dropped.
	maxArrivals = 64
)

// arrivals tracks which interfaces each sender's packets come in on. Its
// Endpoint's mu guards it.
type arrivals map[string]*senderArrivals

type senderArrivals struct {
	on     map[string]*arrival // by interface name
	warned bool
}

type arrival struct {
	last  time.Time
	count int
}

// note records one packet from from on iface. It returns the interfaces to
// warn about when the sender has just become a duplicate, else nil.
func (a arrivals) note(from *net.UDPAddr, iface string, now time.Time) []string {
	if from == nil || iface == "" {
		return nil
	}

	key := from.String()

	s := a[key]
	if s == nil {
		if len(a) >= maxArrivals {
			a.prune(now)
		}

		s = &senderArrivals{on: map[string]*arrival{}}
		a[key] = s
	}

	on := s.on[iface]
	if on == nil || now.Sub(on.last) > ReceivingWindow {
		on = &arrival{}
		s.on[iface] = on
	}

	on.last = now
	on.count++

	if len(s.on) == 1 {
		return nil // the usual case: nothing to compare
	}

	ifaces := s.active(now)

	switch {
	case len(ifaces) < 2:
		s.warned = false
	case !s.warned:
		s.warned = true

		return ifaces
	}

	return nil
}

// active lists the interfaces s is arriving on as of now.
func (s *senderArrivals) active(now time.Time) []string {
	var out []string

	for name, on := range s.on {
		if on.count >= arrivalCount && now.Sub(on.last) <= ReceivingWindow {
			out = append(out, name)
		}
	}

	slices.Sort(out)

	return out
}

func (a arrivals) prune(now time.Time) {
	for key, s := range a {
		for name, on := range s.on {
			if now.Sub(on.last) > ReceivingWindow {
				delete(s.on, name)
			}
		}

		if len(s.on) == 0 {
			delete(a, key)
		}
	}
}

// duplicates lists every sender arriving on several interfaces.
func (a arrivals) duplicates(now time.Time) []Duplicate {
	var out []Duplicate

	for key, s := range a {
		if ifaces := s.active(now); len(ifaces) > 1 {
			out = append(out, Duplicate{Address: key, Interfaces: ifaces})
		}
	}

	slices.SortFunc(out, func(a, b Duplicate) int { return strings.Compare(a.Address, b.Address) })

	return out
}

func warnDuplicate(endpoint string, from *net.UDPAddr, ifaces []string) {
	slog.Warn("one sender's packets arrive on several interfaces; exclude all but one on the Network page",
		"endpoint", endpoint, "from", from, "interfaces", ifaces)
}
