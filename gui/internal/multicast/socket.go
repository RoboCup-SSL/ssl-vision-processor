package multicast

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/libp2p/go-reuseport"
	"golang.org/x/net/ipv4"
)

// errNoInterface is the problem an endpoint reports while no network
// interface is usable: suspended, unplugged, or everything skipped.
var errNoInterface = errors.New("no usable network interface")

// openSocket receives on one plain UDP socket:
//
//   - it hears only the groups it joined itself (onlyJoinedGroups), so
//     groups sharing a port (every camera's video uses 10100) each get only
//     their own traffic. Binding the group address doesn't do this in Go,
//     which binds a multicast address as 0.0.0.0;
//   - it joins the group on every used interface at once;
//   - its kernel receive buffer is opts.ReadBuffer, room for a video
//     keyframe's burst of packets. The kernel caps it at net.core.rmem_max.
//
// A non-multicast address binds 0.0.0.0 on the port instead, hearing
// broadcast (or unicast to this host) on every interface. The returned
// problem is "" if the socket is fully set up. Even with a problem, stop is
// safe to call.
func openSocket(address string, ifaces []Interface, opts Options, receive func([]byte, *net.UDPAddr)) (stop func(), problem string) {
	noop := func() {}

	addr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return noop, err.Error()
	}

	bind := addr.IP
	if !bind.IsMulticast() {
		bind = net.IPv4zero
	}

	pc, err := reuseport.ListenPacket("udp4", net.JoinHostPort(bind.String(), strconv.Itoa(addr.Port)))
	if err != nil {
		return noop, fmt.Sprintf("can't listen: %v", err)
	}

	conn, ok := pc.(*net.UDPConn)
	if !ok {
		_ = pc.Close()

		return noop, fmt.Sprintf("unexpected socket type %T", pc)
	}

	if err := conn.SetReadBuffer(opts.ReadBuffer); err != nil {
		slog.Debug("can't set receive buffer", "address", address, "err", err)
	}

	if addr.IP.IsMulticast() {
		if err := onlyJoinedGroups(conn); err != nil {
			slog.Warn("can't limit socket to its own group; it may hear other groups on its port", "address", address, "err", err)
		}

		if err := join(ipv4.NewPacketConn(conn), addr, ifaces); err != nil {
			problem = err.Error()
		}
	}

	var done sync.WaitGroup

	done.Add(1)

	go func() {
		defer done.Done()

		buf := make([]byte, 65536)

		for {
			n, from, err := conn.ReadFromUDP(buf)
			if err != nil {
				return // closed by stop
			}

			receive(buf[:n], from)
		}
	}()

	return func() {
		_ = conn.Close()
		done.Wait()
	}, problem
}

// join joins addr's group on every used interface. It fails only if it
// joined none: some interfaces refusing (one without multicast, say) is fine
// as long as another works.
func join(pc *ipv4.PacketConn, addr *net.UDPAddr, ifaces []Interface) error {
	group := &net.UDPAddr{IP: addr.IP}

	var (
		joined int
		errs   error
	)

	for _, i := range ifaces {
		if !i.Used {
			continue
		}

		ifi, err := net.InterfaceByName(i.Name)
		if err == nil {
			err = pc.JoinGroup(ifi, group)
		}

		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", i.Name, err))

			continue
		}

		joined++
	}

	switch {
	case joined > 0:
		if errs != nil {
			slog.Debug("joined the group on some interfaces only", "group", addr.IP, "err", errs)
		}

		return nil
	case errs == nil:
		return errNoInterface
	default:
		return fmt.Errorf("can't join %s: %w", addr.IP, errs)
	}
}
