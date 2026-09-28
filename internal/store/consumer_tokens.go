package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ConsumerToken is one issued consumer read token, without its plaintext -
// the plaintext is returned once, by CreateConsumerToken, and never
// stored. Unlike a WriteToken, it's bound to exactly one consumer name
// (openspec/changes/consumer-read-tokens/design.md's "New consumer_tokens
// table" decision) - GET /objects/{slug} only accepts it for an object
// whose used_by includes that consumer.
type ConsumerToken struct {
	ID          string
	Consumer    string
	Description string
	CreatedAt   string
	ExpiresAt   string
	Revoked     bool
	LastUsedAt  string // "" means never used
}

// CreateConsumerToken issues a new read token scoped to consumer, valid
// for ttl from now, and returns its metadata and plaintext. The plaintext
// is never recoverable again once this call returns - only its hash is
// stored, same as CreateWriteToken.
func (s *Store) CreateConsumerToken(ctx context.Context, consumer, description string, ttl time.Duration) (ConsumerToken, string, error) {
	id, err := randomHex(8)
	if err != nil {
		return ConsumerToken{}, "", fmt.Errorf("generate consumer token id: %w", err)
	}

	token, err := randomHex(32)
	if err != nil {
		return ConsumerToken{}, "", fmt.Errorf("generate consumer token: %w", err)
	}

	now := time.Now().UTC()
	ct := ConsumerToken{
		ID:          id,
		Consumer:    consumer,
		Description: description,
		CreatedAt:   now.Format(time.RFC3339),
		ExpiresAt:   now.Add(ttl).Format(time.RFC3339),
	}

	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO consumer_tokens (id, consumer, token_hash, description, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		ct.ID, ct.Consumer, hashToken(token), ct.Description, ct.CreatedAt, ct.ExpiresAt,
	); err != nil {
		return ConsumerToken{}, "", fmt.Errorf("create consumer token: %w", err)
	}

	return ct, token, nil
}

// AuthenticateConsumerToken reports whether token is a currently issued,
// unexpired, unrevoked consumer token and, if so, its id and bound
// consumer name - GET /objects/{slug} needs the consumer name to check
// against the requested object's used_by list.
func (s *Store) AuthenticateConsumerToken(ctx context.Context, token string) (id, consumer string, valid bool, err error) {
	var expiresAt string

	err = s.db.QueryRowContext(ctx,
		`SELECT id, consumer, expires_at FROM consumer_tokens WHERE token_hash = ? AND revoked_at IS NULL`, hashToken(token),
	).Scan(&id, &consumer, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("authenticate consumer token: %w", err)
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "", "", false, fmt.Errorf("parse consumer token expiry: %w", err)
	}

	if !time.Now().UTC().Before(expiry) {
		return "", "", false, nil
	}

	return id, consumer, true, nil
}

// ListConsumerTokens returns every issued consumer token's metadata,
// oldest first, revoked ones included (marked, not omitted) - never the
// plaintext.
func (s *Store) ListConsumerTokens(ctx context.Context) ([]ConsumerToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, consumer, description, created_at, expires_at, revoked_at, last_used_at FROM consumer_tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list consumer tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tokens []ConsumerToken
	for rows.Next() {
		var (
			t                     ConsumerToken
			revokedAt, lastUsedAt sql.NullString
		)

		if err := rows.Scan(&t.ID, &t.Consumer, &t.Description, &t.CreatedAt, &t.ExpiresAt, &revokedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan consumer token: %w", err)
		}

		t.Revoked = revokedAt.Valid
		t.LastUsedAt = lastUsedAt.String
		tokens = append(tokens, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate consumer tokens: %w", err)
	}

	return tokens, nil
}

// RevokeConsumerToken invalidates the consumer token issued under id, as
// a soft-delete, same as RevokeWriteToken - the row stays, marked
// revoked, so an audit log entry attributed to it stays resolvable. It
// returns ErrTokenNotFound if no unrevoked token exists under that id.
func (s *Store) RevokeConsumerToken(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE consumer_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("revoke consumer token: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke consumer token: %w", err)
	}

	if n == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// RotateConsumerToken issues a new secret and a new expiry for the
// consumer token under id, keeping its id, consumer, and description
// unchanged - same in-place rotation RotateWriteToken already does. It
// returns ErrTokenNotFound if no currently valid (unrevoked, unexpired)
// token exists under id.
func (s *Store) RotateConsumerToken(ctx context.Context, id string, ttl time.Duration) (ConsumerToken, string, error) {
	token, err := randomHex(32)
	if err != nil {
		return ConsumerToken{}, "", fmt.Errorf("generate consumer token: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl).Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		`UPDATE consumer_tokens SET token_hash = ?, expires_at = ?, last_used_at = NULL
		 WHERE id = ? AND revoked_at IS NULL AND expires_at > ?`,
		hashToken(token), expiresAt, id, now.Format(time.RFC3339),
	)
	if err != nil {
		return ConsumerToken{}, "", fmt.Errorf("rotate consumer token: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return ConsumerToken{}, "", fmt.Errorf("rotate consumer token: %w", err)
	}

	if n == 0 {
		return ConsumerToken{}, "", ErrTokenNotFound
	}

	var ct ConsumerToken

	if err := s.db.QueryRowContext(ctx,
		`SELECT id, consumer, description, created_at, expires_at FROM consumer_tokens WHERE id = ?`, id,
	).Scan(&ct.ID, &ct.Consumer, &ct.Description, &ct.CreatedAt, &ct.ExpiresAt); err != nil {
		return ConsumerToken{}, "", fmt.Errorf("rotate consumer token: %w", err)
	}

	return ct, token, nil
}

// UpdateConsumerTokenUsage records a successful authentication's
// timestamp, same as UpdateWriteTokenUsage.
func (s *Store) UpdateConsumerTokenUsage(ctx context.Context, id, usedAt string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE consumer_tokens SET last_used_at = ? WHERE id = ?`, usedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update consumer token usage: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update consumer token usage: %w", err)
	}

	if n == 0 {
		return ErrTokenNotFound
	}

	return nil
}
