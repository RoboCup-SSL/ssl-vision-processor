package multicast

import (
	"net"
	"slices"
	"testing"
	"time"
)

func TestArrivalsFindASenderOnSeveralInterfaces(t *testing.T) {
	a := arrivals{}
	vp := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 7), Port: 10006}
	gc := &net.UDPAddr{IP: net.IPv4(10, 0, 0, 8), Port: 10003}
	now := time.Now()
	warnings := 0

	for i := range 10 {
		now = now.Add(10 * time.Millisecond)

		for _, iface := range []string{"eth1", "eth0"} {
			if got := a.note(vp, iface, now); got != nil {
				warnings++

				if !slices.Equal(got, []string{"eth0", "eth1"}) {
					t.Errorf("warned about %v", got)
				}
			}
		}

		a.note(gc, "eth0", now)

		if i == 0 {
			a.note(gc, "eth1", now) // one stray
		}
	}

	if warnings != 1 {
		t.Errorf("warned %d times, want once", warnings)
	}

	got := a.duplicates(now)
	if len(got) != 1 || got[0].Address != "10.0.0.7:10006" || !slices.Equal(got[0].Interfaces, []string{"eth0", "eth1"}) {
		t.Fatalf("duplicates = %+v, want only the sender on both", got)
	}

	// eth1 stops: no longer a duplicate, and a return warns again.
	later := now.Add(2 * ReceivingWindow)
	a.note(vp, "eth0", later)

	if got := a.duplicates(later); len(got) != 0 {
		t.Errorf("still duplicates = %+v", got)
	}

	for range arrivalCount {
		a.note(vp, "eth0", later)

		if a.note(vp, "eth1", later) != nil {
			warnings++
		}
	}

	if warnings != 2 {
		t.Errorf("%d warnings after it came back, want 2", warnings)
	}
}

// Many short-lived senders don't grow the table for ever.
func TestArrivalsForgetSilentSenders(t *testing.T) {
	a := arrivals{}
	now := time.Now()

	for port := range 10 * maxArrivals {
		now = now.Add(time.Second)
		a.note(&net.UDPAddr{IP: net.IPv4(10, 0, 0, 7), Port: port + 1}, "eth0", now)
	}

	if len(a) > maxArrivals {
		t.Errorf("%d senders kept, want at most %d", len(a), maxArrivals)
	}
}
