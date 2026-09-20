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

// Object is a stored secret object: its sealed value, its recorded used_by
// lineage, and its description. The service never decrypts Value - it is
// opaque ciphertext.
type Object struct {
	ID          string
	Value       []byte
	UsedBy      []string
	Description string
}

// CreateObject stores a new object under id. description is fixed at
// creation, the same as usedBy - there is no way to change it later
// (specs/secret-objects/spec.md). It returns ErrAlreadyExists if an object
// already exists under that id - existence is checked and the insert
// performed in the same transaction, so this is race-safe against
// concurrent creates under the same id.
func (s *Store) CreateObject(ctx context.Context, id string, value []byte, usedBy []string, description string) error {
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
		`INSERT INTO objects (id, value, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, value, description, now, now,
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

	switch err := s.db.QueryRowContext(ctx, `SELECT value, description FROM objects WHERE id = ?`, id).Scan(&obj.Value, &obj.Description); {
	case errors.Is(err, sql.ErrNoRows):
		return Object{}, ErrNotFound
	case err != nil:
		return Object{}, fmt.Errorf("select object: %w", err)
	}

	usedBy, err := s.usedByFor(ctx, id)
	if err != nil {
		return Object{}, err
	}
	obj.UsedBy = usedBy

	return obj, nil
}

// usedByFor returns id's recorded used_by lineage, consumer names sorted -
// shared by GetObject and ListObjects rather than each querying it inline.
func (s *Store) usedByFor(ctx context.Context, id string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT consumer FROM used_by WHERE object_id = ? ORDER BY consumer`, id)
	if err != nil {
		return nil, fmt.Errorf("select used_by: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var usedBy []string
	for rows.Next() {
		var consumer string
		if err := rows.Scan(&consumer); err != nil {
			return nil, fmt.Errorf("scan used_by: %w", err)
		}
		usedBy = append(usedBy, consumer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate used_by: %w", err)
	}

	return usedBy, nil
}

// ObjectFilter narrows a ListObjects call. The zero value matches every
// stored object.
type ObjectFilter struct {
	// UsedBy restricts the result to objects whose recorded used_by
	// lineage includes this consumer. Empty means no restriction.
	UsedBy string
}

// ListObjects returns every stored object's metadata (id, used_by,
// description - never the sealed value), sorted by id. filter narrows the
// result; its zero value returns everything.
func (s *Store) ListObjects(ctx context.Context, filter ObjectFilter) ([]Object, error) {
	var (
		rows *sql.Rows
		err  error
	)

	if filter.UsedBy != "" {
		rows, err = s.db.QueryContext(ctx, `
			SELECT DISTINCT o.id, o.description
			FROM objects o
			JOIN used_by u ON u.object_id = o.id
			WHERE u.consumer = ?
			ORDER BY o.id`, filter.UsedBy)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT id, description FROM objects ORDER BY id`)
	}
	if err != nil {
		return nil, fmt.Errorf("select objects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var objs []Object
	for rows.Next() {
		var obj Object
		if err := rows.Scan(&obj.ID, &obj.Description); err != nil {
			return nil, fmt.Errorf("scan object: %w", err)
		}
		objs = append(objs, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate objects: %w", err)
	}

	for i := range objs {
		usedBy, err := s.usedByFor(ctx, objs[i].ID)
		if err != nil {
			return nil, err
		}
		objs[i].UsedBy = usedBy
	}

	return objs, nil
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

// ListConsumers returns every distinct consumer name currently present in
// any object's used_by list, sorted, with no duplicates - the secret
// create/edit form offers these instead of relying on free-text recall
// (alrayyes/hush-hush#251).
func (s *Store) ListConsumers(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT consumer FROM used_by ORDER BY consumer`)
	if err != nil {
		return nil, fmt.Errorf("list consumers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var consumers []string
	for rows.Next() {
		var consumer string
		if err := rows.Scan(&consumer); err != nil {
			return nil, fmt.Errorf("scan consumer: %w", err)
		}

		consumers = append(consumers, consumer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate consumers: %w", err)
	}

	return consumers, nil
}

// DeleteObject permanently removes id, its used_by rows cascading with it
// (schema.sql's ON DELETE CASCADE). It returns ErrNotFound if no object
// exists under id.
func (s *Store) DeleteObject(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM objects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
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
