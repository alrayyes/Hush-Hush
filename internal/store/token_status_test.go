package store_test

import (
	"testing"
	"time"

	"github.com/alrayyes/hush-hush/internal/store"
	"github.com/stretchr/testify/require"
)

func TestTokenStatusFromRevokedFlagAndExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour).Format(time.RFC3339)
	past := now.Add(-time.Hour).Format(time.RFC3339)

	tests := map[string]struct {
		revoked   bool
		expiresAt string
		want      store.TokenStatus
		actions   []string
	}{
		"live":                {false, future, store.TokenActive, []string{"rotate", "revoke"}},
		"expired":             {false, past, store.TokenExpired, []string{"purge"}},
		"revoked":             {true, future, store.TokenRevoked, []string{"purge"}},
		"revoked and expired": {true, past, store.TokenRevoked, []string{"purge"}},
		"expiring right now":  {false, now.Format(time.RFC3339), store.TokenExpired, []string{"purge"}},
		"unparseable expiry":  {false, "not a time", store.TokenActive, []string{"rotate", "revoke"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := store.StatusOfToken(tc.revoked, tc.expiresAt, now)

			require.Equal(t, tc.want, got)
			require.Equal(t, tc.actions, got.AllowedActions())
		})
	}
}
