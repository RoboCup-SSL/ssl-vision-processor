package main

import (
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/alerts"
)

// Two game controllers and two senders on one camera's video, heard by a
// real host's watchers: warnings in a lab, errors once the field is marked
// a competition field, and warnings again when it isn't.
func TestDualGameControllerAndVideoSeverityFollowsCompetitionMode(t *testing.T) {
	h, srv := testHost(t)

	now := time.Now()
	for i := range 3 {
		at := now.Add(time.Duration(i) * 100 * time.Millisecond)
		h.sockets.peers.referee.Record("game controller", nil, &net.UDPAddr{IP: net.IPv4(10, 0, 0, 4), Port: 10003}, at)
		h.sockets.peers.referee.Record("game controller", nil, &net.UDPAddr{IP: net.IPv4(10, 0, 0, 5), Port: 10003}, at)
	}

	for range 10 {
		h.sockets.peers.videoPacket(1, &net.UDPAddr{IP: net.IPv4(10, 0, 0, 1), Port: 50000})
		h.sockets.peers.videoPacket(1, &net.UDPAddr{IP: net.IPv4(10, 0, 0, 2), Port: 50000})
	}

	severities := func() map[string]string {
		t.Helper()

		resp, err := http.Get(srv.URL + "/api/alerts")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var list []alerts.Alert
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatal(err)
		}

		out := map[string]string{}
		for _, a := range list {
			if a.Code == alerts.CodeSenderConflict {
				out[a.Params["kind"].(string)] = a.Severity
			}
		}

		return out
	}

	check := func(mode, want string) {
		t.Helper()

		got := severities()
		for _, kind := range []string{"referee", "video"} {
			if got[kind] != want {
				t.Errorf("%s: %s conflict = %q, want %q (all: %v)", mode, kind, got[kind], want, got)
			}
		}
	}

	check("lab", alerts.Warning)

	if r := patch(t, srv, `{"competitionField":true}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("competition patch = %d", r.StatusCode)
	}

	check("competition field", alerts.Error)

	if r := patch(t, srv, `{"competitionField":false}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("lab patch = %d", r.StatusCode)
	}

	check("back in the lab", alerts.Warning)
}
