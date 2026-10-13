package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A writer that waits for the write lock gives up when its request context
// ends (alrayyes/hush-hush#863). The driver ignores the context while it
// waits on SQLite's own busy handler, so in-process writers queue on a
// semaphore that does honour it.
func TestAWriterWaitingForTheLockStopsWhenItsContextEnds(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "h.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	release, err := s.lockWrite(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	err = s.CreateObject(ctx, "db", []byte("v"), []string{"a"}, "", "")

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 2*time.Second, "it must not wait out the ten-second busy timeout")

	release()

	require.NoError(t, s.CreateObject(context.Background(), "db", []byte("v"), []string{"a"}, "", ""), "the lock is free again")
}

func TestACancelledContextNeverTakesTheLock(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "h.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, s.CreateObject(ctx, "db", []byte("v"), []string{"a"}, "", ""), context.Canceled)
	require.NoError(t, s.CreateObject(context.Background(), "db", []byte("v"), []string{"a"}, "", ""), "a cancelled caller did not leak the lock")
}
