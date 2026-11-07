package hub

import (
	"sync"
	"testing"
	"time"
)

func TestPublishToNoSubscribersDoesNotPanic(t *testing.T) {
	h := New()
	h.Publish("nobody-listening", []byte("x"))
}

func TestSubscribeReceivesPublishedValue(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("geometry")
	defer unsubscribe()

	h.Publish("geometry", []byte(`{"a":1}`))

	select {
	case got := <-ch:
		if string(got) != `{"a":1}` {
			t.Errorf("got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive published value")
	}
}

func TestPublishReachesEverySubscriber(t *testing.T) {
	h := New()
	ch1, unsub1 := h.Subscribe("geometry")
	defer unsub1()
	ch2, unsub2 := h.Subscribe("geometry")
	defer unsub2()

	h.Publish("geometry", []byte("v"))

	for _, ch := range []<-chan []byte{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("a subscriber did not receive the value")
		}
	}
}

func TestPublishIsIsolatedByTopic(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("a")
	defer unsubscribe()

	h.Publish("b", []byte("for b"))

	select {
	case got := <-ch:
		t.Fatalf("subscriber to a received %q, a publish to b", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// unsubscribe closes the channel, giving a forwarding goroutine reading it a
// clean stop signal instead of one that just goes quiet.
func TestUnsubscribeClosesTheChannel(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("geometry")
	unsubscribe()

	select {
	case got, ok := <-ch:
		if ok {
			t.Fatalf("channel not closed, received %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("read from closed channel should not block")
	}
}

// A slow subscriber sees only the latest value, matching the pattern used
// elsewhere in the host (Geometry.Encoded, the Python bus this replaces).
func TestSlowSubscriberSeesOnlyLatestValue(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("geometry")
	defer unsubscribe()

	h.Publish("geometry", []byte("first"))
	h.Publish("geometry", []byte("second"))

	select {
	case got := <-ch:
		if string(got) != "second" {
			t.Fatalf("got %q, want the latest value only", got)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive a value")
	}

	select {
	case got := <-ch:
		t.Fatalf("received a second value %q, want the dropped first one gone for good", got)
	case <-time.After(50 * time.Millisecond):
	}
}

// forwardTopic (websocket.go) shares one connection-wide channel across every
// topic a client subscribes to, unlike Publish's per-subscriber channel --
// sendLatest is what both rely on for the same drop-stale, keep-latest
// guarantee, so it's worth testing directly against a channel with room for
// more than one queued value, not just the size-1 case Publish/Subscribe use.
func TestSendLatestDropsTheOldestQueuedValueWhenFull(t *testing.T) {
	ch := make(chan []byte, 2)

	sendLatest(ch, []byte("a"))
	sendLatest(ch, []byte("b"))
	sendLatest(ch, []byte("c")) // full (a, b): must drop "a", never silently drop "c" itself

	var got []string
	for len(ch) > 0 {
		got = append(got, string(<-ch))
	}

	want := []string{"b", "c"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("queue ended up %v, want %v -- the newest value must never be the one dropped", got, want)
	}
}

func TestConcurrentSubscribePublishUnsubscribe(t *testing.T) {
	h := New()

	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 50 {
				ch, unsubscribe := h.Subscribe("geometry")
				h.Publish("geometry", []byte("v"))

				select {
				case <-ch:
				default:
				}

				unsubscribe()
			}
		}()
	}

	for range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range 50 {
				h.Publish("geometry", []byte("v"))
			}
		}()
	}

	wg.Wait()
}
