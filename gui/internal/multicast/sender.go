package multicast

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"sync"
)

// sender writes datagrams to one address from every interface the host uses.
type sender struct {
	mu    sync.Mutex
	conns []*net.UDPConn
	// problem is why some send socket is missing: one couldn't be opened,
	// or one failed and was dropped (an interface going away). The endpoint
	// sees it through health and reopens.
	problem error
}

// openSender dials address from each used interface it makes sense to send
// from: all of them for a multicast group or the limited broadcast address,
// only the matching one for a subnet broadcast, and the kernel's choice for
// anything else (unicast). A multicast group with no used interface gets no
// socket at all rather than the kernel's choice, which is what "skip every
// interface" means, and which would fail anyway while the network is down.
func openSender(address string, ifaces []Interface) *sender {
	s := &sender{}

	raddr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		s.problem = err

		return s
	}

	used := slices.DeleteFunc(slices.Clone(ifaces), func(i Interface) bool {
		return !i.Used || i.Address == ""
	})

	var from []string

	switch {
	case raddr.IP.IsMulticast() || raddr.IP.Equal(net.IPv4bcast):
		for _, i := range used {
			from = append(from, i.Address)
		}

		if len(from) == 0 {
			s.problem = errNoInterface

			return s
		}
	default:
		for _, i := range used {
			if i.Broadcast == raddr.IP.String() {
				from = append(from, i.Address)
			}
		}
	}

	if len(from) == 0 {
		s.dial(nil, raddr)

		return s
	}

	for _, ip := range from {
		s.dial(&net.UDPAddr{IP: net.ParseIP(ip)}, raddr)
	}

	return s
}

func (s *sender) dial(laddr, raddr *net.UDPAddr) {
	conn, err := net.DialUDP("udp4", laddr, raddr)
	if err != nil {
		s.problem = errors.Join(s.problem, fmt.Errorf("can't send from %v: %w", laddr, err))

		return
	}

	s.conns = append(s.conns, conn)
}

// send writes data on every connection, dropping any that fail until the
// endpoint reopens (it notices through health).
func (s *sender) send(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conns = slices.DeleteFunc(s.conns, func(conn *net.UDPConn) bool {
		if _, err := conn.Write(data); err != nil {
			slog.Debug("send failed, dropping this socket until reopened", "from", conn.LocalAddr(), "to", conn.RemoteAddr(), "err", err)
			s.problem = errors.Join(s.problem, fmt.Errorf("lost send socket %v: %w", conn.LocalAddr(), err))
			_ = conn.Close()

			return true
		}

		return false
	})
}

// health is "" while every send socket that should be open is.
func (s *sender) health() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.problem == nil {
		return ""
	}

	return s.problem.Error()
}

func (s *sender) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range s.conns {
		_ = conn.Close()
	}

	s.conns = nil
}
