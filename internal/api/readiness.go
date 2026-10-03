package api

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// defaultProbeTimeout bounds one database check. An orchestrator's own
	// health-check timeout is slower than the answer it needs (the image's is
	// 5 seconds), so a hung database should read as not ready well before then.
	defaultProbeTimeout = 2 * time.Second

	// defaultProbeCache is how long an answer is reused. Several probers (the
	// container health check, a load balancer, a monitor) can ask in the same
	// moment, and none of them needs the database hit again for each.
	defaultProbeCache = 3 * time.Second
)

var errDraining = errors.New("shutting down")

// Readiness answers /readyz: whether this process should be sent requests.
// That is the database being usable, briefly cached and bounded by a timeout,
// and, once Drain has been called, never. Draining is how a shutdown tells a
// load balancer to stop sending traffic before the server stops accepting it,
// so the requests already on their way aren't refused.
type Readiness struct {
	checker  readinessChecker
	timeout  time.Duration
	cacheFor time.Duration
	now      func() time.Time

	draining atomic.Bool

	mu        sync.Mutex // one probe at a time, so a burst costs one database call
	checked   bool
	checkedAt time.Time
	result    error
}

// ReadinessOption adjusts a Readiness.
type ReadinessOption func(*Readiness)

// WithProbeTimeout sets how long one database check may take before it counts
// as a failure.
func WithProbeTimeout(d time.Duration) ReadinessOption {
	return func(r *Readiness) { r.timeout = d }
}

// WithProbeCache sets how long a database check's answer is reused.
func WithProbeCache(d time.Duration) ReadinessOption {
	return func(r *Readiness) { r.cacheFor = d }
}

// NewReadiness builds a Readiness around checker, usually the store.
func NewReadiness(checker readinessChecker, opts ...ReadinessOption) *Readiness {
	r := &Readiness{
		checker:  checker,
		timeout:  defaultProbeTimeout,
		cacheFor: defaultProbeCache,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Drain makes /readyz answer 503 from now on. It's called when shutdown
// starts and can't be undone: the process is on its way out.
func (r *Readiness) Drain() { r.draining.Store(true) }

// check reports why the process isn't ready, or nil when it is. A drain wins
// over anything cached, so a fresh "ok" can't hide a shutdown.
func (r *Readiness) check(ctx context.Context) error {
	if r.draining.Load() {
		return errDraining
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.checked && r.now().Sub(r.checkedAt) < r.cacheFor {
		return r.result
	}

	// Detached from the request: one client hanging up mid-probe must not
	// leave a cancelled-context failure cached for everyone who asks next.
	probeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
	defer cancel()

	r.result = r.checker.Ready(probeCtx)
	r.checkedAt = r.now()
	r.checked = true

	return r.result
}
