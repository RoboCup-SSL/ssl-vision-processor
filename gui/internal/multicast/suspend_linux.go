package multicast

import (
	"time"

	"golang.org/x/sys/unix"
)

// suspendOffset is CLOCK_BOOTTIME (counts suspend) minus CLOCK_MONOTONIC
// (doesn't). Go's own monotonic clock is CLOCK_MONOTONIC, so time.Since
// alone can't see a suspend.
func suspendOffset() time.Duration {
	var boot, mono unix.Timespec

	if unix.ClockGettime(unix.CLOCK_BOOTTIME, &boot) != nil || unix.ClockGettime(unix.CLOCK_MONOTONIC, &mono) != nil {
		return 0
	}

	return time.Duration(boot.Nano() - mono.Nano())
}
