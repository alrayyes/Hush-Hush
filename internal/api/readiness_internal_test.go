package api

import "time"

// SetClock lets a test control the cache window instead of sleeping through it.
func (r *Readiness) SetClock(now func() time.Time) { r.now = now }
