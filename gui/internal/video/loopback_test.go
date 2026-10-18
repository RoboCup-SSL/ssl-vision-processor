package video

// A camera's RTP stream over real loopback multicast, relayed to a viewer
// over /ws/video: what source_test's fakeEndpoint stands in for. Skipped
// unless loopback multicast is on (scripts/lo-multicast.sh on).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast/mctest"
	"github.com/gorilla/websocket"
)

func TestViewerGetsALoopbackStreamAsFMP4(t *testing.T) {
	lo := mctest.RequireLoopback(t)
	group := mctest.Group(t)

	m := NewManager(false)
	m.Configure(map[int]Stream{0: {Active: true, Address: group}}, lo)

	mux := http.NewServeMux()
	mux.Handle("GET /ws/video/{id}", HandleWebSocket(m, nil))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// The source opens its socket for its first viewer.
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/video/0", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Replay the clip as RTP until the viewer has what a player needs.
	clip := loadClip(t, "320x240.h264")
	sender := mctest.NewSender(t)
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)

	go func() {
		p := &packetizer{ssrc: 0x5eed}

		for ctx.Err() == nil {
			for _, au := range clip {
				for _, pkt := range p.packets(au) {
					data, err := pkt.Marshal()
					if err != nil || ctx.Err() != nil {
						return
					}

					_ = sender.Send(group, data)
				}

				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}

	var format struct {
		Type   string
		Width  int
		Height int
		Codec  string
	}

	binaries := 0

	for format.Type != "format" || binaries < 2 {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("after format %+v and %d segments: %v", format, binaries, err)
		}

		if kind == websocket.BinaryMessage {
			binaries++

			continue
		}

		var head struct{ Type string }
		if json.Unmarshal(data, &head) == nil && head.Type == "format" {
			if err := json.Unmarshal(data, &format); err != nil {
				t.Fatal(err)
			}
		}
	}

	if format.Width != 320 || format.Height != 240 || !strings.HasPrefix(format.Codec, "avc1.") {
		t.Errorf("format = %+v, want 320x240 H.264", format)
	}
}
