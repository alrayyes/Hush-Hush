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
type WriteToken struct {
	ID          string
	Description string
	CreatedAt   string
	ExpiresAt   string
}

// CreateWriteToken issues a new write-path token, valid for ttl from now,
// and returns its id and plaintext. The plaintext is never recoverable
// again once this call returns - only its hash is stored, so a stolen
// database backup can't be replayed as a set of working tokens.
func (s *Store) CreateWriteToken(ctx context.Context, description string, ttl time.Duration) (id, token string, err error) {
	id, err = randomHex(8)
	if err != nil {
		return "", "", fmt.Errorf("generate token id: %w", err)
	}

	token, err = randomHex(32)
	if err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO write_tokens (id, token_hash, description, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, hashToken(token), description, now.Format(time.RFC3339), now.Add(ttl).Format(time.RFC3339),
	); err != nil {
		return "", "", fmt.Errorf("create write token: %w", err)
	}

	return id, token, nil
}

// ValidateWriteToken reports whether token is a currently issued,
// unexpired write token. An unknown or malformed token and an expired one
// are indistinguishable here on purpose - both mean "not authorized",
// same as api/openapi.yaml's write-path 401.
func (s *Store) ValidateWriteToken(ctx context.Context, token string) (bool, error) {
	var expiresAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM write_tokens WHERE token_hash = ?`, hashToken(token),
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("validate write token: %w", err)
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false, fmt.Errorf("parse write token expiry: %w", err)
	}

	return time.Now().UTC().Before(expiry), nil
}

// ListWriteTokens returns every issued token's metadata, oldest first -
// never the plaintext, which by design no longer exists anywhere to list.
func (s *Store) ListWriteTokens(ctx context.Context) ([]WriteToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, description, created_at, expires_at FROM write_tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list write tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tokens []WriteToken
	for rows.Next() {
		var t WriteToken
		if err := rows.Scan(&t.ID, &t.Description, &t.CreatedAt, &t.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan write token: %w", err)
		}

		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate write tokens: %w", err)
	}

	return tokens, nil
}

// RevokeWriteToken permanently invalidates the token issued under id. It
// returns ErrTokenNotFound if no token exists under that id - every other
// issued token is unaffected either way, per design: a leaked or
// forgotten token should never be a shared blast radius.
func (s *Store) RevokeWriteToken(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM write_tokens WHERE id = ?`, id)
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
