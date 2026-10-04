// Package multicast holds the host's SSL network sockets: the vision group
// (geometry in and out) and the game controller group (referee in). Each is an
// Endpoint whose address can change while running; the old sockets are torn
// down and new ones opened on the new address. A multicast address joins the
// group; anything else (normally a broadcast address, for switches whose IGMP
// snooping drops multicast) listens on the port instead.
package multicast

import (
	"context"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-go-tools/pkg/sslnet"
)

// ReceivingWindow is how recently a packet must have been heard for an
// endpoint to count as receiving.
const ReceivingWindow = 2 * time.Second

// Options configures an Endpoint's sockets.
type Options struct {
	Verbose bool
	// Send also opens a sender on the group, for Endpoint.Send.
	Send bool
	// ReadBuffer, when set, receives on a plain socket with this kernel
	// receive buffer instead of sslnet's receiver (see openSocket). For
	// high-rate streams; such an endpoint can't send.
	ReadBuffer int
}

// Consumer handles one datagram and reports whether it counts as heard: a
// valid packet from someone else, not noise or our own looped-back sends.
type Consumer func(data []byte) bool

// opener opens sockets on address over the interfaces marked Used,
// delivering datagrams to receive. It returns how to send (nil when not
// sending) and how to close everything. Swapped out in tests, which can't
// rely on joining a real multicast group.
type opener func(address string, ifaces []Interface, opts Options, receive func([]byte, *net.UDPAddr)) (send func([]byte), stop func())

// Endpoint is one multicast group the host listens on, and optionally sends to.
type Endpoint struct {
	name    string
	opts    Options
	consume Consumer
	open    opener

	// poke wakes Run after SetAddress or SetInterfaces. Size 1: several
	// changes before Run gets to it collapse into one reopen.
	poke chan struct{}

	mu        sync.Mutex
	address   string
	ifaces    []Interface
	send      func([]byte)
	heard     uint64
	lastHeard time.Time
	source    string
}

// Status is an endpoint's address and what it has heard there.
type Status struct {
	Address string `json:"address"`
	// Mode is how the receiver was opened: "multicast" joined the group,
	// "port" listens on the port for anything sent there (broadcast, or
	// unicast to this host).
	Mode      string     `json:"mode"`
	Heard     uint64     `json:"heard"`
	LastHeard *time.Time `json:"lastHeard,omitempty"`
	Source    string     `json:"source,omitempty"`
	Receiving bool       `json:"receiving"`
}

// NewEndpoint prepares an endpoint on address (e.g. "224.5.23.2:10006")
// over ifaces (see Select). Nothing is opened until Run.
func NewEndpoint(name, address string, ifaces []Interface, opts Options, consume Consumer) *Endpoint {
	open := openSSLNet
	if opts.ReadBuffer > 0 {
		open = openSocket
	}

	return &Endpoint{
		name:    name,
		opts:    opts,
		consume: consume,
		open:    open,
		poke:    make(chan struct{}, 1),
		address: address,
		ifaces:  ifaces,
	}
}

// Address is the address the endpoint is (or is about to be) open on.
func (e *Endpoint) Address() string {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.address
}

// SetAddress moves the endpoint to address: Run closes the current sockets
// and opens new ones. Counters restart, since they describe the old group.
// A no-op when address is unchanged.
func (e *Endpoint) SetAddress(address string) {
	e.mu.Lock()

	if address == e.address {
		e.mu.Unlock()

		return
	}

	e.address = address
	e.heard = 0
	e.lastHeard = time.Time{}
	e.source = ""
	e.mu.Unlock()

	e.reopen()
}

// SetInterfaces moves the endpoint onto a new interface selection (see
// Select). A no-op unless the used interfaces or their addresses changed,
// since the host re-evaluates automatic selection every second.
func (e *Endpoint) SetInterfaces(ifaces []Interface) {
	e.mu.Lock()

	if slices.Equal(usedKey(ifaces), usedKey(e.ifaces)) {
		e.mu.Unlock()

		return
	}

	e.ifaces = ifaces
	e.mu.Unlock()

	e.reopen()
}

