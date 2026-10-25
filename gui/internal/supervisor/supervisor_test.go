package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

// TestHelperProcess is the fake vision_processor the tests run: this test
// binary again, with the config path last. It logs each start beside the
// config and behaves as the config's text says: "crash" exits 1 at once,
// "ignore_term" ignores SIGTERM, anything else exits cleanly on SIGTERM.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("SUPERVISOR_HELPER") != "1" {
		return
	}

	path := os.Args[len(os.Args)-1]
	data, _ := os.ReadFile(path)
	mode := string(data)

	f, _ := os.OpenFile(path+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	fmt.Fprintf(f, "start %d\n", os.Getpid())
	f.Close()

	fmt.Println("hello from", path)

	switch {
	case strings.Contains(mode, "crash"):
		fmt.Fprintln(os.Stderr, "crashing")
		os.Exit(1)
	case strings.Contains(mode, "ignore_term"):
		signal.Ignore(syscall.SIGTERM)
		time.Sleep(30 * time.Second)
	default:
		term := make(chan os.Signal, 1)
		signal.Notify(term, syscall.SIGTERM)

		select {
		case <-term:
		case <-time.After(30 * time.Second):
		}
	}

	os.Exit(0)
}

type harness struct {
	t   *testing.T
	sup *Supervisor
	dir string
	doc config.Document
}

func newHarness(t *testing.T, opts Options) *harness {
	t.Helper()
	t.Setenv("SUPERVISOR_HELPER", "1")

	opts.Command = []string{os.Args[0], "-test.run=^TestHelperProcess$", "--"}
	if opts.Debounce == 0 {
		opts.Debounce = 50 * time.Millisecond
	}

	if opts.StopTimeout == 0 {
		opts.StopTimeout = 2 * time.Second
	}

	if opts.RetryDelay == 0 {
		opts.RetryDelay = 20 * time.Millisecond
	}

	if opts.MaxRetryDelay == 0 {
		opts.MaxRetryDelay = 100 * time.Millisecond
	}

	h := &harness{t: t, sup: New(opts), dir: t.TempDir()}
	h.doc = config.Document{Version: config.Version, Cameras: []config.Camera{}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- h.sup.Run(ctx) }()

	t.Cleanup(func() {
		cancel()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("Run didn't return after cancel")
		}
	})

	return h
}

// camera adds or replaces camera id, writing mode as its config file's text.
func (h *harness) camera(id int, mode string, cfg config.Settings) string {
	h.t.Helper()

	path := filepath.Join(h.dir, fmt.Sprintf("cam%d.yml", id))
	if err := os.WriteFile(path, []byte(mode), 0o644); err != nil {
		h.t.Fatal(err)
	}

	cams := []config.Camera{}

	for _, c := range h.doc.Cameras {
		if c.CameraID != id {
			cams = append(cams, c)
		}
	}

	h.doc.Cameras = append(cams, config.Camera{CameraID: id, ConfigPath: path, Config: cfg})
	h.sup.Sync(h.doc)

	return path
}

func (h *harness) remove(id int) {
	cams := []config.Camera{}

	for _, c := range h.doc.Cameras {
		if c.CameraID != id {
			cams = append(cams, c)
		}
	}

	h.doc.Cameras = cams
	h.sup.Sync(h.doc)
}

func starts(path string) int {
	data, _ := os.ReadFile(path + ".log")

	return strings.Count(string(data), "start ")
}

