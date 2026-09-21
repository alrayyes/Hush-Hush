package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrAlreadyExists is returned by CreateObject when an object already exists
// under the given id.
var ErrAlreadyExists = errors.New("object already exists")

// ErrNotFound is returned when no object exists under the given id.
var ErrNotFound = errors.New("object not found")

// ErrUnknownConsumer is returned by RenameConsumer and DeleteConsumer when
// no stored object's used_by list currently records the given name.
var ErrUnknownConsumer = errors.New("unknown consumer")

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

// ConsumerFilter narrows a ListConsumersPage call.
type ConsumerFilter struct {
	// Name restricts results to consumers whose name contains this
	// substring, case-insensitive. Empty means no restriction.
	Name string
	// Page is the 1-based page number and PageSize the maximum number of
	// consumers per page. Both must be positive.
	Page     int
	PageSize int
}

// ConsumerEntry is one consumer returned by ListConsumersPage: its name
// and how many stored secret objects record it in their used_by list.
type ConsumerEntry struct {
	Name        string
	SecretCount int
}

// ConsumerPage is one page of ListConsumersPage's filtered result, plus
// the total count of matching consumers across every page - what a
// caller needs to render page-number navigation.
type ConsumerPage struct {
	Consumers []ConsumerEntry
	Total     int
}

// ListConsumersPage returns one page of distinct consumer names whose
// name contains filter.Name (case-insensitive, every consumer when
// empty), each with a count of the secret objects whose used_by includes
// it, sorted by name - the consumer directory page's own listing
// (alrayyes/hush-hush#252), distinct from ListConsumers's plain,
// unpaginated array that consumer-combobox still relies on.
func (s *Store) ListConsumersPage(ctx context.Context, filter ConsumerFilter) (ConsumerPage, error) {
	pattern := "%" + escapeLike(filter.Name) + "%"

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT consumer) FROM used_by WHERE consumer LIKE ? ESCAPE '\'`,
		pattern,
	).Scan(&total); err != nil {
		return ConsumerPage{}, fmt.Errorf("count consumers: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT consumer, COUNT(*) AS secret_count
		FROM used_by
		WHERE consumer LIKE ? ESCAPE '\'
		GROUP BY consumer
		ORDER BY consumer
		LIMIT ? OFFSET ?`,
		pattern, filter.PageSize, (filter.Page-1)*filter.PageSize,
	)
	if err != nil {
		return ConsumerPage{}, fmt.Errorf("list consumers page: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var consumers []ConsumerEntry
	for rows.Next() {
		var entry ConsumerEntry
		if err := rows.Scan(&entry.Name, &entry.SecretCount); err != nil {
			return ConsumerPage{}, fmt.Errorf("scan consumer: %w", err)
		}

		consumers = append(consumers, entry)
	}

	if err := rows.Err(); err != nil {
		return ConsumerPage{}, fmt.Errorf("iterate consumers: %w", err)
	}

	return ConsumerPage{Consumers: consumers, Total: total}, nil
}

// RenameConsumer replaces oldName with newName in every stored object's
// used_by list that currently records oldName, returning the resulting
// entry under newName. Consumers aren't a stored resource of their own
// (alrayyes/hush-hush#282, ADR 0002) - this is a bulk rewrite across
// used_by, not a rename of a row with its own identity.
//
// If newName already has its own recorded objects, the two merge: an
// object recording both ends up with a single used_by entry, not a
// (object_id, consumer) primary-key violation, and the returned
// SecretCount covers every object now recording newName, old and
// already-there combined. Renaming a name to itself is a no-op. It
// returns ErrUnknownConsumer if oldName isn't currently recorded
// anywhere.
func (s *Store) RenameConsumer(ctx context.Context, oldName, newName string) (ConsumerEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ConsumerEntry{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var exists int
	switch err := tx.QueryRowContext(ctx, `SELECT 1 FROM used_by WHERE consumer = ? LIMIT 1`, oldName).Scan(&exists); {
	case errors.Is(err, sql.ErrNoRows):
		return ConsumerEntry{}, ErrUnknownConsumer
	case err != nil:
		return ConsumerEntry{}, fmt.Errorf("check existing consumer: %w", err)
	}

	if oldName != newName {
		// Drop the old entry wherever the object already records newName -
		// otherwise the UPDATE below would try to insert a second
		// (object_id, newName) row and violate used_by's primary key.
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM used_by
			WHERE consumer = ? AND object_id IN (
				SELECT object_id FROM used_by WHERE consumer = ?
			)`, oldName, newName,
		); err != nil {
			return ConsumerEntry{}, fmt.Errorf("drop merged used_by rows: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `UPDATE used_by SET consumer = ? WHERE consumer = ?`, newName, oldName); err != nil {
			return ConsumerEntry{}, fmt.Errorf("rename used_by rows: %w", err)
		}
	}

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM used_by WHERE consumer = ?`, newName).Scan(&count); err != nil {
		return ConsumerEntry{}, fmt.Errorf("count renamed consumer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ConsumerEntry{}, fmt.Errorf("commit transaction: %w", err)
	}

	return ConsumerEntry{Name: newName, SecretCount: count}, nil
}

// DeleteConsumer strips name from the used_by list of every stored object
// that currently records it. The objects themselves aren't touched
// otherwise, and none are deleted even if this empties their used_by
// list. It returns ErrUnknownConsumer if name isn't currently recorded
// anywhere.
func (s *Store) DeleteConsumer(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM used_by WHERE consumer = ?`, name)
	if err != nil {
		return fmt.Errorf("delete consumer: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return ErrUnknownConsumer
	}

	return nil
}

// escapeLike escapes SQLite LIKE's own wildcard characters (and the
// escape character itself) in s, so a name filter containing a literal
// "%" or "_" matches only that literal text rather than acting as a
// wildcard.
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

	return replacer.Replace(s)
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
