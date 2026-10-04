// Package multicast holds the host's SSL network sockets: the vision group
// (geometry in and out), the game controller group (referee in), and each
// camera's video stream. Each is an Endpoint whose address can change while
// running; the old sockets are torn down and new ones opened on the new
// address. A multicast address joins the group; anything else (normally a
// broadcast address, for switches whose IGMP snooping drops multicast)
// listens on the port instead.
//
// Endpoints heal themselves: while a socket couldn't be set up (no usable
// interface during a suspend, a cable pulled), they retry every
// RetryInterval, logging only when the problem changes.
package multicast

import (
	"context"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"
)

// ReceivingWindow is how recently a packet must have been heard for an
// endpoint to count as receiving.
const ReceivingWindow = 2 * time.Second

// RetryInterval is how often an endpoint whose sockets aren't fully set up
// tries again.
const RetryInterval = 5 * time.Second

// defaultReadBuffer is the kernel receive buffer for endpoints that don't
// ask for one: plenty for the vision and game controller groups' rates.
const defaultReadBuffer = 1 << 20

// Options configures an Endpoint's sockets.
type Options struct {
	Verbose bool
	// Send also opens a sender on the group, for Endpoint.Send.
	Send bool
	// ReadBuffer is the kernel receive buffer; 0 means defaultReadBuffer.
	// Video sets more, for a keyframe's burst of packets.
	ReadBuffer int
}

// Consumer handles one datagram and reports whether it counts as heard: a
// valid packet from someone else, not noise or our own looped-back sends.
type Consumer func(data []byte) bool

// sockets is what an opener hands back.
type sockets struct {
	// send is nil when not sending.
	send func([]byte)
	stop func()
	// health is "" while everything that should be open is, else what's
	// wrong ("no usable network interface", a join error, a send socket
	// lost to an interface going away). Checked every RetryInterval.
	health func() string
}

// opener opens sockets on address over the interfaces marked Used,
// delivering datagrams to receive. Swapped out in tests, which can't rely on
// joining a real multicast group.
type opener func(address string, ifaces []Interface, opts Options, receive func([]byte, *net.UDPAddr)) sockets

// Endpoint is one multicast group the host listens on, and optionally sends to.
type Endpoint struct {
	name    string
	opts    Options
	consume Consumer
	open    opener
	retry   time.Duration

	// poke wakes Run after SetAddress, SetInterfaces, or Reopen. Size 1:
	// several before Run gets to it collapse into one reopen.
	poke chan struct{}

	mu        sync.Mutex
	address   string
	ifaces    []Interface
	send      func([]byte)
	health    func() string
	problem   string // last reported, so it's logged when it changes
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
	// Problem is why the sockets aren't fully open, if they aren't.
	Problem string `json:"problem,omitempty"`
}

// NewEndpoint prepares an endpoint on address (e.g. "224.5.23.2:10006")
// over ifaces (see Select). Nothing is opened until Run.
func NewEndpoint(name, address string, ifaces []Interface, opts Options, consume Consumer) *Endpoint {
	if opts.ReadBuffer == 0 {
		opts.ReadBuffer = defaultReadBuffer
	}

	return &Endpoint{
		name:    name,
		opts:    opts,
		consume: consume,
		open:    openSockets,
		retry:   RetryInterval,
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

// Reopen closes and reopens the sockets even though nothing changed: after a
// suspend, the old ones may be bound to state the network no longer has.
func (e *Endpoint) Reopen() {
	e.reopen()
}

func (e *Endpoint) reopen() {
	select {
	case e.poke <- struct{}{}:
	default:
	}
}

// Run keeps the endpoint open on its current address until ctx is cancelled,
// reopening whenever SetAddress, SetInterfaces, or Reopen asks, and every
// RetryInterval while the sockets aren't fully set up.
func (e *Endpoint) Run(ctx context.Context) error {
	retrying := false

	for {
		e.mu.Lock()
		address, ifaces := e.address, e.ifaces
		e.mu.Unlock()

		s := e.start(address, ifaces, retrying)

		var err error

		retrying, err = e.wait(ctx, s.health)

		s.stop()

		e.mu.Lock()
		e.send, e.health = nil, nil
		e.mu.Unlock()

		if err != nil {
			return err
		}

		if !retrying {
			slog.Info("reopening socket", "endpoint", e.name, "from", address, "to", e.Address(), "interfaces", usedKey(e.interfaces()))
		}
	}
}

// wait returns when the sockets should be reopened: retrying is true when
// it's because they're unhealthy rather than because something changed.
func (e *Endpoint) wait(ctx context.Context, health func() string) (retrying bool, err error) {
	ticker := time.NewTicker(e.retry)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-e.poke:
			return false, nil
		case <-ticker.C:
			if problem := health(); problem != "" {
				e.report(problem)

				return true, nil
			}
		}
	}
}

// report logs a change in what's wrong, so a socket retrying all night
// doesn't fill the log.
func (e *Endpoint) report(problem string) {
	e.mu.Lock()
	previous := e.problem
	e.problem = problem
	e.mu.Unlock()

	switch {
	case problem == previous:
	case problem == "":
		slog.Info("socket healthy again", "endpoint", e.name, "address", e.Address())
	default:
		slog.Warn("socket not fully open, retrying", "endpoint", e.name, "address", e.Address(), "problem", problem, "every", e.retry)
	}
}

func (e *Endpoint) interfaces() []Interface {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.ifaces
}

func (e *Endpoint) start(address string, ifaces []Interface, retrying bool) sockets {
	s := e.open(address, ifaces, e.opts, func(data []byte, from *net.UDPAddr) {
		e.receive(address, data, from)
	})

	e.mu.Lock()
	e.send, e.health = s.send, s.health
	e.mu.Unlock()

	problem := s.health()

	// A retry that's still failing stays quiet; report says what's wrong
	// when that changes.
	if retrying && problem != "" {
		slog.Debug("socket retry", "endpoint", e.name, "address", address, "problem", problem)
	} else {
		slog.Info("socket open", "endpoint", e.name, "address", address, "interfaces", usedKey(ifaces))
	}

	e.report(problem)

	return s
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
		Problem: e.problem,
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

// openSockets is the real opener: a plain receiving socket (openSocket) and,
// for Options.Send, a sender on every used interface (openSender).
func openSockets(address string, ifaces []Interface, opts Options, receive func([]byte, *net.UDPAddr)) sockets {
	stopReceiver, receiverProblem := openSocket(address, ifaces, opts, receive)

	if !opts.Send {
		return sockets{
			stop:   stopReceiver,
			health: func() string { return receiverProblem },
		}
	}

	out := openSender(address, ifaces)

	return sockets{
		send: out.send,
		stop: func() {
			stopReceiver()
			out.close()
		},
		health: func() string {
			if receiverProblem != "" {
				return receiverProblem
			}

			return out.health()
		},
	}
}
