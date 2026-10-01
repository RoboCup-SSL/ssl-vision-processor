package hub

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	writeWait  = 10 * time.Second
)

var upgrader = websocket.Upgrader{}

// clientMessage is what a browser sends: {"action": "subscribe"|"unsubscribe", "topic": "..."}.
type clientMessage struct {
	Action string `json:"action"`
	Topic  string `json:"topic"`
}

// serverMessage is what a browser receives: {"topic": "...", "data": <value>}.
// Data is a RawMessage so an already-JSON-encoded publish is embedded as-is,
// not re-encoded as a string.
type serverMessage struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

// HandleWebSocket upgrades the request and lets the client subscribe to and
// unsubscribe from hub topics for the life of the connection.
func HandleWebSocket(h *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("websocket upgrade failed", "err", err)
			return
		}

		serveConnection(h, conn)
	}
}

// serveConnection owns one connection end to end: it reads client messages
// itself, and runs exactly one other goroutine (writePump) that is the only
// caller of conn.WriteMessage -- concurrent writers on one *websocket.Conn is
// a data race, not just a bad idea.
func serveConnection(h *Hub, conn *websocket.Conn) {
	defer conn.Close()

	outbound := make(chan []byte, 16)
	unsubscribers := make(map[string]func())

	var topics sync.WaitGroup

	writeDone := make(chan struct{})
	go func() {
		writePump(conn, outbound)
		close(writeDone)
	}()

	defer func() {
		for _, unsubscribe := range unsubscribers {
			unsubscribe()
		}
		// Every forwardTopic loop above now exits (its hub channel just
		// closed), so nothing sends to outbound after this.
		topics.Wait()
		close(outbound)
		<-writeDone
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var msg clientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Action {
		case "subscribe":
			if _, already := unsubscribers[msg.Topic]; already {
				continue
			}

			ch, unsubscribe := h.Subscribe(msg.Topic)
			unsubscribers[msg.Topic] = unsubscribe

			topics.Add(1)

			go func(topic string, ch <-chan []byte) {
				defer topics.Done()
				forwardTopic(topic, ch, outbound)
			}(msg.Topic, ch)
		case "unsubscribe":
			if unsubscribe, ok := unsubscribers[msg.Topic]; ok {
				unsubscribe()
				delete(unsubscribers, msg.Topic)
			}
		default:
			slog.Warn("unknown websocket action", "action", msg.Action)
		}
	}
}

// forwardTopic relays one hub topic's values onto outbound until the topic is
// unsubscribed (the hub channel closes). outbound is shared across every
// topic this connection subscribes to, so uses the same drop-stale,
// keep-latest sendLatest as a topic's own per-subscriber channel -- a slow
// connection sees the latest value across its subscriptions, never a growing
// backlog of stale ones queued ahead of it. (Concurrent forwardTopic
// goroutines for different topics may occasionally race on which one's stale
// entry gets dropped when both find outbound full at once; sendLatest's
// channel ops are safe for that, and the guarantee that matters -- no
// unbounded backlog -- still holds either way.)
func forwardTopic(topic string, ch <-chan []byte, outbound chan []byte) {
	for data := range ch {
		envelope, err := json.Marshal(serverMessage{Topic: topic, Data: data})
		if err != nil {
			slog.Error("marshalling websocket envelope", "topic", topic, "err", err)
			continue
		}

		sendLatest(outbound, envelope)
	}
}

// writePump is the sole writer to conn: outbound messages plus a periodic
// ping, so a client that vanished without a clean close (dead wifi, closed
// laptop lid) is detected and this connection's goroutines get cleaned up
// instead of leaking for the life of the process.
func writePump(conn *websocket.Conn, outbound <-chan []byte) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-outbound:
			if !ok {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
