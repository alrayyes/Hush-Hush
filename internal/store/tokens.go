package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrTokenNotFound is returned when no write token exists under the given
// id.
var ErrTokenNotFound = errors.New("write token not found")

// WriteToken is one issued write-path token, without its plaintext - the
// plaintext is returned once, by CreateWriteToken, and never stored.
// Owner is the admin account that created it over HTTP, empty for one
// issued via the token CLI command (tokens/spec.md's "Token ownership"
// requirement - never guessed, only ever recorded when a session actually
// created it).
type WriteToken struct {
	ID          string
	Description string
	Owner       string
	CreatedAt   string
	ExpiresAt   string
	Revoked     bool
}

// CreateWriteToken issues a new write-path token, valid for ttl from now,
// and returns its metadata and plaintext. The plaintext is never
// recoverable again once this call returns - only its hash is stored, so
// a stolen database backup can't be replayed as a set of working tokens.
func (s *Store) CreateWriteToken(ctx context.Context, description string, ttl time.Duration, owner string) (WriteToken, string, error) {
	id, err := randomHex(8)
	if err != nil {
		return WriteToken{}, "", fmt.Errorf("generate token id: %w", err)
	}

	token, err := randomHex(32)
	if err != nil {
		return WriteToken{}, "", fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()
	wt := WriteToken{
		ID:          id,
		Description: description,
		Owner:       owner,
		CreatedAt:   now.Format(time.RFC3339),
		ExpiresAt:   now.Add(ttl).Format(time.RFC3339),
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO write_tokens (id, token_hash, description, created_at, expires_at, owner) VALUES (?, ?, ?, ?, ?, ?)`,
		wt.ID, hashToken(token), wt.Description, wt.CreatedAt, wt.ExpiresAt, nullableString(owner),
	); err != nil {
		return WriteToken{}, "", fmt.Errorf("create write token: %w", err)
	}

	return wt, token, nil
}

// ValidateWriteToken reports whether token is a currently issued,
// unexpired, unrevoked write token. An unknown, expired or revoked token
// are all indistinguishable here on purpose - each means "not
// authorized", same as api/openapi.yaml's write-path 401.
func (s *Store) ValidateWriteToken(ctx context.Context, token string) (bool, error) {
	_, valid, err := s.AuthenticateWriteToken(ctx, token)

	return valid, err
}

// AuthenticateWriteToken reports whether token is a currently issued,
// unexpired, unrevoked write token and, if so, its id - actor
// attribution needs to know exactly which token authenticated a write
// (audit-log/spec.md's "Verified actor attribution" requirement), not
// just that some token did.
func (s *Store) AuthenticateWriteToken(ctx context.Context, token string) (id string, valid bool, err error) {
	var expiresAt string

	err = s.db.QueryRowContext(ctx,
		`SELECT id, expires_at FROM write_tokens WHERE token_hash = ? AND revoked_at IS NULL`, hashToken(token),
	).Scan(&id, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("authenticate write token: %w", err)
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", false, fmt.Errorf("parse write token expiry: %w", err)
	}

	if !time.Now().UTC().Before(expiry) {
		return "", false, nil
	}

	return id, true, nil
}

// ListWriteTokens returns every issued token's metadata, oldest first,
// revoked ones included (marked, not omitted) - never the plaintext,
// which by design no longer exists anywhere to list.
func (s *Store) ListWriteTokens(ctx context.Context) ([]WriteToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, description, owner, created_at, expires_at, revoked_at FROM write_tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list write tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tokens []WriteToken
	for rows.Next() {
		var (
			t                WriteToken
			owner, revokedAt sql.NullString
		)

		if err := rows.Scan(&t.ID, &t.Description, &owner, &t.CreatedAt, &t.ExpiresAt, &revokedAt); err != nil {
			return nil, fmt.Errorf("scan write token: %w", err)
		}

		t.Owner = owner.String
		t.Revoked = revokedAt.Valid
		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate write tokens: %w", err)
	}

	return tokens, nil
}

// RevokeWriteToken invalidates the token issued under id, as a
// soft-delete - the row stays, marked revoked, so an audit log entry
// attributed to it stays resolvable (tokens/spec.md's "Revoked tokens
// stay attributable" requirement). It returns ErrTokenNotFound if no
// unrevoked token exists under that id - every other issued token is
// unaffected either way, per design: a leaked or forgotten token should
// never be a shared blast radius.
func (s *Store) RevokeWriteToken(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE write_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("revoke write token: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke write token: %w", err)
	}

	if n == 0 {
		return ErrTokenNotFound
	}

	return nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return hex.EncodeToString(b), nil
}
