package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrCeremonyNotFound is returned when no in-progress ceremony exists
// under a given id and kind - unknown, wrong-kind, already-consumed, and
// expired are all indistinguishable here on purpose, the same "not
// authorized" shape as an unknown or expired write token.
var ErrCeremonyNotFound = errors.New("ceremony not found")

// SaveCeremony stores in-progress WebAuthn ceremony state - the challenge
// a Begin call issued - until the matching Finish call consumes it via
// GetAndDeleteCeremony, or it expires unconsumed. data is the go-webauthn
// SessionData, opaque to this package.
func (s *Store) SaveCeremony(ctx context.Context, id, kind string, data []byte, createdAt, expiresAt string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO webauthn_ceremonies (id, kind, data, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, kind, data, createdAt, expiresAt,
	); err != nil {
		return fmt.Errorf("save ceremony: %w", err)
	}

	return nil
}

// GetAndDeleteCeremony consumes the ceremony state stored under id for
// kind - one-time use, so a Finish call can't be replayed against the
// same challenge twice. Also opportunistically deletes every expired
// ceremony row first, the cleanup pattern openspec/changes/web-ui/
// design.md's "WebAuthn ceremony state" decision settled on rather than a
// separate background job.
func (s *Store) GetAndDeleteCeremony(ctx context.Context, id, kind string) ([]byte, error) {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM webauthn_ceremonies WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		return nil, fmt.Errorf("clean up expired ceremonies: %w", err)
	}

	var data []byte

	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM webauthn_ceremonies WHERE id = ? AND kind = ?`, id, kind,
	).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCeremonyNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get ceremony: %w", err)
	}

	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM webauthn_ceremonies WHERE id = ? AND kind = ?`, id, kind,
	); err != nil {
		return nil, fmt.Errorf("delete consumed ceremony: %w", err)
	}

	return data, nil
}
