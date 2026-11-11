//go:build !linux

package multicast

import "time"

var start = time.Now()

// suspendOffset falls back to the wall clock (counts suspend) minus Go's
// monotonic clock. Unlike CLOCK_BOOTTIME, the wall clock can jump for a
// manual clock change or an NTP step, which then reads as a suspend; the
// only effect is a harmless reopen.
func suspendOffset() time.Duration {
	now := time.Now()

	return now.Round(0).Sub(start.Round(0)) - now.Sub(start)
}
