package store_test

import (
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestCreateSessionThenGetReturnsIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC()

	require.NoError(t, s.CreateSession(t.Context(), store.Session{
		ID: "sess-1", CSRFToken: "csrf-1",
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	}))

	sess, err := s.GetSession(t.Context(), "sess-1")
	require.NoError(t, err)
	require.Equal(t, "csrf-1", sess.CSRFToken)
}

func TestGetSessionUnknownIDIsErrSessionNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)

	_, err := s.GetSession(t.Context(), "nope")
	require.ErrorIs(t, err, store.ErrSessionNotFound)
}

func TestGetSessionExpiredIsErrSessionNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC()

	require.NoError(t, s.CreateSession(t.Context(), store.Session{
		ID: "sess-1", CSRFToken: "csrf-1",
		CreatedAt: now.Add(-time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339),
	}))

	_, err := s.GetSession(t.Context(), "sess-1")
	require.ErrorIs(t, err, store.ErrSessionNotFound)
}

func TestDeleteSessionInvalidatesIt(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC()

	require.NoError(t, s.CreateSession(t.Context(), store.Session{
		ID: "sess-1", CSRFToken: "csrf-1",
		CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339),
	}))
	require.NoError(t, s.DeleteSession(t.Context(), "sess-1"))

	_, err := s.GetSession(t.Context(), "sess-1")
	require.ErrorIs(t, err, store.ErrSessionNotFound)
}
