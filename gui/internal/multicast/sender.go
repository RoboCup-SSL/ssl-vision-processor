package multicast

import (
	"log/slog"
	"net"
	"slices"
	"sync"
)

// sender writes datagrams to one address from every interface the host uses.
// It replaces sslnet's UdpClient, which sends from every interface regardless
// of the skip list.
type sender struct {
	mu    sync.Mutex
	conns []*net.UDPConn
}

// openSender dials address from each used interface it makes sense to send
// from: all of them for a multicast group or the limited broadcast address,
// only the matching one for a subnet broadcast, and the kernel's choice for
// anything else (unicast).
func openSender(address string, ifaces []Interface) *sender {
	s := &sender{}

	raddr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		slog.Warn("can't send", "address", address, "err", err)

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
		slog.Warn("can't send", "to", raddr, "from", laddr, "err", err)

		return
	}

	s.conns = append(s.conns, conn)
}

// send writes data on every connection, dropping any that fail (an interface
// that went away) until the next reopen.
func (s *sender) send(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conns = slices.DeleteFunc(s.conns, func(conn *net.UDPConn) bool {
		if _, err := conn.Write(data); err != nil {
			slog.Warn("send failed, dropping this interface until reopened", "from", conn.LocalAddr(), "to", conn.RemoteAddr(), "err", err)
			_ = conn.Close()

			return true
		}

		return false
	})
}

func (s *sender) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range s.conns {
		_ = conn.Close()
	}

	s.conns = nil
}
