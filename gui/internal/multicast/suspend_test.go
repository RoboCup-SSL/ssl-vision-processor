package multicast

import (
	"testing"
	"time"
)

func TestSuspendDetector(t *testing.T) {
	offset := time.Duration(0)
	d := newSuspendDetector(func() time.Duration { return offset })

	if slept := d.Check(); slept != 0 {
		t.Fatalf("no time passed, got %v", slept)
	}

	offset += 300 * time.Millisecond // jitter, not a suspend
	if slept := d.Check(); slept != 0 {
		t.Fatalf("jitter reported as a %v suspend", slept)
	}

	offset += 13*time.Hour + 44*time.Minute
	if slept := d.Check(); slept != 13*time.Hour+44*time.Minute {
		t.Fatalf("slept = %v, want 13h44m", slept)
	}

	if slept := d.Check(); slept != 0 {
		t.Fatalf("the same suspend reported twice: %v", slept)
	}
}

func TestSuspendOffsetIsStableWhileAwake(t *testing.T) {
	d := NewSuspendDetector()
	time.Sleep(50 * time.Millisecond)

	if slept := d.Check(); slept != 0 {
		t.Fatalf("reported a %v suspend while awake", slept)
	}
}
