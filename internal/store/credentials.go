package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrCredentialNotFound is returned when no credential exists under a
// given id.
var ErrCredentialNotFound = errors.New("credential not found")

// Credential is one registered WebAuthn passkey. PublicKey, SignCount, and
// AAGUID stay internal to the ceremony logic - only ID, Nickname, and the
// timestamps ever cross the HTTP boundary (api/openapi.yaml's Credential
// schema).
type Credential struct {
	ID         string
	PublicKey  []byte
	SignCount  uint32
	AAGUID     string
	Nickname   string
	CreatedAt  string
	LastUsedAt string // "" means never used
}

// CreateCredential stores a newly registered credential.
func (s *Store) CreateCredential(ctx context.Context, c Credential) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO webauthn_credentials (id, public_key, sign_count, aaguid, nickname, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.PublicKey, c.SignCount, c.AAGUID, c.Nickname, c.CreatedAt,
	); err != nil {
		return fmt.Errorf("create credential: %w", err)
	}

	return nil
}

// ListCredentials returns every registered credential, oldest first. An
// empty result means no admin account exists yet - openspec/changes/
// web-ui/specs/auth/spec.md's "Registering a first passkey" scenario is
// keyed off exactly this.
func (s *Store) ListCredentials(ctx context.Context) ([]Credential, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, public_key, sign_count, aaguid, nickname, created_at, last_used_at FROM webauthn_credentials ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var creds []Credential

	for rows.Next() {
		var (
			c          Credential
			lastUsedAt sql.NullString
		)

		if err := rows.Scan(&c.ID, &c.PublicKey, &c.SignCount, &c.AAGUID, &c.Nickname, &c.CreatedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}

		c.LastUsedAt = lastUsedAt.String
		creds = append(creds, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate credentials: %w", err)
	}

	return creds, nil
}

// UpdateCredentialUsage records a successful login's new signature counter
// and timestamp - go-webauthn's own clone-detection step compares this
// stored counter against a later assertion's, so it has to be kept current.
func (s *Store) UpdateCredentialUsage(ctx context.Context, id string, signCount uint32, lastUsedAt string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE webauthn_credentials SET sign_count = ?, last_used_at = ? WHERE id = ?`,
		signCount, lastUsedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update credential usage: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update credential usage: %w", err)
	}

	if n == 0 {
		return ErrCredentialNotFound
	}

	return nil
}
