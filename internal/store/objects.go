package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrAlreadyExists is returned by CreateObject when an object already exists
// under the given id.
var ErrAlreadyExists = errors.New("object already exists")

// ErrNotFound is returned when no object exists under the given id.
var ErrNotFound = errors.New("object not found")

// Object is a stored secret object: its sealed value and its recorded
// used_by lineage. The service never decrypts Value - it is opaque
// ciphertext.
type Object struct {
	ID     string
	Value  []byte
	UsedBy []string
}

// CreateObject stores a new object under id. It returns ErrAlreadyExists if
// an object already exists under that id - existence is checked and the
// insert performed in the same transaction, so this is race-safe against
// concurrent creates under the same id.
func (s *Store) CreateObject(ctx context.Context, id string, value []byte, usedBy []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM objects WHERE id = ?`, id).Scan(&exists); {
	case err == nil:
		return ErrAlreadyExists
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("check existing object: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO objects (id, value, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, value, now, now,
	); err != nil {
		return fmt.Errorf("insert object: %w", err)
	}

	for _, consumer := range usedBy {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO used_by (object_id, consumer) VALUES (?, ?)`,
			id, consumer,
		); err != nil {
			return fmt.Errorf("insert used_by: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetObject fetches an object's sealed value and used_by lineage. It
// returns ErrNotFound if no object exists under id.
func (s *Store) GetObject(ctx context.Context, id string) (Object, error) {
	obj := Object{ID: id}

	switch err := s.db.QueryRowContext(ctx, `SELECT value FROM objects WHERE id = ?`, id).Scan(&obj.Value); {
	case errors.Is(err, sql.ErrNoRows):
		return Object{}, ErrNotFound
	case err != nil:
		return Object{}, fmt.Errorf("select object: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT consumer FROM used_by WHERE object_id = ? ORDER BY consumer`, id)
	if err != nil {
		return Object{}, fmt.Errorf("select used_by: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var consumer string
		if err := rows.Scan(&consumer); err != nil {
			return Object{}, fmt.Errorf("scan used_by: %w", err)
		}
		obj.UsedBy = append(obj.UsedBy, consumer)
	}
	if err := rows.Err(); err != nil {
		return Object{}, fmt.Errorf("iterate used_by: %w", err)
	}

	return obj, nil
}

// UpdateObject replaces the stored value for id, leaving used_by untouched -
// this call only ever touches the value. It returns ErrNotFound if no
// object exists under id.
func (s *Store) UpdateObject(ctx context.Context, id string, value []byte) error {
	now := time.Now().UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		`UPDATE objects SET value = ?, updated_at = ? WHERE id = ?`,
		value, now, id,
	)
	if err != nil {
		return fmt.Errorf("update object: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
