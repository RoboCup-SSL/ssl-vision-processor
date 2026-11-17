package multicast

import "time"

// suspendThreshold is how much unaccounted time counts as a suspend rather
// than scheduling jitter.
const suspendThreshold = 2 * time.Second

// SuspendDetector notices that the machine was suspended between two checks.
// It compares a clock that keeps counting through suspend with one that
// doesn't (on Linux, CLOCK_BOOTTIME and CLOCK_MONOTONIC): neither jumps for
// NTP or manual clock changes, so their difference grows by exactly the time
// spent asleep and never otherwise.
type SuspendDetector struct {
	// offset reads the suspend-counting clock minus the one that isn't.
	offset func() time.Duration
	last   time.Duration
}

// NewSuspendDetector starts watching from now.
func NewSuspendDetector() *SuspendDetector {
	return newSuspendDetector(suspendOffset)
}

func newSuspendDetector(offset func() time.Duration) *SuspendDetector {
	return &SuspendDetector{offset: offset, last: offset()}
}

// Check reports how long the machine was suspended since the last check, or
// 0 if it wasn't.
func (d *SuspendDetector) Check() time.Duration {
	now := d.offset()
	slept := now - d.last
	d.last = now

	if slept < suspendThreshold {
		return 0
	}

	return slept
}
