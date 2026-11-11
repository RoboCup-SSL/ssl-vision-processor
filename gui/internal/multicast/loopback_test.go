package multicast_test

// Real sockets over loopback: what endpoint_test.go's fake opener stands in
// for. Skipped unless loopback multicast is on (scripts/lo-multicast.sh on).

import (
	"bytes"
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast/mctest"
)

// received records what an endpoint's consumer was handed.
type received struct {
	mu   sync.Mutex
	data [][]byte
	from []string
}

func (r *received) consume(data []byte, from *net.UDPAddr) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data = append(r.data, append([]byte(nil), data...))
	r.from = append(r.from, from.IP.String())

	return true
}

func (r *received) has(want []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, d := range r.data {
		if bytes.Equal(d, want) {
			return true
		}
	}

	return false
}

func run(t *testing.T, e *multicast.Endpoint) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		_ = e.Run(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func TestEndpointReceivesItsGroupOverLoopback(t *testing.T) {
	lo := mctest.RequireLoopback(t)
	group := mctest.Group(t)

	var got received

	e := multicast.NewEndpoint("test", group, lo, multicast.Options{}, got.consume)
	run(t, e)

	mctest.SendUntil(t, group, []byte("hello"), 3*time.Second, func() bool { return got.has([]byte("hello")) })

	status := e.Status(time.Now())
	if status.Heard == 0 || !status.Receiving || status.Problem != "" || status.Mode != "multicast" {
		t.Errorf("status = %+v, want receiving with no problem", status)
	}

	got.mu.Lock()
	from := got.from[0]
	got.mu.Unlock()

	if from != "127.0.0.1" {
		t.Errorf("from = %s, want 127.0.0.1", from)
	}
}

// Go binds a multicast address as 0.0.0.0, which on Linux hears every group
// joined on that port by anything on the host. The endpoint turns
// IP_MULTICAST_ALL off so it hears only its own group: cameras sharing a
// port must not mix.
func TestEndpointIgnoresOtherGroupsOnItsPort(t *testing.T) {
	lo := mctest.RequireLoopback(t)
	group := mctest.Group(t)

	host, port, _ := net.SplitHostPort(group)
	other := net.JoinHostPort(otherGroup(host), port)

	// Something else on this machine joined the other group on that port.
	mctest.Listen(t, other)

	var got received

	run(t, multicast.NewEndpoint("test", group, lo, multicast.Options{}, got.consume))

	mctest.SendUntil(t, group, []byte("mine"), 3*time.Second, func() bool { return got.has([]byte("mine")) })

	for range 5 {
		mctest.Send(t, other, []byte("theirs"))
	}

	time.Sleep(200 * time.Millisecond)

	if got.has([]byte("theirs")) {
		t.Error("endpoint heard a packet sent to another group on its port")
	}
}

func TestEndpointSendsToItsGroupOverLoopback(t *testing.T) {
	lo := mctest.RequireLoopback(t)
	group := mctest.Group(t)
	listener := mctest.Listen(t, group)

	e := multicast.NewEndpoint("test", group, lo, multicast.Options{Send: true}, func([]byte, *net.UDPAddr) bool { return false })
	run(t, e)

	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case data := <-listener:
			if bytes.Equal(data, []byte("geometry")) {
				return
			}
		case <-tick.C:
			e.Send([]byte("geometry"))
		case <-deadline:
			t.Fatalf("nothing the endpoint sent arrived on %s", group)
		}
	}
}

// otherGroup is a group next to host's, in the same scope.
func otherGroup(host string) string {
	ip := net.ParseIP(host).To4()
	ip[3] ^= 1

	return ip.String()
}
