package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// enforceOneVariantPerConsumer makes the schema refuse what the Go checks
// (checkCreatable, UpdateObject, RenameConsumer) already refuse: a consumer in
// two objects that share a slug (alrayyes/hush-hush#848). used_by keys on
// (object_id, consumer), which can't say it, because two variants have
// different object_ids. So used_by carries a copy of its object's slug, kept
// in step by triggers rather than by every writer, with a unique index over
// (slug, consumer).
//
// A database that already holds a duplicate is not repaired: picking which
// variant keeps the consumer would be a guess about someone's secrets. Open
// fails, naming each pair, so an operator removes the consumer from one of
// them with the API on the previous version first.
func enforceOneVariantPerConsumer(db *sql.DB) error {
	// One transaction, so a writer on the previous binary can't slip a
	// duplicate in between the check and the index.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := reportDuplicateVariants(tx); err != nil {
		return err
	}

	if err := addColumnIfMissing(tx, "used_by", "slug", "TEXT"); err != nil {
		return err
	}

	for _, stmt := range []string{
		`UPDATE used_by SET slug = (SELECT slug FROM objects WHERE objects.id = used_by.object_id) WHERE slug IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS used_by_slug_consumer ON used_by (slug, consumer)`,
		// AFTER INSERT rather than BEFORE: a trigger can't edit NEW. The
		// follow-up UPDATE is what the unique index then checks, inside the
		// same statement, so a duplicate still fails the insert.
		`CREATE TRIGGER IF NOT EXISTS used_by_fill_slug AFTER INSERT ON used_by
			WHEN NEW.slug IS NULL
			BEGIN
				UPDATE used_by SET slug = (SELECT slug FROM objects WHERE id = NEW.object_id)
				WHERE object_id = NEW.object_id AND consumer = NEW.consumer;
			END`,
		`CREATE TRIGGER IF NOT EXISTS objects_sync_used_by_slug AFTER UPDATE OF slug ON objects
			BEGIN
				UPDATE used_by SET slug = NEW.slug WHERE object_id = NEW.id;
			END`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("enforce one variant per consumer: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// asSlugConflict turns the unique index's violation into sentinel, so the
// API answers 409 like it does for the pre-checks instead of a 500. Any other
// error passes through.
func asSlugConflict(err, sentinel error) error {
	if err != nil && strings.Contains(err.Error(), "used_by.slug") {
		return fmt.Errorf("%w: %w", sentinel, err)
	}

	return err
}

// reportDuplicateVariants returns an error listing every (slug, consumer)
// pair that appears in more than one object.
func reportDuplicateVariants(db queryExecer) error {
	rows, err := db.Query(`
		SELECT o.slug, u.consumer, COUNT(*)
		FROM used_by u JOIN objects o ON o.id = u.object_id
		GROUP BY o.slug, u.consumer HAVING COUNT(*) > 1
		ORDER BY o.slug, u.consumer`)
	if err != nil {
		return fmt.Errorf("check duplicate variants: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var pairs []string

	for rows.Next() {
		var slug, consumer string

		var n int
		if err := rows.Scan(&slug, &consumer, &n); err != nil {
			return fmt.Errorf("scan duplicate variant: %w", err)
		}

		pairs = append(pairs, fmt.Sprintf("consumer %q in %d secrets named %q", consumer, n, slug))
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate duplicate variants: %w", err)
	}

	if len(pairs) > 0 {
		return fmt.Errorf("%w: %s", ErrDuplicateVariants, strings.Join(pairs, "; "))
	}

	return nil
}
