package store

import (
	"context"
	"database/sql"
	"fmt"
)

// lockWrite waits for this process's turn to write and returns the function
// that gives it back. It returns the context's error, and takes nothing, when
// ctx ends first.
func (s *Store) lockWrite(ctx context.Context) (func(), error) {
	// A context that has already ended never takes the lock, even when it is
	// free: select picks at random between two ready cases.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("wait for the write lock: %w", err)
	}

	select {
	case s.writeLock <- struct{}{}:
		return func() { <-s.writeLock }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for the write lock: %w", ctx.Err())
	}
}

// beginWrite takes the in-process write lock and opens a transaction under
// it. The caller defers release, after the deferred Rollback, so the lock is
// handed on once the transaction has ended.
func (s *Store) beginWrite(ctx context.Context) (*sql.Tx, func(), error) {
	release, err := s.lockWrite(ctx)
	if err != nil {
		return nil, nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		release()

		return nil, nil, fmt.Errorf("begin transaction: %w", err)
	}

	return tx, release, nil
}

// execWrite runs one write statement outside an explicit transaction under the
// same lock beginWrite takes, so it queues with the transactions instead of
// racing them for SQLite's own lock and waiting out the busy timeout without
// the request context. Reads do not take the lock.
func (s *Store) execWrite(ctx context.Context, query string, args ...any) (sql.Result, error) {
	release, err := s.lockWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("exec write: %w", err)
	}

	return result, nil
}