func (h *harness) status(id int) Status {
	for _, s := range h.sup.Statuses() {
		if s.CameraID == id {
			return s
		}
	}

	return Status{}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func exposure(v float64) config.Settings {
	return config.Settings{"camera": map[string]any{"exposure": v}}
}

func TestRestartsOnStartupSettingOnly(t *testing.T) {
	h := newHarness(t, Options{})
	path := h.camera(0, "", exposure(1))

	eventually(t, "first start", func() bool { return starts(path) == 1 && h.status(0).State == StateRunning })

	if out := h.status(0).Output; len(out) == 0 || !strings.Contains(out[0], "hello") {
		t.Errorf("Output = %q, want the process's hello", out)
	}

	h.camera(0, "", config.Settings{"camera": map[string]any{"exposure": 1.0}, "thresholds": map[string]any{"score": 7}})
	time.Sleep(200 * time.Millisecond)

	if n := starts(path); n != 1 {
		t.Fatalf("a live setting restarted it: %d starts", n)
	}

	h.camera(0, "", exposure(2))

	if keys := h.status(0).PendingKeys; len(keys) != 1 || keys[0] != "camera.exposure" {
		t.Errorf("PendingKeys = %v, want [camera.exposure]", keys)
	}

	eventually(t, "restart", func() bool { return starts(path) == 2 && h.status(0).State == StateRunning })

	st := h.status(0)
	if st.Restarts != 1 || len(st.PendingKeys) != 0 || !strings.HasPrefix(st.LastExit, "stopped by the host") {
		t.Errorf("after restart: %+v", st)
	}
}

func TestDebounceRestartsOnce(t *testing.T) {
	h := newHarness(t, Options{Debounce: 150 * time.Millisecond})
	path := h.camera(0, "", exposure(1))

	eventually(t, "first start", func() bool { return h.status(0).State == StateRunning })

	for v := 2; v < 8; v++ {
		h.camera(0, "", exposure(float64(v)))
		time.Sleep(30 * time.Millisecond)
	}

	eventually(t, "restart", func() bool { return starts(path) == 2 && h.status(0).State == StateRunning })
	time.Sleep(300 * time.Millisecond)

	if n := starts(path); n != 2 {
		t.Errorf("%d starts, want 2", n)
	}
}

func TestRevertedChangeCancelsRestart(t *testing.T) {
	h := newHarness(t, Options{Debounce: 150 * time.Millisecond})
	path := h.camera(0, "", exposure(1))

	eventually(t, "first start", func() bool { return h.status(0).State == StateRunning })

	h.camera(0, "", exposure(2))
	h.camera(0, "", exposure(1))
	time.Sleep(300 * time.Millisecond)

	if n := starts(path); n != 1 {
		t.Errorf("%d starts, want 1", n)
	}
}

func TestKillsWhatIgnoresSIGTERM(t *testing.T) {
	h := newHarness(t, Options{StopTimeout: 200 * time.Millisecond})
	path := h.camera(0, "ignore_term", nil)

	eventually(t, "first start", func() bool { return h.status(0).State == StateRunning })

	if err := h.sup.Restart(0); err != nil {
		t.Fatal(err)
	}

	eventually(t, "restart", func() bool { return starts(path) == 2 && h.status(0).State == StateRunning })

	if exit := h.status(0).LastExit; !strings.Contains(exit, "killed") {
		t.Errorf("LastExit = %q, want killed", exit)
	}
}

func TestCrashLoopBacksOff(t *testing.T) {
	h := newHarness(t, Options{})
	path := h.camera(0, "crash", nil)

	eventually(t, "crashes", func() bool { return h.status(0).Crashes >= 3 })

	st := h.status(0)
	if st.LastExit != "exit status 1" || st.State == StateRunning && st.PID == 0 {
		t.Errorf("after crashes: %+v", st)
	}

	if !strings.Contains(strings.Join(st.Output, "\n"), "crashing") {
		t.Errorf("Output = %q, want stderr too", st.Output)
	}

	// Fixed, it runs, and a run that ends cleanly isn't a crash.
	h.camera(0, "", exposure(1))
	eventually(t, "running", func() bool { return h.status(0).State == StateRunning && starts(path) > 3 })
}

func TestRemovedCameraStops(t *testing.T) {
	h := newHarness(t, Options{})
	h.camera(0, "", nil)
	h.camera(1, "", nil)

	eventually(t, "both running", func() bool {
		return h.status(0).State == StateRunning && h.status(1).State == StateRunning
	})

	pid := h.status(1).PID
	h.remove(1)

	if got := len(h.sup.Statuses()); got != 1 {
		t.Errorf("%d statuses, want 1", got)
	}

	eventually(t, "process gone", func() bool { return syscall.Kill(pid, 0) != nil })
}

func TestRestartUnknownCamera(t *testing.T) {
	h := newHarness(t, Options{})
	h.camera(0, "", nil)

	if err := h.sup.Restart(3); !errors.Is(err, ErrNotSupervised) {
		t.Errorf("Restart(3) = %v, want ErrNotSupervised", err)
	}

	var none *Supervisor
	if err := none.Restart(0); !errors.Is(err, ErrNotSupervised) {
		t.Errorf("nil Restart = %v", err)
	}

	if got := none.Statuses(); got == nil || len(got) != 0 {
		t.Errorf("nil Statuses = %#v, want empty", got)
	}
}

func TestStopsAllOnCancel(t *testing.T) {
	t.Setenv("SUPERVISOR_HELPER", "1")

	dir := t.TempDir()
	path := filepath.Join(dir, "cam0.yml")
	_ = os.WriteFile(path, nil, 0o644)

	var mu sync.Mutex
	changes := 0
	sup := New(Options{
		Command:  []string{os.Args[0], "-test.run=^TestHelperProcess$", "--"},
		OnChange: func() { mu.Lock(); changes++; mu.Unlock() },
	})

	// Synced before Run, as the host does: nothing starts until Run.
	sup.Sync(config.Document{Version: config.Version, Cameras: []config.Camera{{CameraID: 0, ConfigPath: path}}})

	if starts(path) != 0 {
		t.Fatal("started before Run")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- sup.Run(ctx) }()

	var pid int

	eventually(t, "running", func() bool {
		st := sup.Statuses()
		if len(st) == 1 && st[0].State == StateRunning {
			pid = st[0].PID

			return true
		}

		return false
	})

	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run didn't return")
	}

	if syscall.Kill(pid, 0) == nil {
		t.Error("process still running after Run returned")
	}

	if st := sup.Statuses(); st[0].State != StateStopped || !strings.Contains(st[0].LastExit, "exited cleanly") {
		t.Errorf("after cancel: %+v", st[0])
	}

	mu.Lock()
	defer mu.Unlock()

	if changes == 0 {
		t.Error("OnChange never ran")
	}
}
