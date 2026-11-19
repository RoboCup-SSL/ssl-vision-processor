package referee

import (
	"net"
	"testing"
	"time"
)

func TestSourceFollowsOneGameControllerUntilItGoesSilent(t *testing.T) {
	var s Source

	a, b := net.ParseIP("10.0.0.4"), net.ParseIP("10.0.0.5")
	t0 := time.Unix(0, 0)

	steps := []struct {
		ip    net.IP
		at    time.Duration
		wantA bool
	}{
		{a, 0, true},                                     // the first heard is followed
		{b, 100 * time.Millisecond, false},               // a second, while the first sends, isn't
		{a, 200 * time.Millisecond, true},                // the first keeps being followed
		{b, 200*time.Millisecond + SourceTimeout, false}, // silent for exactly the timeout: not yet
		{b, 201*time.Millisecond + SourceTimeout, true},  // past it: the second takes over
		{a, 300*time.Millisecond + SourceTimeout, false}, // and the first is now the extra one
	}

	for i, step := range steps {
		if got := s.Accept(step.ip, t0.Add(step.at)); got != step.wantA {
			t.Errorf("step %d: Accept(%s) = %v, want %v", i, step.ip, got, step.wantA)
		}
	}
}
