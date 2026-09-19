package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrSessionNotFound is returned both when no session exists under a given
// id and when one does but has expired - a request carrying an expired
// session is treated as unauthenticated exactly like one carrying no
// session at all (openspec/changes/web-ui/specs/auth/spec.md's "Expired
// session is rejected" scenario).
var ErrSessionNotFound = errors.New("session not found")

// Session is one issued browser session - the web UI's own credential,
// never a substitute for the write bearer token.
type Session struct {
	ID        string
	CSRFToken string
	CreatedAt string
	ExpiresAt string
}

// CreateSession stores a newly issued session.
func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, csrf_token, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.CSRFToken, sess.CreatedAt, sess.ExpiresAt,
	); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

// GetSession returns the session stored under id, or ErrSessionNotFound if
// it's missing or expired.
func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	var sess Session

	err := s.db.QueryRowContext(ctx,
		`SELECT id, csrf_token, created_at, expires_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &sess.CSRFToken, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}

	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	expiry, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("parse session expiry: %w", err)
	}

	if time.Now().UTC().After(expiry) {
		return Session{}, ErrSessionNotFound
	}

	return sess, nil
}

// DeleteSession invalidates a session immediately - logout, or the
// regeneration a fresh login performs.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}