// usedKey is the part of a selection the sockets depend on.
func usedKey(ifaces []Interface) []string {
	var key []string

	for _, i := range ifaces {
		if i.Used {
			key = append(key, i.Name+"="+i.Address)
		}
	}

	return key
}

func (e *Endpoint) reopen() {
	select {
	case e.poke <- struct{}{}:
	default:
	}
}

// Run keeps the endpoint open on its current address until ctx is cancelled,
// reopening whenever SetAddress or SetInterfaces changes it.
func (e *Endpoint) Run(ctx context.Context) error {
	for {
		e.mu.Lock()
		address, ifaces := e.address, e.ifaces
		e.mu.Unlock()

		stop := e.start(address, ifaces)

		select {
		case <-ctx.Done():
			stop()

			return ctx.Err()
		case <-e.poke:
		}

		stop()
		slog.Info("reopening socket", "endpoint", e.name, "from", address, "to", e.Address(), "interfaces", usedKey(e.interfaces()))
	}
}

func (e *Endpoint) interfaces() []Interface {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.ifaces
}

func (e *Endpoint) start(address string, ifaces []Interface) func() {
	send, stop := e.open(address, ifaces, e.opts, func(data []byte, from *net.UDPAddr) {
		e.receive(address, data, from)
	})

	e.mu.Lock()
	e.send = send
	e.mu.Unlock()

	slog.Info("socket open", "endpoint", e.name, "address", address)

	return func() {
		e.mu.Lock()
		e.send = nil
		e.mu.Unlock()

		stop()
	}
}

// receive handles a datagram that arrived on address. A closing socket can
// still deliver one after a move; it's dropped rather than counted against
// the new address.
func (e *Endpoint) receive(address string, data []byte, from *net.UDPAddr) {
	if !e.consume(data) {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if address != e.address {
		return
	}

	e.heard++
	e.lastHeard = time.Now()

	if from != nil {
		e.source = from.IP.String()
	}
}

// Send publishes data onto the group. Dropped while the endpoint is between
// sockets, or if it wasn't opened with Options.Send.
func (e *Endpoint) Send(data []byte) {
	e.mu.Lock()
	send := e.send
	e.mu.Unlock()

	if send != nil {
		send(data)
	}
}

// Status reports the endpoint as of now.
func (e *Endpoint) Status(now time.Time) Status {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := Status{
		Address: e.address,
		Mode:    modeOf(e.address),
		Heard:   e.heard,
		Source:  e.source,
	}

	if !e.lastHeard.IsZero() {
		last := e.lastHeard
		s.LastHeard = &last
		s.Receiving = now.Sub(last) <= ReceivingWindow
	}

	return s
}

// modeOf says how address is received: by joining it as a multicast group, or
// by listening on its port for anything sent there (broadcast, or unicast to
// this host).
func modeOf(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return "multicast"
	}

	if ip := net.ParseIP(host); ip != nil && !ip.IsMulticast() {
		return "port"
	}

	return "multicast"
}

// receiver is what sslnet's MulticastServer and BroadcastServer have in common.
type receiver interface {
	Start()
	Stop()
}

func openSSLNet(address string, ifaces []Interface, opts Options, receive func([]byte, *net.UDPAddr)) (func([]byte), func()) {
	var server receiver

	if modeOf(address) == "port" {
		// Binds 0.0.0.0, so it hears every interface whatever the selection;
		// only sending follows it.
		s := sslnet.NewBroadcastServer(address)
		s.Verbose = opts.Verbose
		s.Consumer = receive
		server = s
	} else {
		s := sslnet.NewMulticastServer(address)
		s.SkipInterfaces = skipped(ifaces)
		s.Verbose = opts.Verbose
		s.Consumer = receive
		server = s
	}

	server.Start()

	if !opts.Send {
		return nil, server.Stop
	}

	out := openSender(address, ifaces)

	return out.send, func() {
		server.Stop()
		out.close()
	}
}

// skipped names the interfaces a selection doesn't use.
func skipped(ifaces []Interface) []string {
	var names []string

	for _, i := range ifaces {
		if !i.Used {
			names = append(names, i.Name)
		}
	}

	return names
}
