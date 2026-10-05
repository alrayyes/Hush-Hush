package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"regexp"
)

// newUUID returns a random (version 4) UUID in its canonical text form. An
// object's id is one, so it can be shown and addressed without being the name
// the object is asked for by (alrayyes/hush-hush#668).
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// migrateObjectIDsToUUID gives every object whose id isn't a UUID one,
// repointing the used_by and tags rows that reference it in the same
// transaction. A database from before ids were UUIDs holds random hex ids;
// nothing outside the store ever saw them, so nothing else has to change.
// Idempotent: a database with only UUID ids is left alone.
func migrateObjectIDsToUUID(db *sql.DB) error {
	old, err := objectIDsThatAreNotUUIDs(db)
	if err != nil {
		return err
	}

	if len(old) == 0 {
		return nil
	}

	return repointObjectIDs(db, old)
}

// objectIDsThatAreNotUUIDs lists the ids migrateObjectIDsToUUID has to replace.
func objectIDsThatAreNotUUIDs(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT id FROM objects`)
	if err != nil {
		return nil, fmt.Errorf("list object ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var old []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan object id: %w", err)
		}

		if !uuidPattern.MatchString(id) {
			old = append(old, id)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate object ids: %w", err)
	}

	return old, nil
}

// repointObjectIDs swaps each id in old for a fresh UUID. The foreign key
// check is deferred to COMMIT, as migrateObjectSlugs does, so the three tables
// can be updated in any order inside the one transaction.
func repointObjectIDs(db *sql.DB, old []string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		return fmt.Errorf("defer foreign keys: %w", err)
	}

	for _, oldID := range old {
		newID, err := newUUID()
		if err != nil {
			return err
		}

		for _, stmt := range []string{
			`UPDATE used_by SET object_id = ? WHERE object_id = ?`,
			`UPDATE tags SET object_id = ? WHERE object_id = ?`,
			`UPDATE objects SET id = ? WHERE id = ?`,
		} {
			if _, err := tx.Exec(stmt, newID, oldID); err != nil {
				return fmt.Errorf("move object %s to a uuid: %w", oldID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
