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

// ErrTokenStillActive is returned by PurgeWriteToken/PurgeConsumerToken
// when the token under the given id is neither revoked nor expired - a
// purge is only allowed once a token is already dead, so nothing still
// authenticating anything can be removed out from under an in-flight
// audit review.
var ErrTokenStillActive = errors.New("token is still active")

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
	LastUsedAt  string // "" means never used
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
		`SELECT id, description, owner, created_at, expires_at, revoked_at, last_used_at FROM write_tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list write tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tokens []WriteToken
	for rows.Next() {
		var (
			t                            WriteToken
			owner, revokedAt, lastUsedAt sql.NullString
		)

		if err := rows.Scan(&t.ID, &t.Description, &owner, &t.CreatedAt, &t.ExpiresAt, &revokedAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan write token: %w", err)
		}

		t.Owner = owner.String
		t.Revoked = revokedAt.Valid
		t.LastUsedAt = lastUsedAt.String
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

// RotateWriteToken issues a new secret and a new expiry for the token
// under id, keeping its id, description and owner unchanged - so an audit
// log entry, or anything else that already refers to the token by id,
// keeps resolving through the rotation the same way ADR 0017's
// revoke-by-flag keeps id references resolvable across a revoke. The old
// secret stops authenticating immediately, since only the new secret's
// hash is stored afterward. It returns ErrTokenNotFound if no currently
// valid (unrevoked, unexpired) token exists under id - a revoked or
// already-expired token isn't rotated back to life; issue a new one
// instead.
func (s *Store) RotateWriteToken(ctx context.Context, id string, ttl time.Duration) (WriteToken, string, error) {
	token, err := randomHex(32)
	if err != nil {
		return WriteToken{}, "", fmt.Errorf("generate token: %w", err)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl).Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		`UPDATE write_tokens SET token_hash = ?, expires_at = ?, last_used_at = NULL
		 WHERE id = ? AND revoked_at IS NULL AND expires_at > ?`,
		hashToken(token), expiresAt, id, now.Format(time.RFC3339),
	)
	if err != nil {
		return WriteToken{}, "", fmt.Errorf("rotate write token: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return WriteToken{}, "", fmt.Errorf("rotate write token: %w", err)
	}

	if n == 0 {
		return WriteToken{}, "", ErrTokenNotFound
	}

	var (
		wt    WriteToken
		owner sql.NullString
	)

	if err := s.db.QueryRowContext(ctx,
		`SELECT id, description, owner, created_at, expires_at FROM write_tokens WHERE id = ?`, id,
	).Scan(&wt.ID, &wt.Description, &owner, &wt.CreatedAt, &wt.ExpiresAt); err != nil {
		return WriteToken{}, "", fmt.Errorf("rotate write token: %w", err)
	}
	wt.Owner = owner.String

	return wt, token, nil
}

// UpdateWriteTokenUsage records a successful authentication's timestamp -
// the same way UpdateCredentialUsage does for a passkey, so an admin can
// tell a token nobody's used from one in daily use when deciding whether
// to revoke it.
func (s *Store) UpdateWriteTokenUsage(ctx context.Context, id, usedAt string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE write_tokens SET last_used_at = ? WHERE id = ?`, usedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update write token usage: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update write token usage: %w", err)
	}

	if n == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// PurgeWriteToken permanently removes the write token issued under id -
// unlike RevokeWriteToken's soft-delete, the row is actually gone
// afterward, so any audit-log entry attributed to it stops resolving to
// a description or owner (the accepted tradeoff for a token id that's
// been deliberately purged, not treated as a bug). It returns
// ErrTokenNotFound if no token exists under id, or ErrTokenStillActive if
// the token is neither revoked nor past its expiry - a purge is only
// allowed once a token is already dead.
func (s *Store) PurgeWriteToken(ctx context.Context, id string) error {
	var (
		revokedAt sql.NullString
		expiresAt string
	)

	switch err := s.db.QueryRowContext(ctx,
		`SELECT revoked_at, expires_at FROM write_tokens WHERE id = ?`, id,
	).Scan(&revokedAt, &expiresAt); {
	case errors.Is(err, sql.ErrNoRows):
		return ErrTokenNotFound
	case err != nil:
		return fmt.Errorf("purge write token: %w", err)
	}

	if !isTokenDead(revokedAt, expiresAt) {
		return ErrTokenStillActive
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM write_tokens WHERE id = ?`, id); err != nil {
		return fmt.Errorf("purge write token: %w", err)
	}

	return nil
}

// isTokenDead reports whether a token is revoked or past its expiry,
// given its raw revoked_at/expires_at column values - shared by
// PurgeWriteToken and PurgeConsumerToken's identical eligibility check.
func isTokenDead(revokedAt sql.NullString, expiresAt string) bool {
	return StatusOfToken(revokedAt.Valid, expiresAt, time.Now().UTC()) != TokenActive
}

// TokenStatus is what a token is right now, by the server's own clock.
type TokenStatus string

// The three states a write or consumer token can be in.
const (
	TokenActive  TokenStatus = "active"
	TokenExpired TokenStatus = "expired"
	TokenRevoked TokenStatus = "revoked"
)

// StatusOfToken classifies a token from its revoked flag and expires_at
// column value at now. Revoked wins over expired. An unparseable expiry
// reads as active, the same way the purge check always has. This is the
// one definition of "dead", shared by the purge eligibility check and the
// status the API reports, so they can't drift apart.
func StatusOfToken(revoked bool, expiresAt string, now time.Time) TokenStatus {
	if revoked {
		return TokenRevoked
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return TokenActive
	}

	if !now.Before(expiry) {
		return TokenExpired
	}

	return TokenActive
}

// AllowedActions lists what may be done to a token in this state: a live
// token can be rotated or revoked, a dead one only purged. It is the same
// rule RotateWriteToken, RevokeWriteToken and PurgeWriteToken enforce
// themselves, spelled out so a client doesn't have to guess it.
func (st TokenStatus) AllowedActions() []string {
	if st == TokenActive {
		return []string{"rotate", "revoke"}
	}

	return []string{"purge"}
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
