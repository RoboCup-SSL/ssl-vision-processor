// Package peers names the other senders the host hears on its multicast
// groups and watches for two of them claiming the same thing: one camera's
// calibration, detections or video, the game controller, or the field
// geometry the host itself publishes. Each is logged with the senders' IP,
// port and hostname, and served as an alert.
package peers

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// lookupTimeout bounds one reverse DNS lookup.
const lookupTimeout = 2 * time.Second

// localRefresh is how often this machine's own addresses are re-read.
const localRefresh = time.Minute

// Names resolves sender IPs to hostnames, caching every answer (including
// none) for the life of the process.
type Names struct {
	mu      sync.Mutex
	cache   map[string]string
	waiting map[string][]chan struct{}

	local     map[string]bool
	localRead time.Time
	hostname  string

	// lookup is net.DefaultResolver.LookupAddr, swapped in tests.
	lookup func(ctx context.Context, ip string) ([]string, error)
}

// NewNames returns an empty cache.
func NewNames() *Names {
	hostname, _ := os.Hostname()

	return &Names{
		cache:    map[string]string{},
		waiting:  map[string][]chan struct{}{},
		hostname: hostname,
		lookup:   net.DefaultResolver.LookupAddr,
	}
}

// Host names ip: this machine's hostname for one of its own addresses, else
// its reverse DNS name, waiting up to lookupTimeout the first time. Empty
// when it has none.
func (n *Names) Host(ip string) string {
	if n.isLocal(ip) {
		return n.hostname
	}

	n.mu.Lock()

	if name, ok := n.cache[ip]; ok {
		n.mu.Unlock()

		return name
	}

	done := make(chan struct{})
	first := len(n.waiting[ip]) == 0
	n.waiting[ip] = append(n.waiting[ip], done)
	n.mu.Unlock()

	if first {
		go n.resolve(ip)
	}

	<-done

	n.mu.Lock()
	defer n.mu.Unlock()

	return n.cache[ip]
}

func (n *Names) resolve(ip string) {
	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()

	name := ""
	if names, err := n.lookup(ctx, ip); err == nil && len(names) > 0 {
		name = strings.TrimSuffix(names[0], ".")
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.cache[ip] = name

	for _, done := range n.waiting[ip] {
		close(done)
	}

	delete(n.waiting, ip)
}

// isLocal reports whether ip is one of this machine's addresses.
func (n *Names) isLocal(ip string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if time.Since(n.localRead) > localRefresh {
		n.local = map[string]bool{}

		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipNet, ok := a.(*net.IPNet); ok {
					n.local[ipNet.IP.String()] = true
				}
			}
		}

		n.localRead = time.Now()
	}

	return n.local[ip]
}

// Sender is one sender as logs and alerts show it.
type Sender struct {
	// Address is "ip:port".
	Address string `json:"address"`
	// Host is its hostname, empty when unknown.
	Host string `json:"host,omitempty"`
	// Local is set for one of this machine's own addresses: another process
	// here, such as a second GUI host or a test.
	Local bool `json:"local"`
}

// peek is Host without waiting: the cached name, else empty while a lookup
// it starts runs in the background.
func (n *Names) peek(ip string) string {
	if n.isLocal(ip) {
		return n.hostname
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if name, ok := n.cache[ip]; ok {
		return name
	}

	if len(n.waiting[ip]) == 0 {
		n.waiting[ip] = append(n.waiting[ip], make(chan struct{}))
		go n.resolve(ip)
	}

	return ""
}

// Describe names from, waiting for its hostname the first time.
func (n *Names) Describe(from *net.UDPAddr) Sender {
	if from == nil {
		return Sender{Address: "unknown"}
	}

	ip := from.IP.String()

	return Sender{Address: from.String(), Host: n.Host(ip), Local: n.isLocal(ip)}
}

// describeCached is Describe without waiting for a hostname not yet known.
func (n *Names) describeCached(from *net.UDPAddr) Sender {
	ip := from.IP.String()

	return Sender{Address: from.String(), Host: n.peek(ip), Local: n.isLocal(ip)}
}

// String is "ip:port (hostname, this machine)", as far as known.
func (s Sender) String() string {
	var notes []string

	if s.Host != "" {
		notes = append(notes, s.Host)
	}

	if s.Local {
		notes = append(notes, "this machine")
	}

	if len(notes) == 0 {
		return s.Address
	}

	return s.Address + " (" + strings.Join(notes, ", ") + ")"
}
