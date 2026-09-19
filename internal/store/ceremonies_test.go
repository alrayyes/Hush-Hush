package store_test

import (
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestSaveCeremonyThenGetAndDeleteReturnsItOnce(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC()

	require.NoError(t, s.SaveCeremony(t.Context(), "cer-1", "registration", []byte("session-data"),
		now.Format(time.RFC3339), now.Add(5*time.Minute).Format(time.RFC3339)))

	data, err := s.GetAndDeleteCeremony(t.Context(), "cer-1", "registration")
	require.NoError(t, err)
	require.Equal(t, []byte("session-data"), data)

	_, err = s.GetAndDeleteCeremony(t.Context(), "cer-1", "registration")
	require.ErrorIs(t, err, store.ErrCeremonyNotFound)
}

func TestGetAndDeleteCeremonyWrongKindIsErrCeremonyNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC()

	require.NoError(t, s.SaveCeremony(t.Context(), "cer-1", "registration", []byte("session-data"),
		now.Format(time.RFC3339), now.Add(5*time.Minute).Format(time.RFC3339)))

	_, err := s.GetAndDeleteCeremony(t.Context(), "cer-1", "login")
	require.ErrorIs(t, err, store.ErrCeremonyNotFound)
}

func TestGetAndDeleteCeremonyExpiredIsErrCeremonyNotFound(t *testing.T) {
	t.Parallel()

	s := openTestStore(t)
	now := time.Now().UTC()

	require.NoError(t, s.SaveCeremony(t.Context(), "cer-1", "registration", []byte("session-data"),
		now.Add(-10*time.Minute).Format(time.RFC3339), now.Add(-5*time.Minute).Format(time.RFC3339)))

	_, err := s.GetAndDeleteCeremony(t.Context(), "cer-1", "registration")
	require.ErrorIs(t, err, store.ErrCeremonyNotFound)
}
