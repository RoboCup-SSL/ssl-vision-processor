package hub

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialTestServer starts an httptest server on HandleWebSocket(h) and returns a
// connected client plus a closer.
func dialTestServer(t *testing.T, h *Hub) *websocket.Conn {
	t.Helper()

	srv := httptest.NewServer(HandleWebSocket(h, nil))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	t.Cleanup(func() { conn.Close() })

	return conn
}

func readEnvelope(t *testing.T, conn *websocket.Conn) serverMessage {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	var msg serverMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}

	return msg
}

func TestWebSocketDeliversPublishedData(t *testing.T) {
	h := New("geometry")
	conn := dialTestServer(t, h)

	if err := conn.WriteJSON(clientMessage{Action: "subscribe", Topic: "geometry"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	// Subscribe is handled by the connection's read loop; give it a moment to
	// register with the hub before publishing, or the publish can race ahead
	// of it.
	time.Sleep(50 * time.Millisecond)

	h.Publish("geometry", []byte(`{"fieldLength":9000}`))

	got := readEnvelope(t, conn)
	if got.Topic != "geometry" {
		t.Errorf("topic = %q, want geometry", got.Topic)
	}

	if string(got.Data) != `{"fieldLength":9000}` {
		t.Errorf("data = %s, want the published bytes unchanged", got.Data)
	}
}

func TestWebSocketUnsubscribeStopsDelivery(t *testing.T) {
	h := New("geometry")
	conn := dialTestServer(t, h)

	if err := conn.WriteJSON(clientMessage{Action: "subscribe", Topic: "geometry"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := conn.WriteJSON(clientMessage{Action: "unsubscribe", Topic: "geometry"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	h.Publish("geometry", []byte(`{"x":1}`))

	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

	var msg json.RawMessage
	if err := conn.ReadJSON(&msg); err == nil {
		t.Fatalf("received a message after unsubscribe: %s", msg)
	}
}

func TestWebSocketTwoConnectionsOnSameTopic(t *testing.T) {
	h := New("geometry")
	a := dialTestServer(t, h)
	b := dialTestServer(t, h)

	for _, conn := range []*websocket.Conn{a, b} {
		if err := conn.WriteJSON(clientMessage{Action: "subscribe", Topic: "geometry"}); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	h.Publish("geometry", []byte(`{"x":1}`))

	for _, conn := range []*websocket.Conn{a, b} {
		got := readEnvelope(t, conn)
		if got.Topic != "geometry" {
			t.Errorf("topic = %q, want geometry", got.Topic)
		}
	}
}

func TestWebSocketSendsTheLatestValueOnSubscribe(t *testing.T) {
	h := New("geometry")
	h.Publish("geometry", []byte(`{"x":1}`))

	conn := dialTestServer(t, h)
	if err := conn.WriteJSON(clientMessage{Action: "subscribe", Topic: "geometry"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	if got := readEnvelope(t, conn); string(got.Data) != `{"x":1}` {
		t.Errorf("data = %s, want the value published before subscribing", got.Data)
	}
}

func TestWebSocketRejectsWhatItCantAct(t *testing.T) {
	h := New("geometry")
	conn := dialTestServer(t, h)

	for raw, want := range map[string]string{
		`{"action":"subscribe","topic":"nope"}`: "unknown_topic",
		`{"action":"shout","topic":"geometry"}`: "unknown_action",
		`not json`:                              "bad_request",
	} {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(raw)); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}

		conn.SetReadDeadline(time.Now().Add(time.Second))

		var got errorMessage
		if err := conn.ReadJSON(&got); err != nil {
			t.Fatalf("%s: ReadJSON: %v", raw, err)
		}

		if got.Error != want {
			t.Errorf("%s: error = %+v, want %s", raw, got, want)
		}
	}
}
