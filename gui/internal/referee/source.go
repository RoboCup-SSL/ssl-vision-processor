package referee

import (
	"log/slog"
	"net"
	"sync"
	"time"
)

// SourceTimeout is how long the followed game controller may stay silent
// before another one's packets are followed instead. A game controller
// sends many times a second, so this is a restart or a switch, not jitter.
const SourceTimeout = 3 * time.Second

// Source picks the one game controller to follow when several send on the
// same group, as in a lab or a team area: the first heard, until it goes
// silent for SourceTimeout. Without it, each sender's packets overwrite the
// team names and sides in turn. The others are still reported as a
// sender conflict elsewhere.
type Source struct {
	mu   sync.Mutex
	ip   net.IP
	last time.Time
}

// Accept reports whether a packet from ip at now is from the followed game
// controller, following ip if there is none or the followed one went silent.
func (s *Source) Accept(ip net.IP, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case s.ip == nil:
		slog.Info("following game controller", "address", ip)
	case s.ip.Equal(ip):
	case now.Sub(s.last) > SourceTimeout:
		slog.Info("following another game controller", "address", ip, "previous", s.ip, "silentFor", now.Sub(s.last).Round(time.Millisecond))
	default:
		return false
	}

	s.ip, s.last = ip, now

	return true
}
