package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

func TestListTokensCarriesStatusAndAllowedActions(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	cookie := seedSession(t, s)
	ctx := t.Context()

	_, _, err := s.CreateWriteToken(ctx, "live", time.Hour, "")
	require.NoError(t, err)
	_, _, err = s.CreateWriteToken(ctx, "expired", -time.Hour, "")
	require.NoError(t, err)
	revoked, _, err := s.CreateWriteToken(ctx, "revoked", time.Hour, "")
	require.NoError(t, err)
	require.NoError(t, s.RevokeWriteToken(ctx, revoked.ID))

	req := httptest.NewRequest(http.MethodGet, "/tokens", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var tokens []hushhush.TokenMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))

	byDescription := make(map[string]hushhush.TokenMetadata, len(tokens))
	for _, tok := range tokens {
		byDescription[tok.Description] = tok
	}

	require.Equal(t, "active", byDescription["live"].Status)
	require.Equal(t, []string{"rotate", "revoke"}, byDescription["live"].AllowedActions)
	require.Equal(t, "expired", byDescription["expired"].Status)
	require.Equal(t, []string{"purge"}, byDescription["expired"].AllowedActions)
	require.Equal(t, "revoked", byDescription["revoked"].Status)
	require.Equal(t, []string{"purge"}, byDescription["revoked"].AllowedActions)
}

func TestListConsumerTokensCarriesStatusAndAllowedActions(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	bearer := issueToken(t, s)
	ctx := t.Context()

	_, _, err := s.CreateConsumerToken(ctx, "homelab", "live", time.Hour)
	require.NoError(t, err)
	_, _, err = s.CreateConsumerToken(ctx, "homelab", "expired", -time.Hour)
	require.NoError(t, err)
	revoked, _, err := s.CreateConsumerToken(ctx, "homelab", "revoked", time.Hour)
	require.NoError(t, err)
	require.NoError(t, s.RevokeConsumerToken(ctx, revoked.ID))

	req := httptest.NewRequest(http.MethodGet, "/consumer-tokens", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var tokens []hushhush.ConsumerTokenMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &tokens))

	byDescription := make(map[string]hushhush.ConsumerTokenMetadata, len(tokens))
	for _, tok := range tokens {
		byDescription[tok.Description] = tok
	}

	require.Equal(t, "active", byDescription["live"].Status)
	require.Equal(t, []string{"rotate", "revoke"}, byDescription["live"].AllowedActions)
	require.Equal(t, "expired", byDescription["expired"].Status)
	require.Equal(t, []string{"purge"}, byDescription["expired"].AllowedActions)
	require.Equal(t, "revoked", byDescription["revoked"].Status)
	require.Equal(t, []string{"purge"}, byDescription["revoked"].AllowedActions)
}

func TestACreatedTokenIsActiveAndAllowsRotateAndRevoke(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	cookie := seedSession(t, s)
	sess, err := s.GetSession(t.Context(), cookie.Value)
	require.NoError(t, err)

	body, err := json.Marshal(hushhush.CreateTokenRequest{Description: "fresh", TTLSeconds: 3600})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, tokenRequest(t, http.MethodPost, "/tokens", body, cookie, sess.CSRFToken, true))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var created hushhush.TokenWithValue
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "active", created.Status)
	require.Equal(t, []string{"rotate", "revoke"}, created.AllowedActions)
}
