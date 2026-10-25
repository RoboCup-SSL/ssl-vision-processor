package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

// TestFakeVisionProcessor stands in for vision_processor under -vp: it notes
// each start beside its config and runs until SIGTERM.
func TestFakeVisionProcessor(t *testing.T) {
	if os.Getenv("FAKE_VISION_PROCESSOR") != "1" {
		return
	}

	path := os.Args[len(os.Args)-1]

	f, _ := os.OpenFile(path+".starts", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	f.WriteString("start\n")
	f.Close()

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)

	select {
	case <-term:
	case <-time.After(30 * time.Second):
	}

	os.Exit(0)
}

// An edit to a startup-only setting through the API restarts the camera's
// vision_processor; an edit to a live one only rewrites its config.yml.
func TestEditRestartsSupervisedVisionProcessor(t *testing.T) {
	t.Setenv("FAKE_VISION_PROCESSOR", "1")

	cfg := filepath.Join(t.TempDir(), "cam0.yml")
	h, srv := testHostWith(t, hostOptions{
		imgDir:    t.TempDir(),
		vpCommand: []string{os.Args[0], "-test.run=^TestFakeVisionProcessor$", "--"},
	}, func(d *config.Document) {
		d.Cameras[0].ConfigPath = cfg
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- h.procs.Run(ctx) }()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	starts := func() int {
		data, _ := os.ReadFile(cfg + ".starts")

		return strings.Count(string(data), "start")
	}

	waitFor := func(what string, cond func() bool) {
		t.Helper()

		deadline := time.Now().Add(10 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}

			time.Sleep(20 * time.Millisecond)
		}
	}

	running := func() bool {
		st := h.procs.Statuses()

		return len(st) == 1 && st[0].State == "running"
	}

	waitFor("first start", func() bool { return starts() == 1 && running() })

	resp, err := http.Get(srv.URL + "/api/processes")
	if err != nil {
		t.Fatal(err)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	checkResponse(t, "/api/processes", body)

	if r := patch(t, srv, `{"defaults":{"thresholds":{"score":9}}}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("live patch = %d", r.StatusCode)
	}

	time.Sleep(1500 * time.Millisecond)

	if n := starts(); n != 1 {
		t.Fatalf("a live setting restarted it: %d starts", n)
	}

	if data, _ := os.ReadFile(cfg); !strings.Contains(string(data), "score: 9") {
		t.Errorf("config.yml wasn't rewritten:\n%s", data)
	}

	if r := patch(t, srv, `{"defaults":{"camera":{"exposure":7}}}`, ""); r.StatusCode != http.StatusOK {
		t.Fatalf("startup patch = %d", r.StatusCode)
	}

	waitFor("restart after edit", func() bool { return starts() == 2 && running() })

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/processes/0/restart", nil)
	if resp, err := http.DefaultClient.Do(req); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("restart: %v %v", resp, err)
	}

	waitFor("manual restart", func() bool { return starts() == 3 && running() })
}

func TestRestartUnsupervisedIs404(t *testing.T) {
	h, _ := testServer(t)

	if rec := do(t, h, http.MethodPost, "/api/processes/0/restart", nil); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
