package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/gorilla/websocket"
)

// testHost is a real host (store, geometry, hub, sockets, origin rules) on a
// copy of testdata/vision.yml, changed by edit, served over real HTTP. Its background loops
// don't run, so no multicast socket is opened.
func testHost(t *testing.T, edit ...func(*config.Document)) (*host, *httptest.Server) {
	t.Helper()

	return testHostWith(t, hostOptions{imgDir: t.TempDir()}, edit...)
}

// testHostWith is testHost with opts.
func testHostWith(t *testing.T, opts hostOptions, edit ...func(*config.Document)) (*host, *httptest.Server) {
	t.Helper()

	src, err := os.ReadFile(visionFixture)
	if err != nil {
		t.Fatal(err)
	}

	if len(edit) > 0 {
		doc, err := config.Parse(src)
		if err != nil {
			t.Fatal(err)
		}

		for _, e := range edit {
			e(&doc)
		}

		if src, err = config.Marshal(doc); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(t.TempDir(), "vision.yml")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := newHost(path, opts)
	if err != nil {
		t.Fatalf("newHost: %v", err)
	}

	srv := httptest.NewServer(h.handler)
	t.Cleanup(srv.Close)

	return h, srv
}

// patch sends a merge patch based on the current revision, as origin if set.
func patch(t *testing.T, srv *httptest.Server, body, origin string) *http.Response {
	t.Helper()

	get, err := http.Get(srv.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	get.Body.Close()

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/config", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("If-Match", get.Header.Get("ETag"))

	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	resp.Body.Close()

	return resp
}

type envelope struct {
	Topic string          `json:"topic"`
	Data  json.RawMessage `json:"data"`
}

// until reads conn's messages until done accepts one, failing after 5 s.
func until(t *testing.T, conn *websocket.Conn, what string, done func(envelope) bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}

	for {
		var msg envelope
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("waiting for %s: %v", what, err)
		}

		if done(msg) {
			return
		}
	}
}

func TestAChangeReachesWebSocketSubscribers(t *testing.T) {
	_, srv := testHost(t)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	for _, topic := range []string{configStateTopic, alertsStateTopic} {
		if err := conn.WriteJSON(map[string]string{"action": "subscribe", "topic": topic}); err != nil {
			t.Fatal(err)
		}
	}

	// Subscriptions are handled on the server's read loop; give them a moment
	// before changing anything.
	time.Sleep(50 * time.Millisecond)

	// Four regions, two cameras: regions 2 and 3 have no camera.
	if resp := patch(t, srv, `{"layout":{"cameraCount":4}}`, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH = %d", resp.StatusCode)
	}

	rev, _ := strconv.Atoi(strings.Trim(mustGet(t, srv.URL+"/api/config").Header.Get("ETag"), `"`))

	until(t, conn, "config.state at the new revision", func(m envelope) bool {
		var state config.State

		return m.Topic == configStateTopic && json.Unmarshal(m.Data, &state) == nil && state.Revision == int64(rev)
	})

	until(t, conn, "alerts.state with the uncovered regions", func(m envelope) bool {
		var list []alerts.Alert
		if m.Topic != alertsStateTopic || json.Unmarshal(m.Data, &list) != nil {
			return false
		}

		ids := map[string]bool{}
		for _, a := range list {
			ids[a.ID] = true
		}

		return ids["uncovered-2"] && ids["uncovered-3"]
	})
}

func TestCompetitionFieldSwitchesTheOriginRulesLive(t *testing.T) {
	_, srv := testHost(t)

	const lanUI = "http://192.168.1.20:3000"

	if resp := patch(t, srv, `{}`, lanUI); resp.StatusCode != http.StatusOK {
		t.Fatalf("lab: write from a LAN UI = %d, want 200", resp.StatusCode)
	}

	if resp := patch(t, srv, `{"competitionField":true}`, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("turning competitionField on = %d", resp.StatusCode)
	}

	if resp := patch(t, srv, `{}`, lanUI); resp.StatusCode != http.StatusForbidden {
		t.Errorf("competition: write from a LAN UI = %d, want 403", resp.StatusCode)
	}

	if resp := patch(t, srv, `{"competitionField":null}`, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("turning competitionField off = %d", resp.StatusCode)
	}

	if resp := patch(t, srv, `{}`, lanUI); resp.StatusCode != http.StatusOK {
		t.Errorf("back in the lab: write from a LAN UI = %d, want 200", resp.StatusCode)
	}
}

func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	resp.Body.Close()

	return resp
}
