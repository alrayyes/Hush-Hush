package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	hushhush "github.com/alrayyes/hush-hush/internal/api"
	"github.com/stretchr/testify/require"
)

// The same consumer named twice in a used_by list is a harmless repeat, so it
// is recorded once, the way tags drop duplicates (alrayyes/hush-hush#646).

func TestCreateObjectWithARepeatedConsumerRecordsItOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{
		Slug: "dup", Value: sealedFixture, UsedBy: []string{"homelab", "other", "homelab"},
	}, issueToken(t, s)))

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var got hushhush.ObjectMetadata
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"homelab", "other"}, got.UsedBy, "the response says what was recorded")

	obj, err := s.GetObject(t.Context(), "dup")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab", "other"}, obj.UsedBy)
}

func TestUpdateObjectWithARepeatedConsumerRecordsItOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, createRequest(t, hushhush.CreateObjectRequest{Slug: "dup", Value: sealedFixture}, token))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	usedBy := []string{"homelab", "homelab"}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, updateRequestWithUsedBy(t, "dup", sealedFixtureNew, &usedBy, token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	obj, err := s.GetObject(t.Context(), "dup")
	require.NoError(t, err)
	require.Equal(t, []string{"homelab"}, obj.UsedBy)
}

func TestMCPToolsWithARepeatedConsumerRecordItOnce(t *testing.T) {
	t.Parallel()

	mux, s := newTestMux(t)
	token := issueToken(t, s)

	result := callTool(t, mux, token, "inject", map[string]any{
		"slug": "mcp_dup", "value": sealedFixture, "used_by": []string{"homelab", "homelab"},
	})
	require.False(t, result.IsError, "inject: %+v", result.Content)

	result = callTool(t, mux, token, "update", map[string]any{
		"slug": "mcp_dup", "value": sealedFixtureNew, "used_by": []string{"vps", "vps"},
	})
	require.False(t, result.IsError, "update: %+v", result.Content)

	obj, err := s.GetObject(t.Context(), "mcp_dup")
	require.NoError(t, err)
	require.Equal(t, []string{"vps"}, obj.UsedBy)
}
