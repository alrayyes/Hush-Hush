package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// backfillOwnership ensures exactly one users row exists - the
// pre-existing admin account, modelled as a real row instead of an
// implicit fact about the schema (openspec/changes/client-side-
// encryption/design.md) - and that every pre-existing
// webauthn_credentials/objects row left with no owner by migrateColumns
// gets backfilled to reference it. Runs on every Open() alongside
// migrateColumns: ALTER TABLE ADD COLUMN's DEFAULT can't be a freshly
// generated id, so this needs real backfill logic rather than a column
// definition. Both the insert and the two updates below are themselves
// idempotent (WHERE user_id/owner_id IS NULL matches nothing once
// already backfilled), so running this on every Open, not just the
// first, is safe.
//
// There is exactly one user in this change's scope - multi-user is
// explicitly deferred (design.md's Non-Goals) - so there's nothing here
// yet that creates a second row or picks among more than one.
func backfillOwnership(db *sql.DB) error {
	userID, err := soleUserID(db)
	if err != nil {
		return err
	}

	if _, err := db.Exec(`UPDATE webauthn_credentials SET user_id = ? WHERE user_id IS NULL`, userID); err != nil {
		return fmt.Errorf("backfill credential owner: %w", err)
	}

	if _, err := db.Exec(`UPDATE objects SET owner_id = ? WHERE owner_id IS NULL`, userID); err != nil {
		return fmt.Errorf("backfill object owner: %w", err)
	}

	return nil
}

// soleUserID returns the id of the one existing users row, inserting it
// first (with a freshly generated id, the same randomHex convention
// tokens.go's CreateWriteToken already uses for an id, rather than
// adding a UUID-library dependency for this one column) if the table is
// still empty.
func soleUserID(db *sql.DB) (string, error) {
	var id string

	switch err := db.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&id); {
	case err == nil:
		return id, nil
	case !errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("select existing user: %w", err)
	}

	newID, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate user id: %w", err)
	}

	if _, err := db.Exec(
		`INSERT INTO users (id, created_at) VALUES (?, ?)`,
		newID, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return "", fmt.Errorf("insert backfilled user: %w", err)
	}

	return newID, nil
}
