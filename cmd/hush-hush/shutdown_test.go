package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// fetchStatus returns a GET's status, or the error that stopped it. It makes
// no assertions, so it's safe to call from a goroutine.
func fetchStatus(url string) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("get %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode, nil
}

func statusOf(t *testing.T, url string) int {
	t.Helper()

	status, err := fetchStatus(url)
	require.NoError(t, err)

	return status
}

// A load balancer has to see "not ready" before the server stops accepting,
// or the requests already on their way get refused. So shutdown first flips
// /readyz to 503, keeps serving for the drain window, and only then closes.
func TestShutdownDrainsBeforeItStopsAccepting(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	ready := hushhush.NewReadiness(s)
	build := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html></html>")}}
	srv := &http.Server{
		Handler:           hushhush.NewMux(s, "", build, "test", hushhush.WithReadiness(ready)),
		ReadHeaderTimeout: time.Second,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() { _ = srv.Serve(listener) }()

	base := "http://" + listener.Addr().String()
	require.Equal(t, http.StatusOK, statusOf(t, base+"/readyz"), "ready before shutdown starts")

	done := make(chan struct{})
	go func() {
		drainAndShutdown(srv, ready, 400*time.Millisecond, 2*time.Second)
		close(done)
	}()

	// Inside the drain window: not ready, but the server still answers.
	require.Eventually(t, func() bool { return statusOf(t, base+"/readyz") == http.StatusServiceUnavailable },
		300*time.Millisecond, 10*time.Millisecond, "/readyz should flip to 503 at once")
	require.Equal(t, http.StatusOK, statusOf(t, base+"/healthz"), "still accepting during the drain")

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown never finished")
	}

	_, err = fetchStatus(base + "/healthz")
	require.Error(t, err, "after the drain and shutdown nothing should be accepting")
}

// http.Server.ListenAndServe returns as soon as Shutdown is called, not when it
// finishes. If serve returned then, the process would exit, and its deferred
// store.Close would run, under requests still in flight. serveUntilDone waits.
func TestServeUntilDoneFinishesInFlightRequestsBeforeItReturns(t *testing.T) {
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	ready := hushhush.NewReadiness(s)
	build := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html></html>")}}
	mux := hushhush.NewMux(s, "", build, "test", hushhush.WithReadiness(ready))

	started := make(chan struct{})
	handler := http.NewServeMux()
	handler.Handle("/", mux)
	handler.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(600 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: time.Second}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- serveUntilDone(ctx, srv, listener, ready, 100*time.Millisecond, 3*time.Second) }()

	type result struct {
		status int
		err    error
	}
	slowResult := make(chan result, 1)
	go func() {
		status, err := fetchStatus("http://" + listener.Addr().String() + "/slow")
		slowResult <- result{status, err}
	}()

	<-started // the slow request is now in flight
	cancel()  // a shutdown signal arrives

	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("serveUntilDone never returned")
	}

	// serveUntilDone may only return once the slow request has been answered.
	select {
	case got := <-slowResult:
		require.NoError(t, got.err)
		require.Equal(t, http.StatusOK, got.status, "the in-flight request should finish, not be cut off")
	default:
		t.Fatal("serveUntilDone returned while a request was still in flight")
	}
}
