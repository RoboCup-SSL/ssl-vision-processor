package video

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	writeWait  = 10 * time.Second
)

var upgrader = websocket.Upgrader{}

// HandleWebSocket serves GET /ws/video/{id}?mode=full|keyframes: one camera's
// stream as JSON format and status messages and binary fMP4 segments, ready
// for a MediaSource SourceBuffer in sequence mode.
func HandleWebSocket(m *Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "camera id must be a number", http.StatusBadRequest)

			return
		}

		mode := Mode(r.URL.Query().Get("mode"))
		if mode == "" {
			mode = ModeFull
		}

		if mode != ModeFull && mode != ModeKeyframes {
			http.Error(w, `mode must be "full" or "keyframes"`, http.StatusBadRequest)

			return
		}

		// Subscribed before upgrading, so an unknown camera is a plain 404.
		ch, unsubscribe, err := m.Subscribe(id, mode)
		if errors.Is(err, ErrUnknownCamera) {
			http.Error(w, err.Error(), http.StatusNotFound)

			return
		}

		defer unsubscribe()

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Warn("video websocket upgrade failed", "err", err)

			return
		}
		defer conn.Close()

		serve(conn, ch)
	}
}

// serve writes ch to conn until either ends. The only reads are for close
// frames and pongs; a viewer that vanished without closing is caught by the
// ping deadline.
func serve(conn *websocket.Conn, ch <-chan Message) {
	gone := make(chan struct{})

	go func() {
		defer close(gone)

		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-gone:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}

			kind := websocket.TextMessage
			if msg.Binary {
				kind = websocket.BinaryMessage
			}

			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err := conn.WriteMessage(kind, msg.Data); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
