// Package hub is an in-process topic pub/sub for the browser-facing
// WebSocket: Publish pushes JSON-encoded data onto a topic, Subscribe hands
// back a size-1 channel of it. A slow subscriber sees only the latest value.
package hub

import "sync"

// Hub fans published data out to subscribers by topic.
type Hub struct {
	mu   sync.Mutex
	subs map[string]map[chan []byte]struct{}
}

// New returns an empty Hub.
func New() *Hub {
	return &Hub{subs: make(map[string]map[chan []byte]struct{})}
}

// Subscribe returns a channel of future values published to topic, and an
// unsubscribe function the caller must call exactly once when done.
func (h *Hub) Subscribe(topic string) (<-chan []byte, func()) {
	ch := make(chan []byte, 1)

	h.mu.Lock()
	if h.subs[topic] == nil {
		h.subs[topic] = make(map[chan []byte]struct{})
	}
	h.subs[topic][ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		delete(h.subs[topic], ch)
		if len(h.subs[topic]) == 0 {
			delete(h.subs, topic)
		}

		// Closing under the same lock that removes ch from subs means Publish
		// can never be mid-send to a channel this is about to close.
		close(ch)
	}

	return ch, unsubscribe
}

// Publish sends data to every current subscriber of topic. A subscriber whose
// channel is still holding an unread value has that value replaced, not
// queued -- matching the size-1, drop-stale semantics the rest of the host
// uses for anything published faster than a slow reader drains it.
func (h *Hub) Publish(topic string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subs[topic] {
		sendLatest(ch, data)
	}
}

// sendLatest sends data on ch, dropping whatever unread value is already
// queued rather than blocking or letting a backlog grow -- the drop-stale,
// keep-latest guarantee this package makes wherever it multiplexes onto a
// size-limited channel: a topic's own per-subscriber channel here, and (see
// websocket.go's forwardTopic) a connection's shared outbound channel.
func sendLatest(ch chan []byte, data []byte) {
	select {
	case ch <- data:
	default:
		select {
		case <-ch:
		default:
		}

		select {
		case ch <- data:
		default:
		}
	}
}
