// Package mctest sends and receives real multicast over the loopback
// interface, for tests of the host's sockets. Loopback multicast is off by
// default on Ubuntu: RequireLoopback skips a test until it's on
// (scripts/lo-multicast.sh on), and fails it under CI, where the workflow
// turns it on.
package mctest

import (
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"golang.org/x/net/ipv4"
)

// RequireLoopback returns the loopback interface, as the only one a socket
// should use, or skips t if it can't carry multicast.
func RequireLoopback(t testing.TB) []multicast.Interface {
	t.Helper()

	name, enabled, ok := multicast.LoopbackMulticast()

	switch {
	case !ok:
		t.Skip("no loopback interface")
	case !enabled && os.Getenv("CI") != "":
		t.Fatalf("multicast is off on %s; the CI workflow should run scripts/lo-multicast.sh on", name)
	case !enabled:
		t.Skipf("multicast is off on %s; enable it with scripts/lo-multicast.sh on", name)
	}

	for _, i := range multicast.ListInterfaces() {
		if i.Name == name {
			i.Used = true

			return []multicast.Interface{i}
		}
	}

	t.Fatalf("loopback %s not in ListInterfaces", name)

	return nil
}

// Group is a fresh administratively scoped group and a free port, so tests
// running at once don't hear each other.
func Group(t testing.TB) string {
	t.Helper()

	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	return net.JoinHostPort(fmt.Sprintf("239.255.%d.%d", rand.IntN(254)+1, rand.IntN(254)+1), strconv.Itoa(port))
}

// Sender is a loopback multicast sender, safe to use from any goroutine.
type Sender struct {
	pc *ipv4.PacketConn
}

// NewSender opens a sender, closed when t ends.
func NewSender(t testing.TB) *Sender {
	t.Helper()

	s, closeFn := openSender(t)
	t.Cleanup(closeFn)

	return s
}

func openSender(t testing.TB) (*Sender, func()) {
	t.Helper()

	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	pc := ipv4.NewPacketConn(conn)
	if err := pc.SetMulticastInterface(loopback(t)); err != nil {
		conn.Close()
		t.Fatal(err)
	}

	if err := pc.SetMulticastLoopback(true); err != nil {
		conn.Close()
		t.Fatal(err)
	}

	return &Sender{pc: pc}, func() { conn.Close() }
}

// Send sends data to group.
func (s *Sender) Send(group string, data []byte) error {
	dst, err := net.ResolveUDPAddr("udp4", group)
	if err != nil {
		return err
	}

	_, err = s.pc.WriteTo(data, nil, dst)

	return err
}

// Send sends data to group over loopback, once.
func Send(t testing.TB, group string, data []byte) {
	t.Helper()

	s, closeFn := openSender(t)
	defer closeFn()

	if err := s.Send(group, data); err != nil {
		t.Fatal(err)
	}
}

// SendUntil sends data to group every 50 ms until done reports true, failing
// t after timeout. A socket being tested opens asynchronously, so a single
// send could arrive before it's listening.
func SendUntil(t testing.TB, group string, data []byte, timeout time.Duration, done func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("nothing received on %s after %v", group, timeout)
		}

		Send(t, group, data)
		time.Sleep(50 * time.Millisecond)
	}
}

// Listen joins group on loopback and returns what arrives, until t ends.
func Listen(t testing.TB, group string) <-chan []byte {
	t.Helper()

	addr, err := net.ResolveUDPAddr("udp4", group)
	if err != nil {
		t.Fatal(err)
	}

	conn, err := net.ListenMulticastUDP("udp4", loopback(t), addr)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { conn.Close() })

	out := make(chan []byte, 16)

	go func() {
		buf := make([]byte, 65536)

		for {
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			select {
			case out <- append([]byte(nil), buf[:n]...):
			default:
			}
		}
	}()

	return out
}

func loopback(t testing.TB) *net.Interface {
	t.Helper()

	name, _, _ := multicast.LoopbackMulticast()

	ifi, err := net.InterfaceByName(name)
	if err != nil {
		t.Fatal(err)
	}

	return ifi
}
