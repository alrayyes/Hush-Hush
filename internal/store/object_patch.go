package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ObjectPatch is a metadata edit: each field is applied only when non-nil, so
// a nil Description is "leave it" and a pointer to "" clears it, the same
// split UpdateObject draws for used_by and tags.
type ObjectPatch struct {
	Description *string
	// Slug renames the name, and every variant of it moves.
	Slug *string
	// Tags replaces the object's tags; an empty, non-nil slice clears them.
	Tags *[]string
}

// PatchObject edits the metadata of the object addressed by slug without
// touching its sealed value, and returns that object's id and its slug
// afterwards. Description and tags change the one variant opts resolves,
// the same way UpdateObject resolves it (ErrAmbiguousSlug for a name with
// several and no WithID). A rename moves every variant of the name; it is
// ErrAlreadyExists if the new slug is taken, and the whole patch is applied
// or none of it. ErrNotFound if nothing matches.
func (s *Store) PatchObject(ctx context.Context, slug string, patch ObjectPatch, opts ...ObjectOption) (id, newSlug string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	id, err = resolveObjectID(ctx, tx, slug, applyObjectOptions(opts))
	if err != nil {
		return "", "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	newSlug = slug

	if patch.Description != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE objects SET description = ?, updated_at = ? WHERE id = ?`, *patch.Description, now, id); err != nil {
			return "", "", fmt.Errorf("update description: %w", err)
		}
	}

	if patch.Tags != nil {
		if err := replaceTags(ctx, tx, id, *patch.Tags); err != nil {
			return "", "", err
		}

		if _, err := tx.ExecContext(ctx, `UPDATE objects SET updated_at = ? WHERE id = ?`, now, id); err != nil {
			return "", "", fmt.Errorf("touch object: %w", err)
		}
	}

	if patch.Slug != nil && *patch.Slug != slug {
		if err := renameSlug(ctx, tx, slug, *patch.Slug, now); err != nil {
			return "", "", err
		}

		newSlug = *patch.Slug
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("commit transaction: %w", err)
	}

	return id, newSlug, nil
}

// renameSlug moves every object named from to the name to, refusing a name
// that already has objects. Nothing else references the slug (ADR 22), so
// this is the whole rename.
func renameSlug(ctx context.Context, q *sql.Tx, from, to, now string) error {
	var taken int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM objects WHERE slug = ?`, to).Scan(&taken); err != nil {
		return fmt.Errorf("check new slug: %w", err)
	}

	if taken > 0 {
		return ErrAlreadyExists
	}

	if _, err := q.ExecContext(ctx, `UPDATE objects SET slug = ?, updated_at = ? WHERE slug = ?`, to, now, from); err != nil {
		return fmt.Errorf("rename slug: %w", err)
	}

	return nil
}
