package store_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

// Concurrent creates on a file database must queue behind one another
// (alrayyes/hush-hush#852) instead of failing with SQLITE_BUSY: the
// race for one slug leaves exactly one winner, and every loser sees the
// duplicate as ErrAlreadyExists, never a raw "database is locked".
func TestConcurrentCreatesOfOneSlugReturnAlreadyExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	const workers = 12

	var wg sync.WaitGroup
	errs := make([]error, workers)
	start := make(chan struct{})

	for i := range workers {
		wg.Go(func() {
			<-start

			errs[i] = s.CreateObject(context.Background(), "release_token", []byte("v"), []string{"a"}, "", "")
		})
	}
	close(start)
	wg.Wait()

	successes := 0
	for i, err := range errs {
		if err != nil {
			require.ErrorIs(t, err, store.ErrAlreadyExists, "worker %d: %v", i, err)

			continue
		}

		successes++
	}
	require.Equal(t, 1, successes, "exactly one create must succeed")
}

// Concurrent creates of different slugs must all succeed - queueing, not
// serialising one of them into a failure.
func TestConcurrentCreatesOfDifferentSlugsAllSucceed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")

	s, err := store.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	const workers = 12

	var wg sync.WaitGroup
	errs := make([]error, workers)
	start := make(chan struct{})

	for i := range workers {
		wg.Go(func() {
			<-start

			errs[i] = s.CreateObject(context.Background(),
				fmt.Sprintf("slug-%d", i), []byte("v"), []string{fmt.Sprintf("c%d", i)}, "", "")
		})
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "worker %d: %v", i, err)
	}
}
