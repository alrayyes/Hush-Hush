package store

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// Writes outside an explicit transaction queue on the same lock, or they race
// it for SQLite's own and wait out the busy timeout without the context.
func TestWritesOutsideATransactionHonourTheContextToo(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "h.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	release, err := s.lockWrite(context.Background())
	require.NoError(t, err)

	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	writes := map[string]func() error{
		"audit log": func() error {
			return s.RecordAuditLog(ctx, "db", AuditActionRead, "ci", "127.0.0.1", "", "")
		},
		"write token": func() error {
			_, _, err := s.CreateWriteToken(ctx, "ci", time.Hour, "")

			return err
		},
		"consumer": func() error { return s.AddConsumer(ctx, "billing") },
		"consumer public key": func() error {
			_, err := s.SetConsumerPublicKey(ctx, "billing", "age1example")

			return err
		},
		"session": func() error {
			return s.CreateSession(ctx, Session{ID: "s1", CSRFToken: "c", CreatedAt: "now", ExpiresAt: "later"})
		},
	}

	for name, write := range writes {
		start := time.Now()

		require.ErrorIs(t, write(), context.DeadlineExceeded, name)
		require.Less(t, time.Since(start), 2*time.Second, name)
	}
}

func TestAFailedBeginGivesTheLockBack(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "h.db"))
	require.NoError(t, err)
	require.NoError(t, s.Close())

	// BeginTx on a closed database fails after the lock was taken.
	require.Error(t, s.CreateObject(context.Background(), "db", []byte("v"), []string{"a"}, "", ""))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	release, err := s.lockWrite(ctx)
	require.NoError(t, err, "the lock must be free after a failed begin")
	release()
}

func TestAnEndedContextNeverWinsAFreeLock(t *testing.T) {
	t.Parallel()

	s, err := Open(filepath.Join(t.TempDir(), "h.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// select picks at random between a free lock and a done context, so one
	// run proves nothing.
	for range 200 {
		release, err := s.lockWrite(ctx)
		if err == nil {
			release()
			t.Fatal("an ended context took the lock")
		}

		require.ErrorIs(t, err, context.Canceled)
	}
}

// Nothing in the store may write straight through s.db: every write goes
// through beginWrite or execWrite so it queues on the lock and honours the
// request context (alrayyes/hush-hush#863). A helper that accepts either a
// database or a transaction is the way one slipped past a search for
// ExecContext, so passing s.db to such a helper is flagged too.
func TestNoWriteBypassesTheWriteLock(t *testing.T) {
	t.Parallel()

	bypass := regexp.MustCompile(`s\.db\.(Exec|ExecContext)\(|(upsertConsumerRow|resolveRenamedPublicKey|rewriteUsedByForRename)\([^)]*\bs\.db\b`)

	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	for _, file := range files {
		if filepath.Ext(file) != ".go" || len(file) > 8 && file[len(file)-8:] == "_test.go" || file == "write_lock.go" {
			continue
		}

		src, err := fs.ReadFile(os.DirFS("."), file)
		require.NoError(t, err)

		require.Empty(t, bypass.FindAllString(string(src), -1), "%s writes outside the write lock", file)
	}
}
