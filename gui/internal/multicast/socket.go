package multicast

import (
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/libp2p/go-reuseport"
	"golang.org/x/net/ipv4"
)

// openSocket receives on one plain UDP socket, the alternative to sslnet's
// receiver for high-rate streams such as video (Options.ReadBuffer):
//
//   - it binds the group address itself, not 0.0.0.0, so cameras sharing a
//     port each get only their own group (Linux otherwise delivers every
//     joined group on the port to every socket bound to it);
//   - it joins the group on every used interface at once, rather than
//     listening on one at a time;
//   - its kernel receive buffer is large enough for a keyframe's burst of
//     packets (sslnet sets 8 KB). The kernel caps it at net.core.rmem_max.
//
// A non-multicast address binds 0.0.0.0 on the port, as the broadcast
// receiver does. It never sends.
func openSocket(address string, ifaces []Interface, opts Options, receive func([]byte, *net.UDPAddr)) (func([]byte), func()) {
	noop := func() {}

	addr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		slog.Warn("can't listen", "address", address, "err", err)

		return nil, noop
	}

	bind := addr.IP
	if !bind.IsMulticast() {
		bind = net.IPv4zero
	}

	pc, err := reuseport.ListenPacket("udp4", net.JoinHostPort(bind.String(), strconv.Itoa(addr.Port)))
	if err != nil {
		slog.Warn("can't listen", "address", address, "err", err)

		return nil, noop
	}

	conn, ok := pc.(*net.UDPConn)
	if !ok {
		_ = pc.Close()

		return nil, noop
	}

	if err := conn.SetReadBuffer(opts.ReadBuffer); err != nil {
		slog.Warn("can't set receive buffer", "address", address, "err", err)
	}

	if addr.IP.IsMulticast() {
		join(ipv4.NewPacketConn(conn), addr, ifaces)
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

	return nil, func() {
		_ = conn.Close()
		done.Wait()
	}
}

// join joins addr's group on every used interface, or on the kernel's
// default one if none is used or found.
func join(pc *ipv4.PacketConn, addr *net.UDPAddr, ifaces []Interface) {
	group := &net.UDPAddr{IP: addr.IP}
	joined := 0

	for _, i := range ifaces {
		if !i.Used {
			continue
		}

		ifi, err := net.InterfaceByName(i.Name)
		if err != nil {
			continue
		}

		if err := pc.JoinGroup(ifi, group); err != nil {
			slog.Warn("can't join group", "group", addr.IP, "interface", i.Name, "err", err)

			continue
		}

		joined++
	}

	if joined == 0 {
		if err := pc.JoinGroup(nil, group); err != nil {
			slog.Warn("can't join group", "group", addr.IP, "err", err)
		}
	}
}
