package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

var errProbeFailed = errors.New("database is unusable")

// fakeChecker stands in for the store: it counts probes, can fail, and can
// block until the probe's context gives up.
type fakeChecker struct {
	calls atomic.Int32
	err   error
	block bool
}

func (c *fakeChecker) Ready(ctx context.Context) error {
	c.calls.Add(1)

	if c.block {
		<-ctx.Done()

		return fmt.Errorf("probe gave up: %w", ctx.Err())
	}

	return c.err
}

// readyzMux serves /readyz from a Readiness around checker, with a clock the
// test controls so the cache window is exact rather than a sleep.
func readyzMux(t *testing.T, checker *fakeChecker, clock *time.Time, opts ...hushhush.ReadinessOption) (*http.ServeMux, *hushhush.Readiness) {
	t.Helper()

	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	readiness := hushhush.NewReadiness(checker, opts...)
	if clock != nil {
		readiness.SetClock(func() time.Time { return *clock })
	}

	return hushhush.NewMux(s, testPublicURL, testWebBuild(), testVersion, hushhush.WithReadiness(readiness)), readiness
}

func probe(mux http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

func TestReadyzFollowsTheChecker(t *testing.T) {
	t.Parallel()

	mux, _ := readyzMux(t, &fakeChecker{}, nil)
	require.Equal(t, http.StatusOK, probe(mux, "/readyz").Code)

	mux, _ = readyzMux(t, &fakeChecker{err: errProbeFailed}, nil)
	rec := probe(mux, "/readyz")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String(), "a failure never says why")
}

// A hung database must not hang the probe: the orchestrator would count a
// probe that never answers, and a 5 second health-check timeout is slower than
// the answer it needs.
func TestReadyzGivesUpOnASlowDatabase(t *testing.T) {
	t.Parallel()

	mux, _ := readyzMux(t, &fakeChecker{block: true}, nil, hushhush.WithProbeTimeout(50*time.Millisecond))

	start := time.Now()
	rec := probe(mux, "/readyz")

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Less(t, time.Since(start), time.Second, "it should give up near the timeout, not hang")
}

func TestReadyzCachesTheResultBriefly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	checker := &fakeChecker{}
	mux, _ := readyzMux(t, checker, &now, hushhush.WithProbeCache(3*time.Second))

	require.Equal(t, http.StatusOK, probe(mux, "/readyz").Code)
	now = now.Add(2 * time.Second)
	require.Equal(t, http.StatusOK, probe(mux, "/readyz").Code)
	require.EqualValues(t, 1, checker.calls.Load(), "a probe inside the window reuses the answer")

	now = now.Add(2 * time.Second) // 4s after the first: the window has passed
	require.Equal(t, http.StatusOK, probe(mux, "/readyz").Code)
	require.EqualValues(t, 2, checker.calls.Load(), "a probe after the window asks again")
}

func TestReadyzProbesOnceForConcurrentRequests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	checker := &fakeChecker{}
	mux, _ := readyzMux(t, checker, &now, hushhush.WithProbeCache(time.Minute))

	var wg sync.WaitGroup
	for range 25 {
		wg.Go(func() { require.Equal(t, http.StatusOK, probe(mux, "/readyz").Code) })
	}
	wg.Wait()

	require.EqualValues(t, 1, checker.calls.Load(), "a burst of probes costs one database call")
}

// Draining is the point of the drain: a load balancer has to see "not ready"
// before the server stops accepting, even when a fresh "ok" is cached.
func TestReadyzReportsUnavailableAsSoonAsDraining(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	checker := &fakeChecker{}
	mux, readiness := readyzMux(t, checker, &now, hushhush.WithProbeCache(time.Minute))

	require.Equal(t, http.StatusOK, probe(mux, "/readyz").Code)

	readiness.Drain()

	rec := probe(mux, "/readyz")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, "the cached ok must not mask the drain")
	require.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String())
}

// /healthz is about the process being up, which it still is while draining.
func TestHealthzStaysUpWhileDraining(t *testing.T) {
	t.Parallel()

	mux, readiness := readyzMux(t, &fakeChecker{}, nil)
	readiness.Drain()

	require.Equal(t, http.StatusOK, probe(mux, "/healthz").Code)
}
