package peers

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func testNames() *Names {
	n := NewNames()
	n.lookup = func(_ context.Context, ip string) ([]string, error) {
		if ip == "10.0.0.1" {
			return []string{"vp-1.lab."}, nil
		}

		return nil, nil
	}

	return n
}

func addr(s string) *net.UDPAddr {
	a, err := net.ResolveUDPAddr("udp4", s)
	if err != nil {
		panic(err)
	}

	return a
}

// captureLogs collects slog output until the test ends.
func captureLogs(t *testing.T) func() string {
	t.Helper()

	var (
		mu  sync.Mutex
		buf bytes.Buffer
	)

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		return buf.Write(p)
	}), nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return func() string {
		mu.Lock()
		defer mu.Unlock()

		return buf.String()
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestTwoSendersConflictAndResolve(t *testing.T) {
	logs := captureLogs(t)
	w := NewWatch(Options{Kind: KindCalibration, Window: time.Minute}, testNames())
	cam := 0
	now := time.Now()

	w.Record("camera 0 calibration", &cam, addr("10.0.0.1:10006"), now)

	if c := w.Conflicts(now); len(c) != 0 {
		t.Fatalf("one sender is a conflict: %v", c)
	}

	w.Record("camera 0 calibration", &cam, addr("10.0.0.2:41234"), now)

	c := w.Conflicts(now)
	if len(c) != 1 || len(c[0].Senders) != 2 || *c[0].CameraID != 0 || c[0].Kind != KindCalibration {
		t.Fatalf("Conflicts = %+v, want camera 0 from 2 senders", c)
	}

	if got := c[0].Senders[0].Address; got != "10.0.0.1:10006" {
		t.Errorf("first sender = %q", got)
	}

	eventually(t, "conflict logged with addresses and hostname", func() bool {
		out := logs()

		return strings.Contains(out, "several senders claim the same thing") &&
			strings.Contains(out, "10.0.0.1:10006 (vp-1.lab)") && strings.Contains(out, "10.0.0.2:41234")
	})

	// The second sender goes quiet past the window.
	later := now.Add(2 * time.Minute)
	w.Record("camera 0 calibration", &cam, addr("10.0.0.1:10006"), later)

	if c := w.Conflicts(later); len(c) != 0 {
		t.Fatalf("still a conflict after the second sender stopped: %v", c)
	}

	eventually(t, "resolution logged", func() bool { return strings.Contains(logs(), "sender conflict resolved") })
}

func TestMinCountIgnoresOneOffs(t *testing.T) {
	w := NewWatch(Options{Kind: KindGeometry, Window: 10 * time.Second, MinCount: 3, WithSelf: true}, testNames())
	now := time.Now()

	// A vision_processor sending the field back once is not a publisher.
	w.Record("field geometry", nil, addr("10.0.0.1:10006"), now)

	if c := w.Conflicts(now); len(c) != 0 {
		t.Fatalf("one-off counted: %v", c)
	}

	// Another host publishing every second is, even alone: WithSelf.
	for i := range 3 {
		w.Record("field geometry", nil, addr("10.0.0.9:50000"), now.Add(time.Duration(i)*time.Second))
	}

	c := w.Conflicts(now.Add(2 * time.Second))
	if len(c) != 1 || len(c[0].Senders) != 1 || c[0].Senders[0].Address != "10.0.0.9:50000" {
		t.Fatalf("Conflicts = %+v, want the other publisher", c)
	}
}

func TestSameIPDifferentPortsAreDifferentSenders(t *testing.T) {
	w := NewWatch(Options{Kind: KindReferee, Window: time.Second}, testNames())
	now := time.Now()

	// Two programs on one machine differ only in port.
	w.Record("game controller", nil, addr("127.0.0.1:40000"), now)
	w.Record("game controller", nil, addr("127.0.0.1:40001"), now)

	c := w.Conflicts(now)
	if len(c) != 1 || len(c[0].Senders) != 2 {
		t.Fatalf("Conflicts = %+v, want 2 senders on one IP", c)
	}

	if !c[0].Senders[0].Local {
		t.Error("127.0.0.1 not marked as this machine")
	}
}

func TestDescribe(t *testing.T) {
	n := testNames()

	if got := n.Describe(addr("10.0.0.1:10006")).String(); got != "10.0.0.1:10006 (vp-1.lab)" {
		t.Errorf("Describe = %q", got)
	}

	if got := n.Describe(addr("10.0.0.2:5")).String(); got != "10.0.0.2:5" {
		t.Errorf("Describe without a name = %q", got)
	}

	if got := n.Describe(addr("127.0.0.1:1")).String(); !strings.Contains(got, "this machine") {
		t.Errorf("Describe of loopback = %q", got)
	}
}
