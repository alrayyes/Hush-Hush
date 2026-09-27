package store

import (
	"context"
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

// CurrentUserID returns the id of the sole existing users row. There is
// exactly one user in this change's scope (design.md's Non-Goals) -
// Open's own backfillOwnership guarantees a users row always exists by
// the time this is called, so this is a plain lookup rather than another
// insert-if-missing.
func (s *Store) CurrentUserID(ctx context.Context) (string, error) {
	var id string

	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users LIMIT 1`).Scan(&id); err != nil {
		return "", fmt.Errorf("select current user: %w", err)
	}

	return id, nil
}

// SetUserEscrow records the escrowed writer identity's public key and its
// recovery-phrase-wrapped private key copy against id, once
// (openspec/changes/client-side-encryption/specs/users/spec.md's
// "Escrowed identity generated once" scenario). A no-op if that user
// already has a public key recorded: the client decides when a
// registration is the account's first (getAuthStatus's bootstrapped ==
// false) and only sends these fields then, but the server never trusts
// that alone - overwriting an already-escrowed identity from a later
// registration would silently strand every credential's existing wrapped
// copy, which was wrapped against the original identity, not a new one.
func (s *Store) SetUserEscrow(ctx context.Context, id, publicKey, recoveryWrappedIdentity string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET public_key = ?, recovery_wrapped_identity = ? WHERE id = ? AND public_key IS NULL`,
		publicKey, recoveryWrappedIdentity, id,
	); err != nil {
		return fmt.Errorf("set user escrow: %w", err)
	}

	return nil
}
