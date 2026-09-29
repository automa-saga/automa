package automa

import (
	"sync/atomic"
	"time"
)

var clock atomic.Pointer[func() time.Time]

// SetClock sets the function automa uses for every report and step timestamp.
// Pass nil to restore time.Now. To keep reports in UTC:
//
//	automa.SetClock(func() time.Time { return time.Now().UTC() })
//
// Call it once at startup, before any workflow runs. Time.UTC strips the
// monotonic clock reading, so durations computed from a UTC clock follow wall
// time and can be skewed by a system clock change mid-run.
func SetClock(now func() time.Time) {
	if now == nil {
		clock.Store(nil)
		return
	}
	clock.Store(&now)
}

// clockNow returns the current time from the clock set by SetClock.
func clockNow() time.Time {
	if now := clock.Load(); now != nil {
		return (*now)()
	}
	return time.Now()
}
