package main

import (
	"bytes"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// listenOnFreePort binds ADDR to an actual free loopback port for the
// duration of t, so the healthcheck subcommand under test has a real port to
// ask about - it reads ADDR itself, it doesn't take one as a flag.
func listenOnFreePort(t *testing.T) net.Listener {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	viper.Reset()
	t.Setenv("ADDR", l.Addr().String())
	t.Cleanup(viper.Reset)

	return l
}

func TestHealthcheckSucceedsWhenHealthzAnswers200(t *testing.T) {
	l := listenOnFreePort(t)

	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })

	root := newRootCmd()
	root.SetArgs([]string{"healthcheck"})
	root.SetOut(new(bytes.Buffer))
	require.NoError(t, root.Execute())
}

func TestHealthcheckFailsWhenHealthzAnswersNon200(t *testing.T) {
	l := listenOnFreePort(t)

	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })

	root := newRootCmd()
	root.SetArgs([]string{"healthcheck"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	require.Error(t, root.Execute())
}

func TestHealthcheckFailsWhenNothingIsListening(t *testing.T) {
	// A free port that was bound and immediately released, rather than
	// listenOnFreePort - nothing serves it, which is the failure mode the
	// container's own HEALTHCHECK is there to catch.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	viper.Reset()
	t.Setenv("ADDR", addr)
	t.Cleanup(viper.Reset)

	root := newRootCmd()
	root.SetArgs([]string{"healthcheck"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	require.Error(t, root.Execute())
}
